package main

import (
	"strings"
	"testing"
	"time"
)

func TestQuotaBar(t *testing.T) {
	b := quotaBar(62)
	if len([]rune(b)) != 10 {
		t.Fatalf("width %q", b)
	}
	if !strings.HasPrefix(b, "██████") { // 6 of 10 for 62%
		t.Fatalf("bar %q", b)
	}
	if quotaBar(0) != "░░░░░░░░░░" {
		t.Fatal(quotaBar(0))
	}
	if quotaBar(100) != "██████████" {
		t.Fatal(quotaBar(100))
	}
}

func TestFormatReset(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	if got := formatReset("2026-08-23T00:02:23Z", now); got != "reset ~2d" {
		t.Fatalf("got %q", got)
	}
	if got := formatReset("2026-08-20T18:00:00Z", now); got != "reset ~6h" {
		t.Fatalf("got %q", got)
	}
	if got := formatReset("2026-08-20T12:30:00Z", now); got != "reset ~30m" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatAccountLine(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	line := formatAccountLine(map[string]any{
		"glyph": "●", "ref": "codex:codex-goodtek", "health": "ok",
		"quota_remaining_pct": 92.0, "quota_reset_at": "2026-08-23T00:02:23Z",
	}, 20, now)
	if !strings.Contains(line, "quota  92%") || !strings.Contains(line, "█████████░") {
		t.Fatalf("%q", line)
	}
	if !strings.Contains(line, "reset ~2d") {
		t.Fatalf("%q", line)
	}
}
