package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/singaaka/darkside/internal/db/dbgen"
	"github.com/singaaka/darkside/internal/github"
)

// manifestStates tracks in-flight state tokens for the manifest CSRF check.
// Tokens expire after 10 minutes; if the user takes longer than that to
// confirm on GitHub, they'll have to retry. Memory only — restart wipes them.
type manifestStates struct {
	mu sync.Mutex
	m  map[string]time.Time
}

func newManifestStates() *manifestStates { return &manifestStates{m: map[string]time.Time{}} }

func (s *manifestStates) issue() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	t := hex.EncodeToString(b)
	s.mu.Lock()
	s.m[t] = time.Now()
	s.mu.Unlock()
	return t
}

func (s *manifestStates) consume(t string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts, ok := s.m[t]
	if !ok {
		return false
	}
	delete(s.m, t)
	return time.Since(ts) < 10*time.Minute
}

// manifest returns the JSON manifest GitHub uses to provision the App.
// `state` is NOT included here — it's a permitted form field on the parent
// POST, but the manifest object itself rejects unknown keys.
func (s *Server) manifest() map[string]any {
	external := strings.TrimRight(s.opts.Config.ExternalURL, "/")
	return map[string]any{
		"name":         fmt.Sprintf("darkside-%s", s.opts.Config.Host()),
		"url":          external,
		"redirect_url": external + "/github/manifest/callback",
		"hook_attributes": map[string]any{
			"url":    external + "/webhooks/github",
			"active": true,
		},
		"default_events": []string{"push"},
		"default_permissions": map[string]string{
			"contents":      "read",
			"metadata":      "read",
			"pull_requests": "read",
			"emails":        "read",
		},
		"public": false,
	}
}

var manifestStartTmpl = template.Must(template.New("manifest-start").Parse(`<!doctype html>
<html><head><title>Connect GitHub — darkside</title></head>
<body>
<p>Redirecting to GitHub to create the darkside GitHub App…</p>
<form id="f" action="{{.PostURL}}" method="post">
  <input type="hidden" name="manifest" value="{{.ManifestJSON}}">
  <input type="hidden" name="state" value="{{.State}}">
</form>
<script>document.getElementById('f').submit();</script>
<noscript><button form="f" type="submit">Continue</button></noscript>
</body></html>`))

// handleManifestStart renders a self-submitting form that POSTs the App
// manifest to GitHub. The user lands on a GitHub confirmation page, then is
// redirected back to /github/manifest/callback.
//
// Query: ?org=<github-org>   (optional — install for an org instead of user)
func (s *Server) handleManifestStart(w http.ResponseWriter, r *http.Request) {
	state := s.manifestStates.issue()
	manifestJSON, _ := json.Marshal(s.manifest())

	postURL := "https://github.com/settings/apps/new"
	if org := r.URL.Query().Get("org"); org != "" {
		postURL = fmt.Sprintf("https://github.com/organizations/%s/settings/apps/new", url.PathEscape(org))
	}

	// Pass raw JSON to the template; html/template auto-escapes inside the
	// attribute value (turning " into &#34; etc). Pre-escaping would
	// double-encode and break GitHub's JSON parser.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = manifestStartTmpl.Execute(w, map[string]string{
		"PostURL":      postURL,
		"ManifestJSON": string(manifestJSON),
		"State":        state,
	})
}

// handleManifestCallback is what GitHub redirects to after the user confirms
// app creation. It exchanges the one-shot code for the App's credentials,
// stores them, and redirects to the dashboard.
func (s *Server) handleManifestCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" {
		http.Error(w, "missing ?code", http.StatusBadRequest)
		return
	}
	if !s.manifestStates.consume(state) {
		http.Error(w, "invalid or expired state — please retry from /settings", http.StatusBadRequest)
		return
	}

	conv, err := github.ExchangeManifestCode(r.Context(), code)
	if err != nil {
		slog.Error("manifest exchange", "error", err)
		http.Error(w, "github exchange failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	if err := s.opts.Store.UpsertGitHubApp(r.Context(), dbgen.UpsertGitHubAppParams{
		AppID:         conv.ID,
		Slug:          conv.Slug,
		Name:          conv.Name,
		ClientID:      conv.ClientID,
		ClientSecret:  conv.ClientSecret,
		WebhookSecret: conv.WebhookSecret,
		PrivateKey:    conv.PEM,
		HtmlUrl:       conv.HTMLURL,
	}); err != nil {
		slog.Error("store github app", "error", err)
		http.Error(w, "store failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("github app connected", "slug", conv.Slug, "app_id", conv.ID)

	// Send the user back to settings — the UI re-fetches GetStatus and shows
	// "connected" with an Install button.
	http.Redirect(w, r, "/settings?connected=1", http.StatusSeeOther)
}
