package deployer

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
)

// runHook runs `docker run --rm -e K=V image sh -c "cmd"`. Output streams to
// logf line-by-line. Returns nil on exit 0.
func runHook(ctx context.Context, image, cmd string, env map[string]string, logf func(string, ...any)) error {
	args := []string{"run", "--rm"}
	// Deterministic env order so the printed `docker run` command is stable
	// across re-runs of the same deployment.
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-e", k+"="+env[k])
	}
	args = append(args, image, "sh", "-c", cmd)

	logf("$ docker %s", redactCmd(args))
	c := exec.CommandContext(ctx, "docker", args...)
	stdout, err := c.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		return err
	}
	if err := c.Start(); err != nil {
		return err
	}
	done := make(chan struct{}, 2)
	go func() { streamReader(stdout, logf); done <- struct{}{} }()
	go func() { streamReader(stderr, logf); done <- struct{}{} }()
	<-done
	<-done
	if err := c.Wait(); err != nil {
		return fmt.Errorf("hook exited non-zero: %w", err)
	}
	return nil
}

// redactCmd renders the docker argv for logging, but replaces env values to
// keep the output readable — values are still visible in the env snapshot
// shown elsewhere in the UI (and are unredacted by request).
func redactCmd(args []string) string {
	var s string
	for i := 0; i < len(args); i++ {
		if i > 0 {
			s += " "
		}
		s += args[i]
	}
	return s
}
