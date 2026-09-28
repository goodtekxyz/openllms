package vendorauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClaudeRefresh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/x-www-form-urlencoded") {
			t.Errorf("content-type %q", ct)
		}
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("client_id") != ClaudeClientID || r.Form.Get("refresh_token") != "old-rt" {
			t.Errorf("form %v", r.Form)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "new-at", "refresh_token": "new-rt", "expires_in": 3600,
		})
	}))
	defer srv.Close()
	prev := ClaudeTokenURL
	ClaudeTokenURL = srv.URL
	t.Cleanup(func() { ClaudeTokenURL = prev })
	HTTPClient = srv.Client()
	t.Cleanup(func() { HTTPClient = nil })

	tok, err := ClaudeRefresh(context.Background(), "old-rt")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "new-at" || tok.RefreshToken != "new-rt" {
		t.Fatalf("%+v", tok)
	}
}

func TestClaudeRefreshKeepsPriorRefreshTokenWhenNotRotated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "new-at", "expires_in": 3600})
	}))
	defer srv.Close()
	prev := ClaudeTokenURL
	ClaudeTokenURL = srv.URL
	t.Cleanup(func() { ClaudeTokenURL = prev })
	HTTPClient = srv.Client()
	t.Cleanup(func() { HTTPClient = nil })

	tok, err := ClaudeRefresh(context.Background(), "old-rt")
	if err != nil {
		t.Fatal(err)
	}
	if tok.RefreshToken != "old-rt" {
		t.Fatalf("expected refresh_token preserved, got %+v", tok)
	}
}

func TestClaudeRefreshError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()
	prev := ClaudeTokenURL
	ClaudeTokenURL = srv.URL
	t.Cleanup(func() { ClaudeTokenURL = prev })
	HTTPClient = srv.Client()
	t.Cleanup(func() { HTTPClient = nil })

	if _, err := ClaudeRefresh(context.Background(), "old-rt"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCodexRefresh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("client_id") != CodexClientID || r.Form.Get("refresh_token") != "old-codex-rt" {
			t.Errorf("form %v", r.Form)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "codex-new-at", "refresh_token": "codex-new-rt",
		})
	}))
	defer srv.Close()
	prev := CodexTokenURL
	CodexTokenURL = srv.URL
	t.Cleanup(func() { CodexTokenURL = prev })
	HTTPClient = srv.Client()
	t.Cleanup(func() { HTTPClient = nil })

	tok, err := CodexRefresh(context.Background(), "old-codex-rt")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "codex-new-at" || tok.RefreshToken != "codex-new-rt" {
		t.Fatalf("%+v", tok)
	}
}

func TestCodexRefreshPreservesChatGPTAccountID(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"https://api.openai.com/auth": map[string]any{"chatgpt_account_id": "acct-refreshed"},
	})
	idToken := "h." + base64.RawURLEncoding.EncodeToString(payload) + ".s"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "codex-new-at", "id_token": idToken,
		})
	}))
	defer srv.Close()
	prev := CodexTokenURL
	CodexTokenURL = srv.URL
	t.Cleanup(func() { CodexTokenURL = prev })
	HTTPClient = srv.Client()
	t.Cleanup(func() { HTTPClient = nil })

	tok, err := CodexRefresh(context.Background(), "old-codex-rt")
	if err != nil {
		t.Fatal(err)
	}
	if tok.ChatGPTAccountID != "acct-refreshed" {
		t.Fatalf("%+v", tok)
	}
}

func TestRefreshDispatch(t *testing.T) {
	claudeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "claude-at"})
	}))
	defer claudeSrv.Close()
	codexSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "codex-at"})
	}))
	defer codexSrv.Close()

	prevClaude, prevCodex := ClaudeTokenURL, CodexTokenURL
	ClaudeTokenURL, CodexTokenURL = claudeSrv.URL, codexSrv.URL
	t.Cleanup(func() { ClaudeTokenURL, CodexTokenURL = prevClaude, prevCodex })
	HTTPClient = http.DefaultClient
	t.Cleanup(func() { HTTPClient = nil })

	tok, err := Refresh(context.Background(), "anthropic", "rt")
	if err != nil || tok.AccessToken != "claude-at" {
		t.Fatalf("anthropic: %+v %v", tok, err)
	}
	tok, err = Refresh(context.Background(), "OpenAI", "rt")
	if err != nil || tok.AccessToken != "codex-at" {
		t.Fatalf("openai: %+v %v", tok, err)
	}
	if _, err := Refresh(context.Background(), "unknown", "rt"); err == nil {
		t.Fatal("expected error for unknown vendor")
	}
}
