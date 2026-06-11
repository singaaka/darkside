package server

import (
	"context"
	"net/http"

	"github.com/singaaka/darkside-fleet/gen/go/fleet/v1/fleetv1connect"
	"github.com/singaaka/darkside-fleet/internal/ansible"
	"github.com/singaaka/darkside-fleet/internal/queue"
	"github.com/singaaka/darkside-fleet/internal/store"
)

// Options contains dependencies injected into the server.
type Options struct {
	Store       *store.Store
	Queue       *queue.Queue
	Runner      *ansible.Runner
	PlaybookDir string
	Frontend    http.Handler
}

// Server holds all state for the HTTP server.
type Server struct {
	opts Options
}

func New(opts Options) *Server {
	return &Server{opts: opts}
}

// Handler returns the root HTTP handler with all routes wired up.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// ── ConnectRPC services ───────────────────────────────────────────────────
	nodePath, nodeH := fleetv1connect.NewNodeServiceHandler(&nodeHandler{s: s})
	mux.Handle(nodePath, nodeH)

	cfgPath, cfgH := fleetv1connect.NewClusterConfigServiceHandler(&clusterConfigHandler{s: s})
	mux.Handle(cfgPath, cfgH)

	jobPath, jobH := fleetv1connect.NewJobServiceHandler(&jobHandler{s: s})
	mux.Handle(jobPath, jobH)

	settingsPath, settingsH := fleetv1connect.NewSettingsServiceHandler(&settingsHandler{s: s})
	mux.Handle(settingsPath, settingsH)

	pbPath, pbH := fleetv1connect.NewPlaybookServiceHandler(&playbookHandler{s: s})
	mux.Handle(pbPath, pbH)

	// Health check.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})

	// SSE endpoint for live job log streaming.
	mux.HandleFunc("/api/jobs/", s.handleJobSSE)

	// SPA frontend.
	mux.Handle("/", s.opts.Frontend)
	return mux
}

// suppress unused
var _ context.Context
