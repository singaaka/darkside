package deployer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/singaaka/darkside/internal/db/dbgen"
	"github.com/singaaka/darkside/internal/loghub"
	"github.com/singaaka/darkside/internal/manifest"
	"github.com/singaaka/darkside/internal/store"
)

const (
	PhasePre    = "pre"
	PhaseDeploy = "deploy"
	PhasePost   = "post"
)

type Deployer struct {
	Store      *store.Store
	Hub        *loghub.Hub
	NomadAddr  string
	Datacenter string
}

// Input is the full set of things the deployer needs from the build step.
type Input struct {
	DeploymentID string
	AppID        string
	AppName      string
	WorkDir      string
	ImageTag     string
	Manifest     *manifest.Manifest
	Environment  dbgen.Environment
	// Env is the already-decrypted env map. If nil, the deployer falls back to
	// decrypting from the workdir — useful when invoked outside the normal
	// build path (e.g. a future "redeploy without rebuild" path).
	Env map[string]string
}

// Run performs pre → deploy → post. Each phase has its own log topic so the UI
// can stream them independently. Aborts on first failure and leaves the
// deployment row in "failed".
func (d *Deployer) Run(ctx context.Context, in Input) {
	jobName := fmt.Sprintf("%s-%s", in.AppName, in.Environment.Name)

	// 1. Resolve env: prefer caller-provided map (decrypted once by builder),
	// fall back to decrypting from the cloned workdir.
	var envMap map[string]string
	var envJSON string
	if in.Env != nil {
		envMap = in.Env
		envJSON = snapshotJSON(envMap)
	} else {
		var err error
		envMap, envJSON, err = resolveEnv(in.WorkDir, in.Manifest, in.Environment)
		if err != nil {
			d.failOverall(ctx, in.DeploymentID, fmt.Errorf("resolve env: %w", err))
			return
		}
	}

	// 2. Render nomad job + persist alongside env snapshot.
	hcl, err := RenderNomadJob(jobName, d.Datacenter, in.ImageTag, in.Manifest, envMap)
	if err != nil {
		d.failOverall(ctx, in.DeploymentID, err)
		return
	}
	if err := d.Store.UpdateDeploymentArtifact(ctx, dbgen.UpdateDeploymentArtifactParams{
		ID:          in.DeploymentID,
		ImageTag:    &in.ImageTag,
		NomadJobHcl: &hcl,
		EnvSnapshot: &envJSON,
	}); err != nil {
		d.failOverall(ctx, in.DeploymentID, fmt.Errorf("persist artifact: %w", err))
		return
	}

	// 3. Pre hook (skipped if manifest has none).
	if cmd := strings.TrimSpace(in.Manifest.Hooks.Pre); cmd != "" {
		if !d.runPhase(ctx, in.DeploymentID, PhasePre, "pre hook", func(logf func(string, ...any)) error {
			logf("[%s] running pre hook on %s", time.Now().Format(time.RFC3339), in.ImageTag)
			return runHook(ctx, in.ImageTag, cmd, envMap, logf)
		}) {
			return
		}
	}

	// 4. Submit nomad job + watch.
	if !d.runPhase(ctx, in.DeploymentID, PhaseDeploy, "nomad deploy", func(logf func(string, ...any)) error {
		logf("[%s] submitting nomad job %s with image %s", time.Now().Format(time.RFC3339), jobName, in.ImageTag)
		if _, err := submitNomadJob(ctx, d.NomadAddr, hcl, logf); err != nil {
			return err
		}
		return waitForJobHealthy(ctx, d.NomadAddr, jobName, logf)
	}) {
		return
	}

	// 5. Post hook (skipped if manifest has none).
	if cmd := strings.TrimSpace(in.Manifest.Hooks.Post); cmd != "" {
		if !d.runPhase(ctx, in.DeploymentID, PhasePost, "post hook", func(logf func(string, ...any)) error {
			logf("[%s] running post hook on %s", time.Now().Format(time.RFC3339), in.ImageTag)
			return runHook(ctx, in.ImageTag, cmd, envMap, logf)
		}) {
			return
		}
	}

	// 6. Done.
	_ = d.Store.UpdateDeploymentStatus(ctx, dbgen.UpdateDeploymentStatusParams{
		ID:     in.DeploymentID,
		Status: "succeeded",
		Error:  nil,
	})
	_ = d.Store.FinishDeployment(ctx, dbgen.FinishDeploymentParams{
		ID:     in.DeploymentID,
		Status: "succeeded",
	})
}

// runPhase wraps the per-phase plumbing: set status, open log topic, run fn,
// persist transcript, close topic. Returns false on failure so the caller
// short-circuits the pipeline.
func (d *Deployer) runPhase(ctx context.Context, depID, phase, label string, fn func(logf func(string, ...any)) error) bool {
	_ = d.Store.UpdateDeploymentStatus(ctx, dbgen.UpdateDeploymentStatusParams{
		ID:     depID,
		Status: phase, // we use the phase name as the status (pre|deploy|post)
		Error:  nil,
	})
	logf := func(format string, args ...any) {
		line := fmt.Sprintf(format, args...)
		if !strings.HasSuffix(line, "\n") {
			line += "\n"
		}
		d.Hub.Publish(depID, phase, line)
	}
	err := fn(logf)
	// Persist whatever accumulated so the UI can replay after the live stream ends.
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
		ID:     depID,
		Status: "failed",
		Error:  &msg,
	})
	_ = d.Store.FinishDeployment(ctx, dbgen.FinishDeploymentParams{ID: depID, Status: "failed"})
}
