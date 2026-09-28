package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		cur, latest string
		want        bool
	}{
		{"v0.1.1-112-gd9f4ee6", "v0.1.1-118-g0703955", true},
		{"v0.1.1-118-g0703955", "v0.1.1-118-g0703955", false},
		{"v0.1.1-118-g0703955", "v0.1.1-112-gd9f4ee6", false},
		{"v0.1.1-200-gabc1234", "v0.1.2", true},
		{"v0.1.1", "v0.1.1-1-gabc1234", true},
		{"v0.1.1-3-gabc1234-dirty", "v0.1.1-4-gdef5678", true},
		{"dev", "v9.9.9", false},
		{"v0.1.1", "garbage", false},
	}
	for _, c := range cases {
		if got := isNewerVersion(c.cur, c.latest); got != c.want {
			t.Errorf("isNewerVersion(%q, %q) = %v, want %v", c.cur, c.latest, got, c.want)
		}
	}
}

func TestUpdateNotice(t *testing.T) {
	msg := updateNotice("v0.1.1-112-gd9f4ee6", "v0.1.1-118-g0703955", "https://llms.goodtek.xyz/")
	if !strings.Contains(msg, "v0.1.1-118-g0703955") || !strings.Contains(msg, "curl -fsSL https://llms.goodtek.xyz/install.sh | bash") {
		t.Fatalf("unexpected notice: %q", msg)
	}
	if updateNotice("v0.1.1-118-g0703955", "v0.1.1-118-g0703955", "https://x") != "" {
		t.Fatal("expected no notice when up to date")
	}
}

func TestLatestVersionCachesWithinTTL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hits := 0
	version := "v0.1.1-118-g0703955"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dist/VERSION" {
			http.NotFound(w, r)
			return
		}
		hits++
		_, _ = w.Write([]byte(version + "\n"))
	}))
	defer srv.Close()

	ctx := context.Background()
	now := time.Now()
	if got := latestVersion(ctx, srv.URL, now, false); got != "v0.1.1-118-g0703955" {
		t.Fatalf("first fetch = %q", got)
	}
	version = "v0.1.1-120-gaaaaaaa"
	if got := latestVersion(ctx, srv.URL, now.Add(time.Hour), false); got != "v0.1.1-118-g0703955" || hits != 1 {
		t.Fatalf("expected cached value within TTL, got %q hits=%d", got, hits)
	}
	if got := latestVersion(ctx, srv.URL, now.Add(25*time.Hour), false); got != "v0.1.1-120-gaaaaaaa" || hits != 2 {
		t.Fatalf("expected refetch after TTL, got %q hits=%d", got, hits)
	}
	if got := latestVersion(ctx, srv.URL, now.Add(25*time.Hour), true); got != "v0.1.1-120-gaaaaaaa" || hits != 3 {
		t.Fatalf("expected forced refetch, got %q hits=%d", got, hits)
	}
}

func TestLatestVersionFallsBackToCacheOnError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saveUpdateCache(updateCache{CheckedAt: time.Now().Add(-48 * time.Hour), Latest: "v0.1.1-118-g0703955"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer srv.Close()
	if got := latestVersion(context.Background(), srv.URL, time.Now(), false); got != "v0.1.1-118-g0703955" {
		t.Fatalf("expected cached fallback, got %q", got)
	}
	c, _ := loadUpdateCache()
	if time.Since(c.CheckedAt) > time.Minute {
		t.Fatal("failed attempt should refresh checked_at to avoid retrying every command")
	}
}
