// Package builder submits the clone+build as a Nomad batch job and hands off
// to the deployer when the image is ready in the private registry.
package builder

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/singaaka/darkside/internal/db/dbgen"
	"github.com/singaaka/darkside/internal/deployer"
	"github.com/singaaka/darkside/internal/github"
	"github.com/singaaka/darkside/internal/loghub"
	"github.com/singaaka/darkside/internal/store"
)

// Builder orchestrates: resolve manifest → submit Nomad build job → hand off to Deployer.
type Builder struct {
	Store        *store.Store
	Hub          *loghub.Hub
	Deployer     *deployer.Deployer
	NomadAddr    string
	RegistryAddr string
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
	TriggerType  string
	// ReuseImageIfExists skips the build step if the image already exists in
	// the registry (used for rollback/redeploy).
	ReuseImageIfExists bool
}

const phaseBuild = "build"

// Run executes the build pipeline. Errors are recorded on the deployment row.
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
	logf("[%s] starting build for %s @ %s", time.Now().Format(time.RFC3339), job.RepoFullName, job.CommitSHA)

	// Mint an installation token for the build job to clone the repo.
	token, err := app.InstallationToken(ctx, installID)
	if err != nil {
		fail(fmt.Errorf("get installation token: %w", err))
		return
	}

	short := shortSHA(job.CommitSHA)
	imageTag := fmt.Sprintf("%s/%s:%s", b.RegistryAddr, job.AppName, short)

	// Check if we can skip the build (rollback with existing image).
	if job.ReuseImageIfExists {
		if exists, _ := imageExistsInRegistry(ctx, b.RegistryAddr, job.AppName, short); exists {
			logf("image %s exists in registry — skipping build", imageTag)
			b.persistLogAndStatus(ctx, job.DeploymentID, "built", "")
			b.persistBuildLog(ctx, job.DeploymentID)
			b.Hub.Close(job.DeploymentID, phaseBuild)
			b.runDeploy(ctx, job, app, installID, imageTag, token, logf, fail)
			return
		}
	}

	// Fetch the darkside.toml from the repo to get manifest config.
	b.persistLogAndStatus(ctx, job.DeploymentID, "building", "")
	mf, err := fetchManifest(ctx, job.RepoFullName, job.CommitSHA, token)
	if err != nil {
		fail(fmt.Errorf("fetch darkside.toml: %w", err))
		return
	}
	logf("manifest ok: app=%s, branch=%s, env_file=%s", mf.Name, mf.Branch, mf.EnvFile)

	// Read the encrypted env file from the repo.
	appRow, err := b.Store.GetApp(ctx, job.AppID)
	if err != nil {
		fail(fmt.Errorf("get app: %w", err))
		return
	}

	var buildArgEnv map[string]string
	envMap, err := decryptEnvFromRepo(ctx, job.RepoFullName, job.CommitSHA, mf.EnvFile, appRow.AgePrivateKey, token)
	if err != nil {
		logf("WARN: could not decrypt env file (%s) — building without env args: %v", mf.EnvFile, err)
		buildArgEnv = map[string]string{}
	} else {
		buildArgEnv = envMap
		logf("decrypted %s → %d env vars", mf.EnvFile, len(envMap))
	}

	// Build command: git clone + docker build + docker push.
	buildCmd := buildScript(job.RepoFullName, job.CommitSHA, token, imageTag, mf.Build.Dockerfile, mf.Build.Context, buildArgEnv)

	buildJobHCL, err := deployer.RenderBatchJob(deployer.BatchJobInput{
		JobName:    fmt.Sprintf("build-%s-%s", job.AppName, short),
		Datacenter: "dc1",
		Image:      "docker:cli",
		Command:    buildCmd,
		Env: map[string]string{
			"DOCKER_HOST": "unix:///var/run/docker.sock",
		},
		NodeMeta: map[string]string{"darkside.paas": "true"},
	})
	if err != nil {
		fail(fmt.Errorf("render build job: %w", err))
		return
	}

	// Persist build job HCL.
	buildJobHCLPtr := buildJobHCL
	_ = b.Store.UpdateDeploymentArtifact(ctx, dbgen.UpdateDeploymentArtifactParams{
		ID: job.DeploymentID, BuildJobHcl: &buildJobHCLPtr,
	})

	// Submit to Nomad and stream logs.
	nc := newNomadClientForBuilder(b.NomadAddr)
	buildJobName := fmt.Sprintf("build-%s-%s", job.AppName, short)
	logf("submitting build job %s", buildJobName)
	if _, err := nc.submitHCLJob(ctx, buildJobHCL, logf); err != nil {
		fail(fmt.Errorf("submit build job: %w", err))
		return
	}

	// Stream build logs while waiting.
	go func() {
		_ = nc.streamAllocLogs(ctx, buildJobName, "run", logf)
	}()

	if err := nc.waitForBatchJob(ctx, buildJobName, logf); err != nil {
		fail(fmt.Errorf("build job: %w", err))
		return
	}
	logf("build complete: %s", imageTag)
	b.persistLogAndStatus(ctx, job.DeploymentID, "built", "")
	b.persistBuildLog(ctx, job.DeploymentID)
	b.Hub.Close(job.DeploymentID, phaseBuild)

	b.runDeploy(ctx, job, app, installID, imageTag, token, logf, fail)
}

func (b *Builder) runDeploy(ctx context.Context, job Job, app *github.App, installID int64, imageTag, token string, logf func(string, ...any), fail func(error)) {
	appRow, err := b.Store.GetApp(ctx, job.AppID)
	if err != nil {
		fail(fmt.Errorf("get app: %w", err))
		return
	}

	mf, err := fetchManifest(ctx, job.RepoFullName, job.CommitSHA, token)
	if err != nil {
		fail(fmt.Errorf("fetch manifest: %w", err))
		return
	}

	// Check if the deploy branch in the manifest matches the pushed branch.
	if job.Branch != "" && mf.Branch != "" && job.Branch != mf.Branch {
		logf("branch %q does not match manifest deploy branch %q — skipping deploy", job.Branch, mf.Branch)
		_ = b.Store.UpdateDeploymentStatus(ctx, dbgen.UpdateDeploymentStatusParams{
			ID: job.DeploymentID, Status: "built",
		})
		_ = b.Store.FinishDeployment(ctx, dbgen.FinishDeploymentParams{ID: job.DeploymentID, Status: "built"})
		return
	}

	envMap, err := decryptEnvFromRepo(ctx, job.RepoFullName, job.CommitSHA, mf.EnvFile, appRow.AgePrivateKey, token)
	if err != nil {
		logf("WARN: could not decrypt env file — deploying with empty env: %v", err)
		envMap = map[string]string{}
	}

	b.Deployer.Run(ctx, deployer.Input{
		DeploymentID: job.DeploymentID,
		AppID:        job.AppID,
		AppName:      job.AppName,
		ImageTag:     imageTag,
		Manifest:     mf,
		Env:          envMap,
	})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func shortSHA(s string) string {
	if len(s) >= 7 {
		return s[:7]
	}
	return s
}

func buildScript(repoFullName, sha, token, imageTag, dockerfile, context string, envArgs map[string]string) string {
	cloneURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", token, repoFullName)
	args := []string{
		"git clone --depth=1 " + cloneURL + " /workspace",
		"cd /workspace",
		fmt.Sprintf("git fetch --depth=1 origin %s && git checkout %s", sha, sha),
	}

	// Build args.
	keys := make([]string, 0, len(envArgs))
	for k := range envArgs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	buildArgs := ""
	for _, k := range keys {
		buildArgs += fmt.Sprintf(" --build-arg %s=%q", k, envArgs[k])
	}

	args = append(args,
		fmt.Sprintf("docker build -t %s -f %s%s %s", imageTag, dockerfile, buildArgs, context),
		"docker push "+imageTag,
	)
	return strings.Join(args, " && ")
}

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
