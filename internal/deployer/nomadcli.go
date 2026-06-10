package deployer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// submitNomadJob pipes the HCL2 spec into `nomad job run -` and parses the
// printed eval id. Mirrors what an operator would do at a shell.
func submitNomadJob(ctx context.Context, nomadAddr, hcl string, logf func(string, ...any)) (string, error) {
	cmd := exec.CommandContext(ctx, "nomad", "job", "run", "-detach", "-")
	cmd.Env = append(cmd.Environ(), "NOMAD_ADDR="+nomadAddr)
	cmd.Stdin = strings.NewReader(hcl)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logf("nomad job run failed: %s", strings.TrimSpace(stderr.String()))
		return "", fmt.Errorf("nomad job run: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	if s := strings.TrimSpace(stderr.String()); s != "" {
		logf("%s", s)
	}
	out := strings.TrimSpace(stdout.String())
	for _, line := range strings.Split(out, "\n") {
		logf("%s", line)
	}
	// `nomad job run -detach` prints the evaluation ID. We capture it but
	// honestly we mostly use the job name to query deployment status below.
	return out, nil
}

// jobStatus is the slim shape of `nomad job status -json` we care about.
type jobStatus struct {
	ID                 string `json:"ID"`
	Status             string `json:"Status"` // running|pending|dead
	LatestDeployment   *struct {
		ID     string `json:"ID"`
		Status string `json:"Status"` // running|successful|failed|paused|cancelled|blocked
		StatusDescription string `json:"StatusDescription"`
	} `json:"LatestDeployment"`
}

// waitForJobHealthy polls `nomad job status` until the latest deployment
// reaches a terminal state. Returns nil on "successful", error otherwise.
func waitForJobHealthy(ctx context.Context, nomadAddr, jobName string, logf func(string, ...any)) error {
	deadline := time.Now().Add(10 * time.Minute)
	var lastDeployment string
	for {
		if time.Now().After(deadline) {
			return errors.New("nomad deployment did not become healthy within 10m")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		st, err := readJobStatus(ctx, nomadAddr, jobName)
		if err != nil {
			// Status may not be available immediately after submit; retry briefly.
			logf("status check error: %s", err)
		} else {
			if st.LatestDeployment != nil && st.LatestDeployment.ID != lastDeployment {
				lastDeployment = st.LatestDeployment.ID
				logf("deployment %s: %s", st.LatestDeployment.ID, st.LatestDeployment.Status)
			}
			if st.LatestDeployment != nil {
				switch st.LatestDeployment.Status {
				case "successful":
					return nil
				case "failed", "cancelled", "blocked":
					return fmt.Errorf("deployment %s: %s — %s",
						st.LatestDeployment.ID, st.LatestDeployment.Status, st.LatestDeployment.StatusDescription)
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

func readJobStatus(ctx context.Context, nomadAddr, jobName string) (*jobStatus, error) {
	cmd := exec.CommandContext(ctx, "nomad", "job", "status", "-json", jobName)
	cmd.Env = append(cmd.Environ(), "NOMAD_ADDR="+nomadAddr)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var st jobStatus
	if err := json.Unmarshal(stdout.Bytes(), &st); err != nil {
		// `nomad job status -json` can return an array for some versions; try.
		var arr []jobStatus
		if err2 := json.Unmarshal(stdout.Bytes(), &arr); err2 == nil && len(arr) > 0 {
			return &arr[0], nil
		}
		return nil, fmt.Errorf("parse status: %w", err)
	}
	return &st, nil
}

// streamReader is a tiny helper for line-buffered streaming of command output.
// Used by hooks.go for `docker run`.
func streamReader(r io.Reader, logf func(string, ...any)) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		logf("%s", sc.Text())
	}
}
