package builder

// nomadbridge.go — thin re-export so builder can call the Nomad HTTP API
// without importing deployer (which would create a circular dependency).

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

// nomadClientBuilder is a local copy of the Nomad HTTP client methods that
// builder.go needs. It mirrors deployer/nomadapi.go but lives here to avoid
// a circular import (deployer → builder → deployer).
type nomadClientBuilder struct {
	addr string
}

func newNomadClientForBuilder(addr string) *nomadClientBuilder {
	return &nomadClientBuilder{addr: strings.TrimRight(addr, "/")}
}

func (c *nomadClientBuilder) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.addr+path, r)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
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

func (c *nomadClientBuilder) submitHCLJob(ctx context.Context, hcl string, logf func(string, ...any)) (string, error) {
	var parseOut struct {
		Job json.RawMessage `json:"Job"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/jobs/parse", map[string]any{"JobHCL": hcl, "Canonicalize": false}, &parseOut); err != nil {
		return "", fmt.Errorf("parse HCL: %w", err)
	}
	var submitOut struct {
		EvalID string `json:"EvalID"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/jobs", map[string]any{"Job": json.RawMessage(parseOut.Job)}, &submitOut); err != nil {
		return "", fmt.Errorf("submit: %w", err)
	}
	if submitOut.EvalID != "" {
		logf("nomad eval %s", submitOut.EvalID)
	}
	return submitOut.EvalID, nil
}

func (c *nomadClientBuilder) waitForBatchJob(ctx context.Context, jobName string, logf func(string, ...any)) error {
	deadline := time.Now().Add(30 * time.Minute)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("batch job %s did not complete within 30m", jobName)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		var st struct {
			Status string `json:"Status"`
		}
		if err := c.doJSON(ctx, http.MethodGet, "/v1/job/"+jobName, nil, &st); err != nil {
			logf("status check: %s", err)
		} else if st.Status == "dead" {
			return c.checkBatchSuccess(ctx, jobName)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func (c *nomadClientBuilder) checkBatchSuccess(ctx context.Context, jobName string) error {
	var allocs []struct {
		TaskStates map[string]struct {
			Failed bool `json:"Failed"`
		} `json:"TaskStates"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/v1/job/"+jobName+"/allocations", nil, &allocs); err != nil {
		return fmt.Errorf("get allocs: %w", err)
	}
	for _, a := range allocs {
		for task, state := range a.TaskStates {
			if state.Failed {
				return fmt.Errorf("task %s failed", task)
			}
		}
	}
	return nil
}

func (c *nomadClientBuilder) streamAllocLogs(ctx context.Context, jobName, taskName string, logf func(string, ...any)) error {
	var allocs []struct {
		ID string `json:"ID"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/v1/job/"+jobName+"/allocations", nil, &allocs); err != nil {
		return err
	}
	if len(allocs) == 0 {
		return errors.New("no allocs")
	}
	url := fmt.Sprintf("%s/v1/client/fs/logs/%s?task=%s&type=stdout&follow=true&plain=true", c.addr, allocs[len(allocs)-1].ID, taskName)
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
