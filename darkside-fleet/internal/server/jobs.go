package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/singaaka/darkside-fleet/gen/go/fleet/v1"
	"github.com/singaaka/darkside-fleet/internal/db/dbgen"
)

type jobHandler struct{ s *Server }

func (h *jobHandler) List(ctx context.Context, req *connect.Request[v1.ListJobsRequest]) (*connect.Response[v1.ListJobsResponse], error) {
	limit := int64(req.Msg.Limit)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	jobs, err := h.s.opts.Store.ListJobs(ctx, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*v1.Job, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, jobToProto(j))
	}
	return connect.NewResponse(&v1.ListJobsResponse{Jobs: out}), nil
}

func (h *jobHandler) Get(ctx context.Context, req *connect.Request[v1.GetJobRequest]) (*connect.Response[v1.Job], error) {
	j, err := h.s.opts.Store.GetJob(ctx, req.Msg.Id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("job not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(jobToProto(j)), nil
}

// handleJobSSE streams live job output over Server-Sent Events.
// GET /api/jobs/:id/stream
func (s *Server) handleJobSSE(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}
	jobID := parts[2] // /api/jobs/:id/stream → parts = ["api","jobs",":id","stream"]

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	var lastLen int
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			j, err := s.opts.Store.GetJob(r.Context(), jobID)
			if err != nil {
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
				flusher.Flush()
				return
			}
			if len(j.Output) > lastLen {
				newData := j.Output[lastLen:]
				fmt.Fprintf(w, "data: %s\n\n", strings.ReplaceAll(newData, "\n", "\ndata: "))
				flusher.Flush()
				lastLen = len(j.Output)
			}
			if j.Status == "done" || j.Status == "failed" {
				fmt.Fprintf(w, "event: done\ndata: %s\n\n", j.Status)
				flusher.Flush()
				return
			}
		}
	}
}

func jobToProto(j dbgen.Job) *v1.Job {
	out := &v1.Job{
		Id:            j.ID,
		Type:          j.Type,
		Status:        j.Status,
		Payload:       j.Payload,
		Output:        j.Output,
		CreatedAtUnix: j.CreatedAt.Unix(),
	}
	if j.StartedAt != nil {
		out.StartedAtUnix = j.StartedAt.Unix()
	}
	if j.FinishedAt != nil {
		out.FinishedAtUnix = j.FinishedAt.Unix()
	}
	return out
}
