// Package ansible runs ansible-playbook commands and streams their output.
package ansible

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Runner executes ansible playbooks.
type Runner struct {
	PlaybookDir string // path to directory containing playbooks
}

// RunOptions holds per-invocation options.
type RunOptions struct {
	Inventory  string            // path to inventory file (or inline "-" format)
	ExtraVars  map[string]string // --extra-vars
	Playbook   string            // basename of the playbook file
	PrivateKey string            // path to SSH private key
	RemoteUser string
	LogF       func(string) // called for each output line
}

// Run executes the playbook and streams output to opts.LogF.
func (r *Runner) Run(ctx context.Context, opts RunOptions) error {
	playbookPath := filepath.Join(r.PlaybookDir, opts.Playbook)

	args := []string{playbookPath}
	if opts.Inventory != "" {
		args = append(args, "-i", opts.Inventory)
	}
	if opts.PrivateKey != "" {
		args = append(args, "--private-key", opts.PrivateKey)
	}
	if opts.RemoteUser != "" {
		args = append(args, "-u", opts.RemoteUser)
	}
	for k, v := range opts.ExtraVars {
		args = append(args, "--extra-vars", k+"="+v)
	}
	args = append(args, "--diff")

	cmd := exec.CommandContext(ctx, "ansible-playbook", args...)
	cmd.Env = append(os.Environ(), "ANSIBLE_FORCE_COLOR=0", "ANSIBLE_NOCOLOR=1")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ansible-playbook: %w (is it installed?)", err)
	}

	logf := opts.LogF
	if logf == nil {
		logf = func(string) {}
	}

	done := make(chan struct{}, 2)
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			logf(sc.Text())
		}
		done <- struct{}{}
	}()
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			logf(sc.Text())
		}
		done <- struct{}{}
	}()
	<-done
	<-done

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ansible-playbook failed: %w", err)
	}
	return nil
}

// Check verifies that ansible-playbook is installed and returns its version.
func Check() (string, error) {
	var out bytes.Buffer
	cmd := exec.Command("ansible-playbook", "--version")
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ansible-playbook not found in PATH: %w", err)
	}
	lines := strings.SplitN(out.String(), "\n", 2)
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0]), nil
	}
	return "ansible-playbook (version unknown)", nil
}
