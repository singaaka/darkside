package server

import (
	"context"
	"net/http"

	"github.com/singaaka/darkside-node-manager/gen/go/nm/v1/nmv1connect"
	"github.com/singaaka/darkside-node-manager/internal/ansible"
	"github.com/singaaka/darkside-node-manager/internal/queue"
	"github.com/singaaka/darkside-node-manager/internal/store"
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
	nodePath, nodeH := nmv1connect.NewNodeServiceHandler(&nodeHandler{s: s})
	mux.Handle(nodePath, nodeH)

	cfgPath, cfgH := nmv1connect.NewClusterConfigServiceHandler(&clusterConfigHandler{s: s})
	mux.Handle(cfgPath, cfgH)

	jobPath, jobH := nmv1connect.NewJobServiceHandler(&jobHandler{s: s})
	mux.Handle(jobPath, jobH)

	settingsPath, settingsH := nmv1connect.NewSettingsServiceHandler(&settingsHandler{s: s})
	mux.Handle(settingsPath, settingsH)

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
