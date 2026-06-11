// Package deployer — Nomad HTTP API client.
// Replaces the old nomadcli.go (which used the nomad CLI binary).
// All Nomad communication now goes through the HTTP API directly so the
// darkside container doesn't need the nomad binary installed.
package deployer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// nomadClient wraps calls to the Nomad HTTP API.
type nomadClient struct {
	addr string // e.g. "http://127.0.0.1:4646"
}

func newNomadClient(addr string) *nomadClient {
	return &nomadClient{addr: strings.TrimRight(addr, "/")}
}

func (c *nomadClient) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.addr+path, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return http.DefaultClient.Do(req)
}

func (c *nomadClient) doJSON(ctx context.Context, method, path string, body any, out any) error {
	resp, err := c.do(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("nomad %s %s: %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(rb)))
	}
	if out != nil {
		return json.Unmarshal(rb, out)
	}
	return nil
}

// ── Job submission ────────────────────────────────────────────────────────────

// submitHCLJob submits a raw HCL2 job spec. Nomad parses it server-side.
// Returns the eval ID.
func (c *nomadClient) submitHCLJob(ctx context.Context, hcl string, logf func(string, ...any)) (string, error) {
	payload := map[string]any{
		"JobHCL":       hcl,
		"Canonicalize": false,
	}
	var parseOut struct {
		Job json.RawMessage `json:"Job"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/jobs/parse", payload, &parseOut); err != nil {
		return "", fmt.Errorf("parse job HCL: %w", err)
	}

	submitPayload := map[string]any{
		"Job": json.RawMessage(parseOut.Job),
	}
	var submitOut struct {
		EvalID string `json:"EvalID"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/jobs", submitPayload, &submitOut); err != nil {
		return "", fmt.Errorf("submit job: %w", err)
	}
	if submitOut.EvalID != "" {
		logf("nomad eval %s", submitOut.EvalID)
	}
	return submitOut.EvalID, nil
}

// scaleJob updates the task group count for a running service job.
func (c *nomadClient) scaleJob(ctx context.Context, jobName string, count int) error {
	payload := map[string]any{
		"Count": count,
		"Target": map[string]string{
			"Job":   jobName,
			"Group": "app",
		},
		"Message": fmt.Sprintf("scaled to %d by darkside", count),
	}
	return c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/v1/job/%s/scale", jobName), payload, nil)
}

// ── Job status polling ────────────────────────────────────────────────────────

type deploymentStatus struct {
	ID                string `json:"ID"`
	Status            string `json:"Status"`
	StatusDescription string `json:"StatusDescription"`
}

type jobStatusResp struct {
	ID                string            `json:"ID"`
	Status            string            `json:"Status"`
	LatestDeployment  *deploymentStatus `json:"LatestDeployment"`
}

func (c *nomadClient) waitForJobHealthy(ctx context.Context, jobName string, logf func(string, ...any)) error {
	deadline := time.Now().Add(10 * time.Minute)
	var lastDepID string
	for {
		if time.Now().After(deadline) {
			return errors.New("nomad deployment did not become healthy within 10m")
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		var st jobStatusResp
		if err := c.doJSON(ctx, http.MethodGet, "/v1/job/"+jobName, nil, &st); err != nil {
			logf("status check error: %s", err)
		} else {
			if d := st.LatestDeployment; d != nil && d.ID != lastDepID {
				lastDepID = d.ID
				logf("deployment %s: %s", d.ID, d.Status)
			}
			if d := st.LatestDeployment; d != nil {
				switch d.Status {
				case "successful":
					return nil
				case "failed", "cancelled", "blocked":
					return fmt.Errorf("deployment %s: %s — %s", d.ID, d.Status, d.StatusDescription)
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

// ── Batch job polling ─────────────────────────────────────────────────────────

type batchJobStatus struct {
	Status string `json:"Status"` // "pending"|"running"|"dead"
}

// waitForBatchJob polls a batch job until it exits (Status=dead) or the context
// is cancelled. Returns nil if all tasks succeeded, error otherwise.
func (c *nomadClient) waitForBatchJob(ctx context.Context, jobName string, logf func(string, ...any)) error {
	deadline := time.Now().Add(30 * time.Minute)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("batch job %s did not complete within 30m", jobName)
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		var st batchJobStatus
		if err := c.doJSON(ctx, http.MethodGet, "/v1/job/"+jobName, nil, &st); err != nil {
			logf("status check: %s", err)
		} else if st.Status == "dead" {
			return c.checkBatchJobSuccess(ctx, jobName)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

type allocSummary struct {
	ID       string `json:"ID"`
	TaskStates map[string]struct {
		Failed   bool   `json:"Failed"`
		State    string `json:"State"`
	} `json:"TaskStates"`
}

func (c *nomadClient) checkBatchJobSuccess(ctx context.Context, jobName string) error {
	var allocs []allocSummary
	if err := c.doJSON(ctx, http.MethodGet, "/v1/job/"+jobName+"/allocations", nil, &allocs); err != nil {
		return fmt.Errorf("get allocs: %w", err)
	}
	for _, a := range allocs {
		for task, state := range a.TaskStates {
			if state.Failed {
				return fmt.Errorf("task %s in alloc %s failed", task, a.ID[:8])
			}
		}
	}
	return nil
}

// ── Log streaming from alloc ──────────────────────────────────────────────────

// streamAllocLogs streams logs from a Nomad alloc's task to logf. This is
// best-effort — if the alloc isn't found we fall back to the persisted log.
func (c *nomadClient) streamAllocLogs(ctx context.Context, jobName, taskName string, logf func(string, ...any)) error {
	// Find the latest alloc for the job.
	var allocs []struct {
		ID     string `json:"ID"`
		Status string `json:"ClientStatus"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/v1/job/"+jobName+"/allocations", nil, &allocs); err != nil {
		return err
	}
	if len(allocs) == 0 {
		return errors.New("no allocations found")
	}
	allocID := allocs[len(allocs)-1].ID

	// Stream stdout from the task.
	url := fmt.Sprintf("%s/v1/client/fs/logs/%s?task=%s&type=stdout&follow=true&plain=true",
		c.addr, allocID, taskName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		logf("%s", sc.Text())
	}
	return sc.Err()
}
