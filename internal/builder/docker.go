package builder

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
)

// dockerBuild shells out to `docker build` against the local daemon. Env vars
// are forwarded as --build-arg so the user's Dockerfile can `ARG FOO` them
// in. We use the CLI rather than the SDK so:
//   - the dep tree stays small (no /github.com/docker/docker)
//   - the build output users see in darkside matches what `docker build` shows
//     them locally
//
// The runtime container has docker-cli installed and the host socket mounted.
func dockerBuild(ctx context.Context, contextDir, dockerfile, tag string, env map[string]string, logf func(string, ...any)) error {
	dockerfilePath := filepath.Join(contextDir, dockerfile)
	args := []string{"build", "-t", tag, "-f", dockerfilePath, "--progress=plain"}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "--build-arg", k+"="+env[k])
	}
	args = append(args, contextDir)

	cmd := exec.CommandContext(ctx, "docker", args...)
	if err := streamCmd(ctx, cmd, logf); err != nil {
		return fmt.Errorf("docker build: %w", err)
	}
	return nil
}
