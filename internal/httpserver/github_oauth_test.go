package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goodtekxyz/openllms/internal/config"
)

func TestSanitizeOAuthNext(t *testing.T) {
	cases := map[string]string{
		"":                    "/console",
		"/console":            "/console",
		"/billing":            "/billing",
		"/admin":              "/admin",
		"https://evil/x":      "/console",
		"//evil":              "/console",
		"/install":            "/console",
		"/console/extra":      "/console/extra",
	}
	for in, want := range cases {
		if got := sanitizeOAuthNext(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestGitHubOAuthStartRequiresSecret(t *testing.T) {
	s := &Server{cfg: config.Config{GitHubClientID: "cid", PublicBaseURL: "https://llms.example"}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/github?next=/console", nil)
	s.handleGitHubOAuthStart(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code %d body %s", rec.Code, rec.Body.String())
	}
}

func TestGitHubOAuthStartRedirects(t *testing.T) {
	s := &Server{cfg: config.Config{
		GitHubClientID:     "cid",
		GitHubClientSecret: "sec",
		PublicBaseURL:      "https://llms.example",
		AdminCookieSecure:  false,
	}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/github?next=/billing", nil)
	s.handleGitHubOAuthStart(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("code %d body %s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "github.com/login/oauth/authorize") || !strings.Contains(loc, "client_id=cid") {
		t.Fatalf("location: %s", loc)
	}
	if !strings.Contains(loc, urlQueryEscape("https://llms.example/auth/github/callback")) &&
		!strings.Contains(loc, "redirect_uri=") {
		t.Fatalf("missing redirect_uri in %s", loc)
	}
	cookies := rec.Result().Cookies()
	var state, next string
	for _, c := range cookies {
		if c.Name == oauthStateCookie {
			state = c.Value
		}
		if c.Name == oauthNextCookie {
			next = c.Value
		}
	}
	if state == "" || next != "/billing" {
		t.Fatalf("cookies state=%q next=%q", state, next)
	}
}

func urlQueryEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, ":", "%3A"), "/", "%2F")
}
