package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/singaaka/darkside-paas/internal/builder"
	"github.com/singaaka/darkside-paas/internal/db/dbgen"
	"github.com/singaaka/darkside-paas/internal/github"
)

type pushEvent struct {
	Ref        string `json:"ref"`
	After      string `json:"after"`
	HeadCommit struct {
		Message string `json:"message"`
	} `json:"head_commit"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
}

func verifyWebhookSignature(secret, body, signatureHeader string) error {
	if !strings.HasPrefix(signatureHeader, "sha256=") {
		return errors.New("missing sha256= prefix")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signatureHeader)) {
		return errors.New("signature mismatch")
	}
	return nil
}

func (s *Server) handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	gh, err := s.opts.Store.GetGitHubApp(r.Context())
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "github app not connected", http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 5<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	if err := verifyWebhookSignature(gh.WebhookSecret, string(body), r.Header.Get("X-Hub-Signature-256")); err != nil {
		slog.Warn("webhook signature invalid", "error", err)
		http.Error(w, "invalid signature", http.StatusForbidden)
		return
	}

	event := r.Header.Get("X-GitHub-Event")
	if event == "ping" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"pong":true}`))
		return
	}
	if event != "push" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var p pushEvent
	if err := json.Unmarshal(body, &p); err != nil {
		http.Error(w, "decode push: "+err.Error(), http.StatusBadRequest)
		return
	}
	branch := strings.TrimPrefix(p.Ref, "refs/heads/")
	if branch == p.Ref {
		w.WriteHeader(http.StatusOK)
		return
	}

	app, err := s.opts.Store.GetAppByRepo(r.Context(), p.Repository.FullName)
	if errors.Is(err, sql.ErrNoRows) {
		slog.Info("webhook: no darkside app for repo", "repo", p.Repository.FullName)
		w.WriteHeader(http.StatusOK)
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	// Only deploy if the pushed branch matches the app's configured deploy branch.
	if branch != app.Branch {
		slog.Info("webhook: branch not deploy branch, skipping", "branch", branch, "deploy_branch", app.Branch)
		w.WriteHeader(http.StatusOK)
		return
	}

	depID := uuid.NewString()
	br := branch
	if err := s.opts.Store.CreateDeployment(r.Context(), dbgen.CreateDeploymentParams{
		ID:            depID,
		AppID:         app.ID,
		CommitSha:     p.After,
		CommitMessage: strings.SplitN(p.HeadCommit.Message, "\n", 2)[0],
		Status:        "pending",
		TriggerType:   "github",
		TriggerBranch: &br,
	}); err != nil {
		http.Error(w, "create deployment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	job := &buildJob{
		s:         s,
		gh:        github.NewApp(gh.AppID, gh.PrivateKey),
		installID: app.InstallationID,
		job: builder.Job{
			DeploymentID: depID,
			AppID:        app.ID,
			AppName:      app.Name,
			RepoFullName: app.RepoFullName,
			Branch:       branch,
			CommitSHA:    p.After,
			CommitMsg:    strings.SplitN(p.HeadCommit.Message, "\n", 2)[0],
			TriggerType:  "github",
		},
	}
	if ok := s.queue.Submit(job); !ok {
		slog.Warn("webhook: queue full", "app", app.Name)
		_ = s.opts.Store.UpdateDeploymentStatus(r.Context(), dbgen.UpdateDeploymentStatusParams{
			ID:     depID,
			Status: "failed",
			Error:  strPtr("build queue full"),
		})
		http.Error(w, "queue full", http.StatusServiceUnavailable)
		return
	}

	slog.Info("webhook: enqueued", "app", app.Name, "branch", branch, "sha", p.After, "deployment", depID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"deployment_id":"` + depID + `"}`))
}

// buildJob implements buildqueue.Job.
type buildJob struct {
	s         *Server
	gh        *github.App
	installID int64
	job       builder.Job
}

func (j *buildJob) AppID() string { return j.job.AppID }
func (j *buildJob) Run(ctx context.Context) {
	b := &builder.Builder{
		Store:        j.s.opts.Store,
		Hub:          j.s.hub,
		Deployer:     j.s.deployer,
		NomadAddr:    j.s.opts.Config.NomadAddr,
		RegistryAddr: j.s.opts.Config.RegistryAddr,
	}
	b.Run(ctx, j.gh, j.installID, j.job)
}

func strPtr(s string) *string { return &s }
