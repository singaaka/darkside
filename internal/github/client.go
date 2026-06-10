package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const apiBase = "https://api.github.com"

// App is a client for a single registered GitHub App. Construct via NewApp.
type App struct {
	ID         int64
	PrivateKey string
	http       *http.Client
}

func NewApp(appID int64, privateKeyPEM string) *App {
	return &App{
		ID:         appID,
		PrivateKey: privateKeyPEM,
		http:       &http.Client{Timeout: 15 * time.Second},
	}
}

// jwt returns a fresh ~10min JWT for App-level auth.
func (a *App) jwt() (string, error) { return signAppJWT(a.PrivateKey, a.ID, time.Now()) }

// callJSON does an authenticated request against api.github.com using the
// provided token. Token format: "Bearer <jwt>" or "token <installation_token>".
func (a *App) callJSON(ctx context.Context, method, path, token string, body, out any) error {
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, apiBase+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("github %s %s: %d %s", method, path, resp.StatusCode, string(respBody))
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode github response: %w (body: %s)", err, string(respBody))
		}
	}
	return nil
}

// ----------------- Manifest exchange (no auth needed) -----------------

// ManifestConversion is the response body when exchanging a temporary code for
// a newly-created App's credentials. Only the fields we care about.
type ManifestConversion struct {
	ID            int64  `json:"id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
	WebhookSecret string `json:"webhook_secret"`
	PEM           string `json:"pem"`
	HTMLURL       string `json:"html_url"`
}

// ExchangeManifestCode trades the one-shot ?code= from the manifest flow
// callback for the App's permanent credentials. Stateless — does not require
// an existing App.
func ExchangeManifestCode(ctx context.Context, code string) (*ManifestConversion, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/app-manifests/"+code+"/conversions", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("manifest exchange: %d %s", resp.StatusCode, string(body))
	}
	var out ManifestConversion
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("manifest exchange decode: %w (body: %s)", err, string(body))
	}
	return &out, nil
}

// ----------------- App-level calls (require App JWT) -----------------

type Installation struct {
	ID      int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"account"`
}

func (a *App) ListInstallations(ctx context.Context) ([]Installation, error) {
	jwt, err := a.jwt()
	if err != nil {
		return nil, err
	}
	var out []Installation
	if err := a.callJSON(ctx, http.MethodGet, "/app/installations", "Bearer "+jwt, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// InstallationToken mints a short-lived installation access token (~1h).
func (a *App) InstallationToken(ctx context.Context, installationID int64) (string, error) {
	jwt, err := a.jwt()
	if err != nil {
		return "", err
	}
	var resp struct {
		Token string `json:"token"`
	}
	path := fmt.Sprintf("/app/installations/%d/access_tokens", installationID)
	if err := a.callJSON(ctx, http.MethodPost, path, "Bearer "+jwt, nil, &resp); err != nil {
		return "", err
	}
	return resp.Token, nil
}

// ----------------- Installation-level calls (require installation token) -----------------

type Repo struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	Description   string `json:"description"`
}

// ListInstallationRepos pages through every repo the installation has access to.
func (a *App) ListInstallationRepos(ctx context.Context, installationID int64) ([]Repo, error) {
	token, err := a.InstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	var all []Repo
	page := 1
	for {
		var resp struct {
			Repositories []Repo `json:"repositories"`
		}
		path := fmt.Sprintf("/installation/repositories?per_page=100&page=%d", page)
		if err := a.callJSON(ctx, http.MethodGet, path, "token "+token, nil, &resp); err != nil {
			return nil, err
		}
		if len(resp.Repositories) == 0 {
			break
		}
		all = append(all, resp.Repositories...)
		if len(resp.Repositories) < 100 {
			break
		}
		page++
	}
	return all, nil
}
