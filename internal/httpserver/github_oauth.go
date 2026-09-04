package httpserver

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/goodtekxyz/openllms/internal/config"
	"github.com/goodtekxyz/openllms/internal/githubauth"
)

const (
	oauthStateCookie = "llms_oauth_state"
	oauthNextCookie  = "llms_oauth_next"
)

func (s *Server) githubCallbackURL() string {
	return strings.TrimRight(s.cfg.PublicBaseURL, "/") + "/auth/github/callback"
}

func (s *Server) githubOAuthConfigured() bool {
	return s.cfg.GitHubClientID != "" && s.cfg.GitHubClientSecret != ""
}

// handleGitHubOAuthStart redirects the browser to GitHub authorize.
// Query: next=/console|/billing|/admin (relative path only).
func (s *Server) handleGitHubOAuthStart(w http.ResponseWriter, r *http.Request) {
	if !s.githubOAuthConfigured() {
		http.Error(w, "github_oauth_not_configured: set GITHUB_CLIENT_ID/SECRET in env or Infisical "+config.OpsGitHubSecretPath+" (client_id, client_secret); register callback "+s.githubCallbackURL(), http.StatusServiceUnavailable)
		return
	}
	next := sanitizeOAuthNext(r.URL.Query().Get("next"))
	state, err := randomHex(16)
	if err != nil {
		http.Error(w, "state_failed", http.StatusInternalServerError)
		return
	}
	secure := s.cfg.AdminCookieSecure
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   600,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     oauthNextCookie,
		Value:    next,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   600,
	})
	http.Redirect(w, r, githubauth.AuthorizeURL(s.cfg.GitHubClientID, s.githubCallbackURL(), state, ""), http.StatusFound)
}

// handleGitHubOAuthCallback exchanges the code, issues session cookies, redirects to next.
func (s *Server) handleGitHubOAuthCallback(w http.ResponseWriter, r *http.Request) {
	clearOAuthCookies(w, s.cfg.AdminCookieSecure)

	if errParam := r.URL.Query().Get("error"); errParam != "" {
		desc := r.URL.Query().Get("error_description")
		redirectAuthError(w, r, "/console", errParam, desc)
		return
	}
	if !s.githubOAuthConfigured() {
		redirectAuthError(w, r, "/console", "oauth_not_configured", "")
		return
	}

	stateCookie, err := r.Cookie(oauthStateCookie)
	if err != nil || stateCookie.Value == "" {
		redirectAuthError(w, r, "/console", "missing_state", "")
		return
	}
	qState := r.URL.Query().Get("state")
	if qState == "" || qState != stateCookie.Value {
		redirectAuthError(w, r, "/console", "state_mismatch", "")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		redirectAuthError(w, r, "/console", "missing_code", "")
		return
	}

	next := "/console"
	if c, err := r.Cookie(oauthNextCookie); err == nil {
		next = sanitizeOAuthNext(c.Value)
	}

	tok, err := githubauth.ExchangeCode(r.Context(), s.cfg.GitHubClientID, s.cfg.GitHubClientSecret, code, s.githubCallbackURL())
	if err != nil {
		redirectAuthError(w, r, next, "token_exchange_failed", err.Error())
		return
	}
	u, err := githubauth.FetchUser(r.Context(), tok)
	if err != nil {
		redirectAuthError(w, r, next, "github_user_failed", err.Error())
		return
	}
	ghID := fmt.Sprintf("%d", u.ID)

	if strings.HasPrefix(next, "/admin") {
		if !s.admin.Enabled() {
			redirectAuthError(w, r, "/admin", "admin_disabled", "")
			return
		}
		if err := s.admin.Issue(w, u.Login, ghID); err != nil {
			redirectAuthError(w, r, "/admin", "admin_forbidden", err.Error())
			return
		}
		http.Redirect(w, r, next, http.StatusFound)
		return
	}

	if !s.user.Enabled() {
		redirectAuthError(w, r, next, "session_disabled", "")
		return
	}
	if _, _, _, _, err := s.store.UpsertGitHubUser(r.Context(), ghID, u.Login); err != nil {
		redirectAuthError(w, r, next, "upsert_failed", err.Error())
		return
	}
	if err := s.user.Issue(w, u.Login, ghID); err != nil {
		redirectAuthError(w, r, next, "session_failed", err.Error())
		return
	}
	http.Redirect(w, r, next, http.StatusFound)
}

func sanitizeOAuthNext(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "/console"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "" || u.Host != "" || !strings.HasPrefix(u.Path, "/") {
		return "/console"
	}
	path := u.Path
	switch {
	case path == "/console" || strings.HasPrefix(path, "/console/"):
		return path
	case path == "/billing" || strings.HasPrefix(path, "/billing/"):
		return path
	case path == "/admin" || strings.HasPrefix(path, "/admin/"):
		return path
	default:
		return "/console"
	}
}

func redirectAuthError(w http.ResponseWriter, r *http.Request, next, code, detail string) {
	next = sanitizeOAuthNext(next)
	q := url.Values{}
	q.Set("auth_error", code)
	if detail != "" {
		if len(detail) > 180 {
			detail = detail[:180]
		}
		q.Set("auth_detail", detail)
	}
	http.Redirect(w, r, next+"?"+q.Encode(), http.StatusFound)
}

func clearOAuthCookies(w http.ResponseWriter, secure bool) {
	for _, name := range []string{oauthStateCookie, oauthNextCookie} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1,
			Secure:   secure,
		})
	}
}

func randomHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
