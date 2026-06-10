package server

import (
	"context"
	"net/http"

	"github.com/singaaka/darkside/gen/go/darkside/v1/darksidev1connect"
	"github.com/singaaka/darkside/internal/builder"
	"github.com/singaaka/darkside/internal/buildqueue"
	"github.com/singaaka/darkside/internal/config"
	"github.com/singaaka/darkside/internal/deployer"
	"github.com/singaaka/darkside/internal/github"
	"github.com/singaaka/darkside/internal/loghub"
	"github.com/singaaka/darkside/internal/store"
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
			Store:     opts.Store,
			Hub:       hub,
			NomadAddr: opts.Config.NomadAddr,
		},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	healthPath, healthH := darksidev1connect.NewHealthServiceHandler(&healthHandler{
		version: s.opts.Version,
		domain:  s.opts.Config.Domain,
	})
	mux.Handle(healthPath, healthH)

	githubPath, githubH := darksidev1connect.NewGitHubServiceHandler(&githubHandler{store: s.opts.Store})
	mux.Handle(githubPath, githubH)

	appsPath, appsH := darksidev1connect.NewAppServiceHandler(&appsHandler{store: s.opts.Store, cfg: s.opts.Config})
	mux.Handle(appsPath, appsH)

	envPath, envH := darksidev1connect.NewEnvironmentServiceHandler(&envHandler{store: s.opts.Store})
	mux.Handle(envPath, envH)

	depPath, depH := darksidev1connect.NewDeploymentServiceHandler(&deploymentsHandler{
		store: s.opts.Store,
		hub:   s.hub,
		queue: s.queue,
		makeJob: func(j builder.Job, gh *github.App, installID int64) buildqueue.Job {
			return &buildJob{s: s, gh: gh, installID: installID, job: j}
		},
	})
	mux.Handle(depPath, depH)

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"version":"` + s.opts.Version + `","domain":"` + s.opts.Config.Domain + `"}`))
	})

	mux.HandleFunc("/github/manifest/start", s.handleManifestStart)
	mux.HandleFunc("/github/manifest/callback", s.handleManifestCallback)
	mux.HandleFunc("/webhooks/github", s.handleGitHubWebhook)

	mux.Handle("/", s.opts.Frontend)
	return mux
}
