// Package queue is a SQLite-backed serial job queue.
// One worker goroutine processes jobs one at a time — cluster mutations must
// be serial to avoid split-brain races in Nomad/Consul.
package queue

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/singaaka/darkside-fleet/internal/db/dbgen"
	"github.com/singaaka/darkside-fleet/internal/store"
)

// Handler is a function that executes a job. It receives the job and a logf
// function to append output to the job row in real time.
type Handler func(ctx context.Context, job dbgen.Job, logf func(string)) error

// Queue polls the jobs table and dispatches pending jobs to registered handlers.
type Queue struct {
	store    *store.Store
	handlers map[string]Handler
	interval time.Duration
}

func New(st *store.Store) *Queue {
	return &Queue{
		store:    st,
		handlers: map[string]Handler{},
		interval: time.Second,
	}
}

// Register associates a job type with a handler function.
func (q *Queue) Register(jobType string, h Handler) {
	q.handlers[jobType] = h
}

// Run starts the worker loop. Blocks until ctx is cancelled.
func (q *Queue) Run(ctx context.Context) {
	slog.Info("job queue started")
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(q.interval):
			if err := q.processOne(ctx); err != nil && !errors.Is(err, sql.ErrNoRows) {
				slog.Error("queue: process job", "error", err)
			}
		}
	}
}

func (q *Queue) processOne(ctx context.Context) error {
	job, err := q.store.GetPendingJob(ctx)
	if err != nil {
		return err // includes sql.ErrNoRows when queue is empty
	}

	h, ok := q.handlers[job.Type]
	if !ok {
		slog.Warn("queue: no handler for job type", "type", job.Type, "id", job.ID)
		return q.store.FinishJob(ctx, dbgen.FinishJobParams{Status: "failed", ID: job.ID})
	}

	_ = q.store.StartJob(ctx, job.ID)
	slog.Info("queue: starting job", "type", job.Type, "id", job.ID)

	logf := func(line string) {
		if line != "" && line[len(line)-1] != '\n' {
			line += "\n"
		}
		_ = q.store.AppendJobOutput(ctx, line, job.ID)
	}

	err = h(ctx, job, logf)
	status := "done"
	if err != nil {
		logf("ERROR: " + err.Error())
		status = "failed"
		slog.Error("queue: job failed", "type", job.Type, "id", job.ID, "error", err)
	} else {
		slog.Info("queue: job done", "type", job.Type, "id", job.ID)
	}
	return q.store.FinishJob(ctx, dbgen.FinishJobParams{Status: status, ID: job.ID})
}
