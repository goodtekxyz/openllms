package quota

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseCodexUsage(t *testing.T) {
	raw := []byte(`{
	  "rate_limit": {
	    "primary_window": {"used_percent": 10, "reset_at": 1700000000},
	    "secondary_window": {"used_percent": 40, "reset_at": 1700500000}
	  }
	}`)
	s, err := ParseCodexUsage(raw)
	if err != nil {
		t.Fatal(err)
	}
	if s.RemainingPct != 60 {
		t.Fatalf("remaining %v want 60 (tightest=40 used)", s.RemainingPct)
	}
	if s.ResetAt == nil || s.ResetAt.Unix() != 1700500000 {
		t.Fatalf("reset %+v", s.ResetAt)
	}
	if s.Source != "codex:7d" {
		t.Fatalf("source %s", s.Source)
	}
}

func TestParseClaudeUsage(t *testing.T) {
	raw := []byte(`{
	  "five_hour": {"utilization": 70, "resets_at": "2026-04-08T18:59:59Z"},
	  "seven_day": {"utilization": 20, "resets_at": "2026-04-14T16:59:59Z"}
	}`)
	s, err := ParseClaudeUsage(raw)
	if err != nil {
		t.Fatal(err)
	}
	if s.RemainingPct != 30 {
		t.Fatalf("remaining %v", s.RemainingPct)
	}
	if s.Source != "claude:5h" {
		t.Fatalf("source %s", s.Source)
	}
	want, _ := time.Parse(time.RFC3339, "2026-04-08T18:59:59Z")
	if s.ResetAt == nil || !s.ResetAt.Equal(want) {
		t.Fatalf("reset %+v", s.ResetAt)
	}
}

func TestFetchCodexWire(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("auth %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("ChatGPT-Account-Id") != "acct" {
			t.Errorf("acct %q", r.Header.Get("ChatGPT-Account-Id"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"rate_limit": map[string]any{
				"primary_window": map[string]any{"used_percent": 5, "reset_at": 1},
			},
		})
	}))
	defer srv.Close()
	prev := CodexUsageURL
	CodexUsageURL = srv.URL
	HTTPClient = srv.Client()
	t.Cleanup(func() { CodexUsageURL = prev; HTTPClient = nil })

	s, err := FetchCodex(context.Background(), "tok", "acct")
	if err != nil {
		t.Fatal(err)
	}
	if s.RemainingPct != 95 {
		t.Fatalf("%v", s.RemainingPct)
	}
}

func TestFetchClaudeWire(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("anthropic-beta") == "" {
			t.Error("missing beta")
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing ua")
		}
		_, _ = io.WriteString(w, `{"five_hour":{"utilization":25,"resets_at":"2026-01-02T03:04:05Z"}}`)
	}))
	defer srv.Close()
	prev := ClaudeUsageURL
	ClaudeUsageURL = srv.URL
	HTTPClient = srv.Client()
	t.Cleanup(func() { ClaudeUsageURL = prev; HTTPClient = nil })

	s, err := FetchClaude(context.Background(), "atok")
	if err != nil {
		t.Fatal(err)
	}
	if s.RemainingPct != 75 {
		t.Fatalf("%v", s.RemainingPct)
	}
}
