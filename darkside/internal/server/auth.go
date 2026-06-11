package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/singaaka/darkside/internal/db/dbgen"
)

const sessionCookie = "ds_session"
const sessionDuration = 30 * 24 * time.Hour

// sessionUser is stored in request context.
type sessionUser struct {
	ID          int64
	GithubID    int64
	Login       string
	Email       string
	IsAdmin     bool
	Whitelisted bool
}

type contextKey string

const ctxUser contextKey = "user"

func userFromCtx(ctx context.Context) *sessionUser {
	u, _ := ctx.Value(ctxUser).(*sessionUser)
	return u
}

// authMiddleware reads the session cookie and attaches the user to the context.
// It does NOT enforce authentication — routes that require it call requireAuth.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err == nil && cookie.Value != "" {
			sw, err := s.opts.Store.GetSession(r.Context(), cookie.Value)
			if err == nil {
				u := &sessionUser{
					ID:          sw.Uid,
					GithubID:    sw.GithubID,
					Login:       sw.Login,
					IsAdmin:     sw.IsAdmin == 1,
					Whitelisted: sw.Whitelisted == 1,
				}
				if sw.Email != nil {
					u.Email = *sw.Email
				}
				r = r.WithContext(context.WithValue(r.Context(), ctxUser, u))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// requireAuth is middleware that enforces a whitelisted session exists.
// API routes return 401; page routes redirect to /login.
func requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := userFromCtx(r.Context())
		if u == nil || !u.Whitelisted {
			if isAPIPath(r.URL.Path) {
				http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isAPIPath(p string) bool {
	return len(p) >= 5 && p[:5] == "/api/"
}

// handleAuthGitHub initiates the GitHub OAuth flow using the GitHub App's
// client_id. The redirect_uri is the App's existing callback path.
func (s *Server) handleAuthGitHub(w http.ResponseWriter, r *http.Request) {
	gh, err := s.opts.Store.GetGitHubApp(r.Context())
	if errors.Is(err, sql.ErrNoRows) {
		// GitHub App not connected yet — send to onboarding.
		http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	state := s.manifestStates.issue()
	redirectURI := s.opts.Config.ExternalURL + "/auth/callback"
	authURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=read:user,user:email&state=%s",
		gh.ClientID, redirectURI, state,
	)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleAuthCallback handles the OAuth redirect from GitHub. Exchanges the
// code for a token, fetches the user profile, upserts the user, and issues
// a session cookie. The first user ever becomes admin + whitelisted.
func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" {
		http.Error(w, "missing ?code", http.StatusBadRequest)
		return
	}
	if !s.manifestStates.consume(state) {
		http.Error(w, "invalid or expired state", http.StatusBadRequest)
		return
	}

	gh, err := s.opts.Store.GetGitHubApp(r.Context())
	if err != nil {
		http.Error(w, "github app not configured", http.StatusServiceUnavailable)
		return
	}

	accessToken, err := exchangeOAuthCode(r.Context(), gh.ClientID, gh.ClientSecret, code, s.opts.Config.ExternalURL+"/auth/callback")
	if err != nil {
		slog.Error("oauth exchange", "error", err)
		http.Error(w, "oauth exchange failed", http.StatusBadGateway)
		return
	}

	profile, err := fetchGitHubProfile(r.Context(), accessToken)
	if err != nil {
		slog.Error("github profile", "error", err)
		http.Error(w, "fetch profile failed", http.StatusBadGateway)
		return
	}

	// Determine if this is the first user — they become admin + whitelisted.
	count, _ := s.opts.Store.CountUsers(r.Context())
	isAdmin := int64(0)
	isWhitelisted := int64(0)
	if count == 0 {
		isAdmin = 1
		isWhitelisted = 1
	}

	var emailPtr *string
	if profile.Email != "" {
		emailPtr = &profile.Email
	}

	user, err := s.opts.Store.UpsertUser(r.Context(), dbgen.UpsertUserParams{
		GithubID:    int64(profile.ID),
		Login:       profile.Login,
		Email:       emailPtr,
		IsAdmin:     isAdmin,
		Whitelisted: isWhitelisted,
	})
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	if user.Whitelisted == 0 {
		// Not whitelisted yet — show pending page.
		http.Redirect(w, r, "/login?pending=1", http.StatusSeeOther)
		return
	}

	token, err := generateSessionToken()
	if err != nil {
		http.Error(w, "token generation failed", http.StatusInternalServerError)
		return
	}
	if err := s.opts.Store.CreateSession(r.Context(), dbgen.CreateSessionParams{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(sessionDuration),
	}); err != nil {
		http.Error(w, "session creation failed", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(sessionDuration),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.URL.Scheme == "https",
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleAuthVerify is the Traefik forward-auth endpoint.
// Returns 200 if session is valid, 302 to /login otherwise.
func (s *Server) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	u := userFromCtx(r.Context())
	if u == nil || !u.Whitelisted {
		http.Redirect(w, r, s.opts.Config.ExternalURL+"/login", http.StatusFound)
		return
	}
	w.Header().Set("X-Forwarded-User", u.Login)
	w.WriteHeader(http.StatusOK)
}

// handleAuthLogout clears the session cookie and redirects to login.
func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		_ = s.opts.Store.DeleteSession(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:    sessionCookie,
		Value:   "",
		Path:    "/",
		Expires: time.Unix(0, 0),
		MaxAge:  -1,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// ── OAuth helpers ─────────────────────────────────────────────────────────────

type githubProfile struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
}

func exchangeOAuthCode(ctx context.Context, clientID, clientSecret, code, redirectURI string) (string, error) {
	url := fmt.Sprintf("https://github.com/login/oauth/access_token?client_id=%s&client_secret=%s&code=%s&redirect_uri=%s",
		clientID, clientSecret, code, redirectURI)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if out.Error != "" {
		return "", fmt.Errorf("github oauth: %s", out.Error)
	}
	return out.AccessToken, nil
}

func fetchGitHubProfile(ctx context.Context, token string) (*githubProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var p githubProfile
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

func generateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
