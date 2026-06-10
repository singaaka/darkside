// Package builder runs the build half of a deployment: clone the repo at a
// specific commit, read its darkside.toml, and produce a tagged docker image.
//
// Phase 4 stops after the build succeeds. Phase 5 (deploy.go in this package
// in a later iteration, or a sibling package) picks up where we leave off:
// decrypt env, run pre hook, submit nomad job, run post hook.
package builder

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/singaaka/darkside/internal/ageenv"
	"github.com/singaaka/darkside/internal/db/dbgen"
	"github.com/singaaka/darkside/internal/deployer"
	"github.com/singaaka/darkside/internal/github"
	"github.com/singaaka/darkside/internal/loghub"
	"github.com/singaaka/darkside/internal/manifest"
	"github.com/singaaka/darkside/internal/store"
)

// Builder turns a queued Job into a built artifact, recording status + logs as
// it goes. After a successful build, hands off to Deployer for pre → nomad → post.
type Builder struct {
	Store    *store.Store
	Hub      *loghub.Hub
	Deployer *deployer.Deployer
	WorkRoot string // host path under which to clone repos, e.g. /data/builds
}

// Job is the work item the queue runs through Builder.
type Job struct {
	DeploymentID string
	AppID        string
	AppName      string
	RepoFullName string
	Branch       string
	CommitSHA    string
	CommitMsg    string
	// EnvOverride forces a target environment regardless of the manifest's
	// [branches] map. Set by Redeploy so re-running an old commit on a branch
	// that's since been remapped still goes to the original env.
	EnvOverride string
}

const phaseBuild = "build"

// Run executes the build pipeline. Errors are recorded on the deployment row
// and streamed to the log topic; the caller does not need to handle them.
func (b *Builder) Run(ctx context.Context, app *github.App, installID int64, job Job) {
	logf := func(format string, args ...any) {
		line := fmt.Sprintf(format, args...)
		if !strings.HasSuffix(line, "\n") {
			line += "\n"
		}
		b.Hub.Publish(job.DeploymentID, phaseBuild, line)
	}
	fail := func(err error) {
		logf("ERROR: %s", err)
		b.persistLogAndStatus(ctx, job.DeploymentID, "failed", err.Error())
	}

	b.persistLogAndStatus(ctx, job.DeploymentID, "cloning", "")
	logf("[%s] clone repo %s @ %s", time.Now().Format(time.RFC3339), job.RepoFullName, job.CommitSHA)

	workDir := filepath.Join(b.WorkRoot, job.DeploymentID)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		fail(fmt.Errorf("mkdir work dir: %w", err))
		return
	}
	defer func() {
		// Keep workdir for debugging if build failed. For successful builds,
		// nothing keeps us from wiping it; for now leave it for inspection.
		_ = workDir
	}()

	token, err := app.InstallationToken(ctx, installID)
	if err != nil {
		fail(fmt.Errorf("get installation token: %w", err))
		return
	}

	if err := cloneAtCommit(ctx, job.RepoFullName, job.CommitSHA, token, workDir, logf); err != nil {
		fail(err)
		return
	}

	manifestBytes, err := os.ReadFile(filepath.Join(workDir, "darkside.toml"))
	if err != nil {
		fail(fmt.Errorf("read darkside.toml: %w", err))
		return
	}
	m, err := manifest.Parse(manifestBytes)
	if err != nil {
		fail(fmt.Errorf("parse darkside.toml: %w", err))
		return
	}
	logf("manifest ok: app=%s, environments=%d, build context=%s", m.Name, len(m.Environments), m.Build.Context)

	// Resolve target env: explicit override wins, else branch → env from manifest.
	envName := job.EnvOverride
	if envName == "" {
		envName = m.EnvForBranch(job.Branch)
	}
	var envRow *dbgen.Environment
	if envName == "" {
		logf("branch %q has no [branches] mapping — building artifact only, no deploy", job.Branch)
	} else {
		env, err := b.Store.GetEnvironment(ctx, dbgen.GetEnvironmentParams{AppID: job.AppID, Name: envName})
		if err != nil {
			fail(fmt.Errorf("environment %q from [branches] not created in darkside: %w", envName, err))
			return
		}
		envRow = &env
		logf("branch %q → environment %q (id=%s)", job.Branch, envName, env.ID)
		if err := b.Store.UpdateDeploymentEnvironment(ctx, dbgen.UpdateDeploymentEnvironmentParams{
			ID:            job.DeploymentID,
			EnvironmentID: &env.ID,
		}); err != nil {
			fail(fmt.Errorf("attach environment to deployment: %w", err))
			return
		}
	}

	// If we have a target env, decrypt its env file now so we can pass the
	// resolved vars to `docker build --build-arg` (build phase) and downstream
	// to deployer (pre/run/post). When there's no env, build runs without any.
	var envMap map[string]string
	if envRow != nil {
		envBlock := m.EnvironmentByName(envRow.Name)
		if envBlock == nil {
			fail(fmt.Errorf("manifest has no [[environments]] block named %q", envRow.Name))
			return
		}
		envCipher, err := os.ReadFile(filepath.Join(workDir, envBlock.EnvFile))
		if err != nil {
			fail(fmt.Errorf("read encrypted env %s: %w", envBlock.EnvFile, err))
			return
		}
		plain, err := ageenv.Decrypt(envCipher, envRow.AgePrivateKey)
		if err != nil {
			fail(fmt.Errorf("decrypt %s: %w", envBlock.EnvFile, err))
			return
		}
		envMap, err = ageenv.ParseEnvFile(plain)
		if err != nil {
			fail(fmt.Errorf("parse %s: %w", envBlock.EnvFile, err))
			return
		}
		logf("decrypted %s → %d env vars", envBlock.EnvFile, len(envMap))
	}

	short := shortSHA(job.CommitSHA)
	imageTag := fmt.Sprintf("%s:%s", job.AppName, short)

	b.persistLogAndStatus(ctx, job.DeploymentID, "building", "")
	logf("docker build → %s", imageTag)

	contextDir := filepath.Join(workDir, m.Build.Context)
	if err := dockerBuild(ctx, contextDir, m.Build.Dockerfile, imageTag, envMap, logf); err != nil {
		fail(err)
		return
	}
	logf("docker build complete: %s", imageTag)

	if err := b.Store.UpdateDeploymentArtifact(ctx, dbgen.UpdateDeploymentArtifactParams{
		ID:          job.DeploymentID,
		ImageTag:    ptr(imageTag),
		NomadJobHcl: nil,
		EnvSnapshot: nil,
	}); err != nil {
		fail(fmt.Errorf("save artifact: %w", err))
		return
	}
	logf("build phase complete")
	// Persist the build log and close its topic before handing off.
	b.persistBuildLog(ctx, job.DeploymentID)
	b.Hub.Close(job.DeploymentID, phaseBuild)

	// No env → no deploy; we still mark the deployment "built" so the UI shows
	// the artifact and the user can promote it manually later (Phase 6).
	if envRow == nil || b.Deployer == nil {
		_ = b.Store.UpdateDeploymentStatus(ctx, dbgen.UpdateDeploymentStatusParams{
			ID: job.DeploymentID, Status: "built", Error: nil,
		})
		_ = b.Store.FinishDeployment(ctx, dbgen.FinishDeploymentParams{
			ID: job.DeploymentID, Status: "built",
		})
		return
	}

	// Hand off to deployer with the already-resolved env so it doesn't have to
	// re-decrypt.
	b.Deployer.Run(ctx, deployer.Input{
		DeploymentID: job.DeploymentID,
		AppID:        job.AppID,
		AppName:      job.AppName,
		WorkDir:      workDir,
		ImageTag:     imageTag,
		Manifest:     m,
		Environment:  *envRow,
		Env:          envMap,
	})
}

// persistBuildLog flushes the live buffer to the deployment_log table.
func (b *Builder) persistBuildLog(ctx context.Context, depID string) {
	lines := b.Hub.Buffered(depID, phaseBuild)
	if len(lines) == 0 {
		return
	}
	_ = b.Store.UpsertDeploymentLog(ctx, dbgen.UpsertDeploymentLogParams{
		DeploymentID: depID,
		Phase:        phaseBuild,
		Content:      strings.Join(lines, ""),
	})
}

func (b *Builder) persistLogAndStatus(ctx context.Context, depID, status, errMsg string) {
	b.persistBuildLog(ctx, depID)
	if status == "" {
		return
	}
	var errPtr *string
	if errMsg != "" {
		errPtr = &errMsg
	}
	_ = b.Store.UpdateDeploymentStatus(ctx, dbgen.UpdateDeploymentStatusParams{
		ID:     depID,
		Status: status,
		Error:  errPtr,
	})
	if status == "failed" {
		b.Hub.Close(depID, phaseBuild)
		_ = b.Store.FinishDeployment(ctx, dbgen.FinishDeploymentParams{ID: depID, Status: status})
	}
}

func shortSHA(s string) string {
	if len(s) >= 7 {
		return s[:7]
	}
	return s
}

func ptr[T any](v T) *T { return &v }

// streamCmd runs cmd, prefixing each output line and feeding it to logf. We
// intentionally line-buffer rather than blast raw bytes so the live log view
// stays readable.
func streamCmd(ctx context.Context, cmd *exec.Cmd, logf func(string, ...any)) error {
	cmd.Stdout = nil
	cmd.Stderr = nil
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	pump := func(r io.Reader, prefix string) {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			logf("%s%s", prefix, sc.Text())
		}
	}
	done := make(chan struct{}, 2)
	go func() { pump(stdout, ""); done <- struct{}{} }()
	go func() { pump(stderr, ""); done <- struct{}{} }()
	<-done
	<-done

	if err := cmd.Wait(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return ctx.Err()
		}
		return err
	}
	return nil
}
