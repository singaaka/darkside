package builder

import (
	"context"
	"fmt"
	"os/exec"
)

// cloneAtCommit shallow-clones the repo into workDir and checks out the given
// commit. Uses the installation token via the x-access-token basic-auth scheme
// GitHub recommends for Apps.
func cloneAtCommit(ctx context.Context, repoFullName, sha, token, workDir string, logf func(string, ...any)) error {
	url := fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", token, repoFullName)
	logf("git init")
	if err := streamCmd(ctx, exec.CommandContext(ctx, "git", "init", "-q", workDir), logf); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	add := exec.CommandContext(ctx, "git", "-C", workDir, "remote", "add", "origin", url)
	if err := streamCmd(ctx, add, logf); err != nil {
		return fmt.Errorf("git remote add: %w", err)
	}
	logf("git fetch %s", sha)
	fetch := exec.CommandContext(ctx, "git", "-C", workDir, "fetch", "--depth", "1", "origin", sha)
	if err := streamCmd(ctx, fetch, logf); err != nil {
		return fmt.Errorf("git fetch %s: %w", sha, err)
	}
	co := exec.CommandContext(ctx, "git", "-C", workDir, "checkout", "-q", "FETCH_HEAD")
	if err := streamCmd(ctx, co, logf); err != nil {
		return fmt.Errorf("git checkout %s: %w", sha, err)
	}
	logf("checked out %s", sha)
	return nil
}
