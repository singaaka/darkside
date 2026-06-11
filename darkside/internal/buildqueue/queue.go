// Package buildqueue is the in-memory work queue feeding the builder. One FIFO
// per app (so two pushes to the same repo serialise) with goroutine-per-app.
//
// Queue depth is in-memory only — restarts drop pending jobs. Because GitHub
// retries webhooks on 5xx but not on 2xx, we accept the job + 200 first, then
// process. Crash-after-accept means a missed deploy until the next push.
// Tradeoff documented for Phase 4; durability lands later if needed.
package buildqueue

import (
	"context"
	"log/slog"
	"sync"
)

type Job interface {
	AppID() string
	Run(ctx context.Context)
}

type Queue struct {
	mu      sync.Mutex
	workers map[string]chan Job
	cap     int
	ctx     context.Context
}

func New(ctx context.Context, perAppCapacity int) *Queue {
	if perAppCapacity <= 0 {
		perAppCapacity = 64
	}
	return &Queue{
		workers: map[string]chan Job{},
		cap:     perAppCapacity,
		ctx:     ctx,
	}
}

// Submit enqueues a job. Returns false if the per-app queue is full.
func (q *Queue) Submit(job Job) bool {
	q.mu.Lock()
	ch, ok := q.workers[job.AppID()]
	if !ok {
		ch = make(chan Job, q.cap)
		q.workers[job.AppID()] = ch
		go q.run(job.AppID(), ch)
	}
	q.mu.Unlock()

	select {
	case ch <- job:
		return true
	default:
		return false
	}
}

func (q *Queue) run(appID string, ch chan Job) {
	for {
		select {
		case <-q.ctx.Done():
			return
		case job := <-ch:
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("buildqueue: worker panicked", "app", appID, "recover", r)
					}
				}()
				job.Run(q.ctx)
			}()
		}
	}
}
