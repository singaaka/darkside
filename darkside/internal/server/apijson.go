package server

// apijson.go — plain JSON REST handlers at /api/v1/
// All new functionality (app detail with age keys, key rotation, scaling,
// manual deploys, user management) lives here so we never touch the generated
// proto/connect files.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/singaaka/darkside/internal/ageenv"
	"github.com/singaaka/darkside/internal/builder"
	"github.com/singaaka/darkside/internal/db/dbgen"
	"github.com/singaaka/darkside/internal/github"
)

// jsonOK writes v as JSON with status 200.
func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// jsonErr writes an error JSON response.
func jsonErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	return json.NewDecoder(r.Body).Decode(v)
}

// ── GET /api/v1/me ─────────────────────────────────────────────────────────

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u := userFromCtx(r.Context())
	if u == nil {
		jsonErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	jsonOK(w, map[string]any{
		"id":          u.ID,
		"login":       u.Login,
		"email":       u.Email,
		"is_admin":    u.IsAdmin,
		"whitelisted": u.Whitelisted,
	})
}

// ── GET /api/v1/users   POST /api/v1/users/:id/whitelist ──────────────────

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	u := userFromCtx(r.Context())
	if u == nil || !u.IsAdmin {
		jsonErr(w, http.StatusForbidden, "admin required")
		return
	}
	users, err := s.opts.Store.ListUsers(r.Context())
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, users)
}

func (s *Server) handleWhitelistUser(w http.ResponseWriter, r *http.Request) {
	u := userFromCtx(r.Context())
	if u == nil || !u.IsAdmin {
		jsonErr(w, http.StatusForbidden, "admin required")
		return
	}
	id := pathSegment(r.URL.Path, 4) // /api/v1/users/:id/whitelist
	if id == "" {
		jsonErr(w, http.StatusBadRequest, "missing user id")
		return
	}
	var userID int64
	if _, err := fmt.Sscanf(id, "%d", &userID); err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := s.opts.Store.WhitelistUser(r.Context(), userID); err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// ── GET /api/v1/apps/:id ──────────────────────────────────────────────────

func (s *Server) handleGetAppDetail(w http.ResponseWriter, r *http.Request) {
	id := pathSegment(r.URL.Path, 3) // /api/v1/apps/:id
	a, err := s.opts.Store.GetApp(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErr(w, http.StatusNotFound, "app not found")
		return
	}
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, appDetailJSON(a))
}

func appDetailJSON(a dbgen.App) map[string]any {
	return map[string]any{
		"id":              a.ID,
		"name":            a.Name,
		"repo_full_name":  a.RepoFullName,
		"installation_id": a.InstallationID,
		"created_at":      a.CreatedAt.Unix(),
		"branch":          a.Branch,
		"env_file":        a.EnvFile,
		"age_public_key":  a.AgePublicKey,
		"age_key_id":      a.AgeKeyID,
		// private key deliberately omitted from list endpoint — only shown once
	}
}

// ── POST /api/v1/apps (create with full fields) ───────────────────────────

func (s *Server) handleCreateAppJSON(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           string `json:"name"`
		RepoFullName   string `json:"repo_full_name"`
		InstallationID int64  `json:"installation_id"`
		Branch         string `json:"branch"`
		EnvFile        string `json:"env_file"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if !appNameRe.MatchString(req.Name) {
		jsonErr(w, http.StatusBadRequest, "invalid app name")
		return
	}
	if req.Branch == "" {
		req.Branch = "main"
	}
	if req.EnvFile == "" {
		req.EnvFile = "env.age"
	}

	kp, err := ageenv.GenerateKeypair()
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	id := uuid.NewString()
	if err := s.opts.Store.CreateApp(r.Context(), dbgen.CreateAppParams{
		ID:             id,
		Name:           req.Name,
		RepoFullName:   req.RepoFullName,
		InstallationID: req.InstallationID,
		Branch:         req.Branch,
		EnvFile:        req.EnvFile,
		AgePublicKey:   kp.PublicKey,
		AgePrivateKey:  kp.PrivateKey,
		AgeKeyID:       "key-v1",
	}); err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	a, _ := s.opts.Store.GetApp(r.Context(), id)

	// Return the private key only on creation — never shown again.
	out := appDetailJSON(a)
	out["age_private_key"] = kp.PrivateKey
	out["setup_instructions"] = buildAgeSetup(a.Name, "key-v1", kp.PublicKey, kp.PrivateKey, a.EnvFile)
	jsonOK(w, out)
}

// ── POST /api/v1/apps/:id/rotate-key ─────────────────────────────────────

func (s *Server) handleRotateKey(w http.ResponseWriter, r *http.Request) {
	id := pathSegment(r.URL.Path, 3) // /api/v1/apps/:id/rotate-key
	a, err := s.opts.Store.GetApp(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErr(w, http.StatusNotFound, "app not found")
		return
	}
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Increment key ID: key-v1 → key-v2, key-v2 → key-v3, etc.
	newKeyID := incrementKeyID(a.AgeKeyID)
	kp, err := ageenv.GenerateKeypair()
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.opts.Store.UpdateAppAgeKey(r.Context(), dbgen.UpdateAppAgeKeyParams{
		AgePublicKey:  kp.PublicKey,
		AgePrivateKey: kp.PrivateKey,
		AgeKeyID:      newKeyID,
		ID:            id,
	}); err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, map[string]any{
		"age_key_id":         newKeyID,
		"age_public_key":     kp.PublicKey,
		"age_private_key":    kp.PrivateKey,
		"setup_instructions": buildAgeSetup(a.Name, newKeyID, kp.PublicKey, kp.PrivateKey, a.EnvFile),
		"toml_snippet":       buildKeyTomlSnippet(newKeyID, kp.PublicKey, a.EnvFile),
	})
}

// ── POST /api/v1/apps/:id/scale ──────────────────────────────────────────

func (s *Server) handleScale(w http.ResponseWriter, r *http.Request) {
	id := pathSegment(r.URL.Path, 3)
	a, err := s.opts.Store.GetApp(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErr(w, http.StatusNotFound, "app not found")
		return
	}
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	var req struct {
		Count int `json:"count"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Count < 1 || req.Count > 20 {
		jsonErr(w, http.StatusBadRequest, "count must be 1-20")
		return
	}

	// Scale by updating the running Nomad job directly.
	if err := s.deployer.ScaleJob(r.Context(), a.Name, req.Count); err != nil {
		jsonErr(w, http.StatusBadGateway, "nomad scale: "+err.Error())
		return
	}

	jsonOK(w, map[string]any{
		"ok":      true,
		"count":   req.Count,
		"warning": "This is temporary. To make it permanent, set count=" + fmt.Sprintf("%d", req.Count) + " in your darkside.toml and push a commit.",
	})
}

// ── POST /api/v1/apps/:id/deploy (manual deploy) ─────────────────────────

func (s *Server) handleManualDeploy(w http.ResponseWriter, r *http.Request) {
	id := pathSegment(r.URL.Path, 3)
	a, err := s.opts.Store.GetApp(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErr(w, http.StatusNotFound, "app not found")
		return
	}
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	var req struct {
		Branch    string `json:"branch"`     // optional — defaults to app.Branch
		CommitSHA string `json:"commit_sha"` // optional — uses branch HEAD if empty
	}
	_ = decodeJSON(r, &req)
	branch := req.Branch
	if branch == "" {
		branch = a.Branch
	}

	gh, err := s.opts.Store.GetGitHubApp(r.Context())
	if errors.Is(err, sql.ErrNoRows) {
		jsonErr(w, http.StatusServiceUnavailable, "github app not connected")
		return
	}

	// If no commit SHA provided, resolve branch HEAD via GitHub API.
	sha := req.CommitSHA
	if sha == "" {
		ghApp := github.NewApp(gh.AppID, gh.PrivateKey)
		token, err := ghApp.InstallationToken(r.Context(), a.InstallationID)
		if err != nil {
			jsonErr(w, http.StatusBadGateway, "github token: "+err.Error())
			return
		}
		sha, err = resolveBranchHead(r.Context(), a.RepoFullName, branch, token)
		if err != nil {
			jsonErr(w, http.StatusBadGateway, "resolve branch: "+err.Error())
			return
		}
	}

	depID := uuid.NewString()
	br := branch
	if err := s.opts.Store.CreateDeployment(r.Context(), dbgen.CreateDeploymentParams{
		ID:            depID,
		AppID:         a.ID,
		CommitSha:     sha,
		CommitMessage: "manual deploy",
		Status:        "pending",
		TriggerType:   "manual",
		TriggerBranch: &br,
	}); err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	job := &buildJob{
		s:         s,
		gh:        github.NewApp(gh.AppID, gh.PrivateKey),
		installID: a.InstallationID,
		job: builder.Job{
			DeploymentID: depID,
			AppID:        a.ID,
			AppName:      a.Name,
			RepoFullName: a.RepoFullName,
			Branch:       branch,
			CommitSHA:    sha,
			CommitMsg:    "manual deploy",
			TriggerType:  "manual",
		},
	}
	if !s.queue.Submit(job) {
		_ = s.opts.Store.UpdateDeploymentStatus(r.Context(), dbgen.UpdateDeploymentStatusParams{
			ID: depID, Status: "failed", Error: strPtr("queue full"),
		})
		jsonErr(w, http.StatusServiceUnavailable, "queue full")
		return
	}

	slog.Info("manual deploy enqueued", "app", a.Name, "branch", branch, "sha", sha)
	d, _ := s.opts.Store.GetDeployment(r.Context(), depID)
	jsonOK(w, deploymentDetailJSON(d))
}

// ── GET /api/v1/deployments/:id ───────────────────────────────────────────

func (s *Server) handleGetDeploymentDetail(w http.ResponseWriter, r *http.Request) {
	id := pathSegment(r.URL.Path, 3)
	d, err := s.opts.Store.GetDeployment(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErr(w, http.StatusNotFound, "deployment not found")
		return
	}
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, deploymentDetailJSON(d))
}

func deploymentDetailJSON(d dbgen.Deployment) map[string]any {
	out := map[string]any{
		"id":             d.ID,
		"app_id":         d.AppID,
		"commit_sha":     d.CommitSha,
		"commit_message": d.CommitMessage,
		"status":         d.Status,
		"trigger_type":   d.TriggerType,
		"started_at":     d.StartedAt.Unix(),
	}
	if d.ImageTag != nil {
		out["image_tag"] = *d.ImageTag
	}
	if d.TriggerBranch != nil {
		out["trigger_branch"] = *d.TriggerBranch
	}
	if d.NomadJobHcl != nil {
		out["nomad_job_hcl"] = *d.NomadJobHcl
	}
	if d.BuildJobHcl != nil {
		out["build_job_hcl"] = *d.BuildJobHcl
	}
	if d.PreHookHcl != nil {
		out["pre_hook_hcl"] = *d.PreHookHcl
	}
	if d.PostHookHcl != nil {
		out["post_hook_hcl"] = *d.PostHookHcl
	}
	if d.EnvSnapshot != nil {
		out["env_snapshot"] = *d.EnvSnapshot
	}
	if d.Error != nil {
		out["error"] = *d.Error
	}
	if d.FinishedAt != nil {
		out["finished_at"] = d.FinishedAt.Unix()
	}
	return out
}

// ── POST /api/v1/deployments/:id/redeploy ────────────────────────────────

func (s *Server) handleRedeployJSON(w http.ResponseWriter, r *http.Request) {
	id := pathSegment(r.URL.Path, 3)
	src, err := s.opts.Store.GetDeployment(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErr(w, http.StatusNotFound, "source deployment not found")
		return
	}
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	app, err := s.opts.Store.GetApp(r.Context(), src.AppID)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	gh, err := s.opts.Store.GetGitHubApp(r.Context())
	if err != nil {
		jsonErr(w, http.StatusServiceUnavailable, "github app not connected")
		return
	}

	depID := uuid.NewString()
	branch := app.Branch
	if src.TriggerBranch != nil {
		branch = *src.TriggerBranch
	}
	if err := s.opts.Store.CreateDeployment(r.Context(), dbgen.CreateDeploymentParams{
		ID:            depID,
		AppID:         app.ID,
		CommitSha:     src.CommitSha,
		CommitMessage: "rollback to " + src.CommitSha[:7],
		Status:        "pending",
		TriggerType:   "rollback",
		TriggerBranch: &branch,
	}); err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	job := &buildJob{
		s:         s,
		gh:        github.NewApp(gh.AppID, gh.PrivateKey),
		installID: app.InstallationID,
		job: builder.Job{
			DeploymentID:       depID,
			AppID:              app.ID,
			AppName:            app.Name,
			RepoFullName:       app.RepoFullName,
			Branch:             branch,
			CommitSHA:          src.CommitSha,
			CommitMsg:          "rollback to " + src.CommitSha[:7],
			TriggerType:        "rollback",
			ReuseImageIfExists: true, // skip build if image is in registry
		},
	}
	if !s.queue.Submit(job) {
		_ = s.opts.Store.UpdateDeploymentStatus(r.Context(), dbgen.UpdateDeploymentStatusParams{
			ID: depID, Status: "failed", Error: strPtr("queue full"),
		})
		jsonErr(w, http.StatusServiceUnavailable, "queue full")
		return
	}

	d, _ := s.opts.Store.GetDeployment(r.Context(), depID)
	jsonOK(w, deploymentDetailJSON(d))
}

// ── Helpers ────────────────────────────────────────────────────────────────

// pathSegment returns the nth segment (0-based) of a URL path split by "/".
// e.g. "/api/v1/apps/123" → segment 3 = "123"
func pathSegment(path string, n int) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if n >= len(parts) {
		return ""
	}
	return parts[n]
}

func incrementKeyID(current string) string {
	// Expects "key-vN" format.
	var n int
	if _, err := fmt.Sscanf(current, "key-v%d", &n); err != nil {
		return "key-v2"
	}
	return fmt.Sprintf("key-v%d", n+1)
}

func buildAgeSetup(appName, keyID, publicKey, privateKey, envFile string) string {
	return fmt.Sprintf(`# 1. Save the private key locally (do NOT commit).
mkdir -p ~/.darkside/%[1]s
cat > ~/.darkside/%[1]s/%[2]s.txt <<'EOF'
%[3]s
EOF
chmod 600 ~/.darkside/%[1]s/%[2]s.txt

# 2. Encrypt your env file with the new public key.
#    Create env.plain with your KEY=VALUE pairs, then:
age -r '%[4]s' -o %[5]s env.plain
rm env.plain

# 3. Commit the encrypted env file.
git add %[5]s && git commit -m "darkside: rotate age key to %[2]s"`,
		appName, keyID, privateKey, publicKey, envFile)
}

func buildKeyTomlSnippet(keyID, publicKey, envFile string) string {
	return fmt.Sprintf("key_id = %q\nenv_file = %q\n# age public key: %s", keyID, envFile, publicKey)
}

func resolveBranchHead(ctx context.Context, repoFullName, branch, token string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/git/refs/heads/%s", repoFullName, branch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Object.SHA == "" {
		return "", fmt.Errorf("could not resolve branch %s HEAD", branch)
	}
	return out.Object.SHA, nil
}

