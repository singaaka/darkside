package deployer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/singaaka/darkside-paas/internal/db/dbgen"
	"github.com/singaaka/darkside-paas/internal/loghub"
	"github.com/singaaka/darkside-paas/internal/manifest"
	"github.com/singaaka/darkside-paas/internal/store"
)

const (
	PhasePre    = "pre"
	PhaseDeploy = "deploy"
	PhasePost   = "post"
)

type Deployer struct {
	Store        *store.Store
	Hub          *loghub.Hub
	NomadAddr    string
	RegistryAddr string
	Datacenter   string
}

// Input is everything the deployer needs after the build phase.
type Input struct {
	DeploymentID string
	AppID        string
	AppName      string
	ImageTag     string
	Manifest     *manifest.Manifest
	Env          map[string]string
}

// Run performs pre → deploy → post using Nomad batch jobs for hooks and a
// Nomad service job for the long-lived workload.
func (d *Deployer) Run(ctx context.Context, in Input) {
	nc := newNomadClient(d.NomadAddr)
	jobName := in.AppName

	// Env snapshot for persistence.
	envJSON := snapshotJSON(in.Env)

	// Render the service job spec.
	hcl, err := RenderNomadJob(jobName, d.Datacenter, in.ImageTag, in.Manifest, in.Env)
	if err != nil {
		d.failOverall(ctx, in.DeploymentID, err)
		return
	}

	// Render hook HCLs (may be empty if no hooks defined).
	var preHCL, postHCL string
	if cmd := strings.TrimSpace(in.Manifest.Hooks.Pre); cmd != "" {
		preHCL, err = RenderBatchJob(BatchJobInput{
			JobName:    fmt.Sprintf("pre-%s-%s", jobName, in.DeploymentID[:8]),
			Datacenter: d.Datacenter,
			Image:      in.ImageTag,
			Command:    cmd,
			Env:        in.Env,
		})
		if err != nil {
			d.failOverall(ctx, in.DeploymentID, fmt.Errorf("render pre hook: %w", err))
			return
		}
	}
	if cmd := strings.TrimSpace(in.Manifest.Hooks.Post); cmd != "" {
		postHCL, err = RenderBatchJob(BatchJobInput{
			JobName:    fmt.Sprintf("post-%s-%s", jobName, in.DeploymentID[:8]),
			Datacenter: d.Datacenter,
			Image:      in.ImageTag,
			Command:    cmd,
			Env:        in.Env,
		})
		if err != nil {
			d.failOverall(ctx, in.DeploymentID, fmt.Errorf("render post hook: %w", err))
			return
		}
	}

	// Persist all rendered HCLs + env snapshot.
	var prePtr, postPtr *string
	if preHCL != "" {
		prePtr = &preHCL
	}
	if postHCL != "" {
		postPtr = &postHCL
	}
	if err := d.Store.UpdateDeploymentArtifact(ctx, dbgen.UpdateDeploymentArtifactParams{
		ID:          in.DeploymentID,
		ImageTag:    &in.ImageTag,
		NomadJobHcl: &hcl,
		EnvSnapshot: &envJSON,
		PreHookHcl:  prePtr,
		PostHookHcl: postPtr,
	}); err != nil {
		d.failOverall(ctx, in.DeploymentID, fmt.Errorf("persist artifact: %w", err))
		return
	}

	// ── Pre hook ─────────────────────────────────────────────────────────────
	if preHCL != "" {
		preJobName := fmt.Sprintf("pre-%s-%s", jobName, in.DeploymentID[:8])
		if !d.runPhase(ctx, in.DeploymentID, PhasePre, "pre hook", func(logf func(string, ...any)) error {
			logf("[%s] submitting pre hook job %s", time.Now().Format(time.RFC3339), preJobName)
			if _, err := nc.submitHCLJob(ctx, preHCL, logf); err != nil {
				return err
			}
			// Stream logs while waiting.
			go func() {
				_ = nc.streamAllocLogs(ctx, preJobName, "run", logf)
			}()
			return nc.waitForBatchJob(ctx, preJobName, logf)
		}) {
			return
		}
	}

	// ── Deploy (Nomad service job) ────────────────────────────────────────────
	if !d.runPhase(ctx, in.DeploymentID, PhaseDeploy, "nomad deploy", func(logf func(string, ...any)) error {
		logf("[%s] submitting nomad service job %s with image %s", time.Now().Format(time.RFC3339), jobName, in.ImageTag)
		if _, err := nc.submitHCLJob(ctx, hcl, logf); err != nil {
			return err
		}
		return nc.waitForJobHealthy(ctx, jobName, logf)
	}) {
		return
	}

	// ── Post hook ────────────────────────────────────────────────────────────
	if postHCL != "" {
		postJobName := fmt.Sprintf("post-%s-%s", jobName, in.DeploymentID[:8])
		if !d.runPhase(ctx, in.DeploymentID, PhasePost, "post hook", func(logf func(string, ...any)) error {
			logf("[%s] submitting post hook job %s", time.Now().Format(time.RFC3339), postJobName)
			if _, err := nc.submitHCLJob(ctx, postHCL, logf); err != nil {
				return err
			}
			go func() {
				_ = nc.streamAllocLogs(ctx, postJobName, "run", logf)
			}()
			return nc.waitForBatchJob(ctx, postJobName, logf)
		}) {
			return
		}
	}

	// ── Done ─────────────────────────────────────────────────────────────────
	_ = d.Store.UpdateDeploymentStatus(ctx, dbgen.UpdateDeploymentStatusParams{
		ID: in.DeploymentID, Status: "succeeded", Error: nil,
	})
	_ = d.Store.FinishDeployment(ctx, dbgen.FinishDeploymentParams{
		ID: in.DeploymentID, Status: "succeeded",
	})
}

// ScaleJob scales a running Nomad service job without redeploying.
func (d *Deployer) ScaleJob(ctx context.Context, appName string, count int) error {
	return newNomadClient(d.NomadAddr).scaleJob(ctx, appName, count)
}

func (d *Deployer) runPhase(ctx context.Context, depID, phase, label string, fn func(logf func(string, ...any)) error) bool {
	_ = d.Store.UpdateDeploymentStatus(ctx, dbgen.UpdateDeploymentStatusParams{
		ID: depID, Status: phase, Error: nil,
	})
	logf := func(format string, args ...any) {
		line := fmt.Sprintf(format, args...)
		if !strings.HasSuffix(line, "\n") {
			line += "\n"
		}
		d.Hub.Publish(depID, phase, line)
	}
	err := fn(logf)
	if buf := d.Hub.Buffered(depID, phase); len(buf) > 0 {
		_ = d.Store.UpsertDeploymentLog(ctx, dbgen.UpsertDeploymentLogParams{
			DeploymentID: depID,
			Phase:        phase,
			Content:      strings.Join(buf, ""),
		})
	}
	d.Hub.Close(depID, phase)

	if err != nil {
		logf("ERROR: %s failed: %s", label, err)
		d.failOverall(ctx, depID, fmt.Errorf("%s: %w", label, err))
		return false
	}
	return true
}

func (d *Deployer) failOverall(ctx context.Context, depID string, err error) {
	msg := err.Error()
	_ = d.Store.UpdateDeploymentStatus(ctx, dbgen.UpdateDeploymentStatusParams{
		ID: depID, Status: "failed", Error: &msg,
	})
	_ = d.Store.FinishDeployment(ctx, dbgen.FinishDeploymentParams{ID: depID, Status: "failed"})
}
