package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/singaaka/darkside-paas/gen/go/darkside/v1/darksidev1connect"
	"github.com/singaaka/darkside-paas/internal/builder"
	"github.com/singaaka/darkside-paas/internal/buildqueue"
	"github.com/singaaka/darkside-paas/internal/config"
	"github.com/singaaka/darkside-paas/internal/deployer"
	"github.com/singaaka/darkside-paas/internal/github"
	"github.com/singaaka/darkside-paas/internal/loghub"
	"github.com/singaaka/darkside-paas/internal/store"
)

type Options struct {
	Config   *config.Config
	Store    *store.Store
	Version  string
	Frontend http.Handler
}

type Server struct {
	opts           Options
	manifestStates *manifestStates
	hub            *loghub.Hub
	queue          *buildqueue.Queue
	deployer       *deployer.Deployer
}

func New(ctx context.Context, opts Options) *Server {
	hub := loghub.New()
	return &Server{
		opts:           opts,
		manifestStates: newManifestStates(),
		hub:            hub,
		queue:          buildqueue.New(ctx, 64),
		deployer: &deployer.Deployer{
			Store:       opts.Store,
			Hub:         hub,
			NomadAddr:   opts.Config.NomadAddr,
			RegistryAddr: opts.Config.RegistryAddr,
		},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// ── ConnectRPC services (existing) ───────────────────────────────────────
	healthPath, healthH := darksidev1connect.NewHealthServiceHandler(&healthHandler{
		version: s.opts.Version,
		domain:  s.opts.Config.Host(),
	})
	mux.Handle(healthPath, healthH)

	githubPath, githubH := darksidev1connect.NewGitHubServiceHandler(&githubHandler{store: s.opts.Store})
	mux.Handle(githubPath, githubH)

	appsPath, appsH := darksidev1connect.NewAppServiceHandler(&appsHandler{store: s.opts.Store, cfg: s.opts.Config})
	mux.Handle(appsPath, appsH)

	depPath, depH := darksidev1connect.NewDeploymentServiceHandler(&deploymentsHandler{
		store: s.opts.Store,
		hub:   s.hub,
		queue: s.queue,
		makeJob: func(j builder.Job, gh *github.App, installID int64) buildqueue.Job {
			return &buildJob{s: s, gh: gh, installID: installID, job: j}
		},
	})
	mux.Handle(depPath, depH)

	// ── Plain HTTP routes ─────────────────────────────────────────────────────
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"version":"` + s.opts.Version + `","host":"` + s.opts.Config.Host() + `"}`))
	})

	// GitHub App manifest flow (also used for OAuth registration).
	mux.HandleFunc("/github/manifest/start", s.handleManifestStart)
	mux.HandleFunc("/github/manifest/callback", s.handleManifestCallback)

	// Webhooks bypass auth (high-priority in Traefik config).
	mux.HandleFunc("/webhooks/github", s.handleGitHubWebhook)

	// Auth routes — no session required.
	mux.HandleFunc("/auth/github", s.handleAuthGitHub)
	mux.HandleFunc("/auth/callback", s.handleAuthCallback)
	mux.HandleFunc("/auth/logout", s.handleAuthLogout)
	// Traefik forward-auth endpoint (session required implicitly via cookie).
	mux.HandleFunc("/auth/verify", s.handleAuthVerify)

	// ── REST API (session required) ───────────────────────────────────────────
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/v1/me", s.handleMe)
	apiMux.HandleFunc("/api/v1/users", s.handleListUsers)
	apiMux.HandleFunc("/api/v1/users/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/whitelist") {
			s.handleWhitelistUser(w, r)
		} else {
			http.NotFound(w, r)
		}
	})
	apiMux.HandleFunc("/api/v1/apps", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			s.handleCreateAppJSON(w, r)
		} else {
			http.NotFound(w, r)
		}
	})
	apiMux.HandleFunc("/api/v1/apps/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case r.Method == http.MethodGet && !strings.HasSuffix(path, "/"):
			s.handleGetAppDetail(w, r)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/rotate-key"):
			s.handleRotateKey(w, r)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/scale"):
			s.handleScale(w, r)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/deploy"):
			s.handleManualDeploy(w, r)
		default:
			http.NotFound(w, r)
		}
	})
	apiMux.HandleFunc("/api/v1/deployments/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case r.Method == http.MethodGet && !strings.HasSuffix(path, "/"):
			s.handleGetDeploymentDetail(w, r)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/redeploy"):
			s.handleRedeployJSON(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	mux.Handle("/api/v1/", s.authMiddleware(requireAuth(apiMux)))

	// Auth middleware on everything else (attaches user to context but doesn't require it).
	mux.Handle("/", s.authMiddleware(s.opts.Frontend))
	return mux
}
