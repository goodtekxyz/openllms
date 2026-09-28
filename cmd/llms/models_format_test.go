package main

import (
	"strings"
	"testing"
)

func TestFilterModelRows(t *testing.T) {
	rows := []modelRow{
		{ID: "gpt-5.5", Providers: []string{"codex"}, AccountIDs: []string{"codex:work"}},
		{ID: "claude-sonnet-4-5", Providers: []string{"claude"}, AccountIDs: []string{"claude:personal"}},
		{ID: "deepseek-chat", Providers: []string{"deepseek"}, AccountIDs: []string{"deepseek:default"}},
	}
	got := filterModelRows(rows, "claude")
	if len(got) != 1 || got[0].ID != "claude-sonnet-4-5" {
		t.Fatalf("filter by provider: %#v", got)
	}
	got = filterModelRows(rows, "codex:work")
	if len(got) != 1 || got[0].ID != "gpt-5.5" {
		t.Fatalf("filter by account: %#v", got)
	}
	got = filterModelRows(rows, "GPT-5")
	if len(got) != 1 || got[0].ID != "gpt-5.5" {
		t.Fatalf("filter by id case-insensitive: %#v", got)
	}
	if n := len(filterModelRows(rows, "")); n != 3 {
		t.Fatalf("empty filter should keep all, got %d", n)
	}
}

func TestFormatModelsBoard(t *testing.T) {
	rows := []modelRow{
		{ID: "gpt-5.5", Providers: []string{"codex"}, AccountIDs: []string{"codex:a"}},
	}
	accounts := []any{
		map[string]any{"id": "codex:a", "status": "ok", "models": []any{map[string]any{"id": "gpt-5.5"}}},
		map[string]any{"id": "codex:b", "status": "error", "error": "timeout", "models": []any{}},
	}
	out := formatModelsBoard("demo", "sequential", "gpt-5.5", rows, accounts)
	for _, want := range []string{
		"ROUTE  demo  sequential",
		"SUGGESTED  gpt-5.5",
		"gpt-5.5",
		"providers=codex",
		"accounts=codex:a",
		"codex:a",
		"codex:b",
		"timeout",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	// Union model line must not present account ref as the model id column alone.
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "  ") || strings.Contains(line, "status") || strings.Contains(line, "models=") {
			continue
		}
		if strings.Contains(line, "providers=") {
			fields := strings.Fields(line)
			if len(fields) > 0 && looksLikeAccountRefID(fields[0]) {
				t.Fatalf("union model id looks like account ref: %q", line)
			}
		}
	}
}

func TestLooksLikeAccountRefID(t *testing.T) {
	if !looksLikeAccountRefID("codex:work") {
		t.Fatal("expected account ref")
	}
	if looksLikeAccountRefID("gpt-5.5") {
		t.Fatal("model id is not account ref")
	}
	if looksLikeAccountRefID("claude-sonnet-4-5") {
		t.Fatal("hyphenated model id is not account ref")
	}
}

func TestParseUnionModels(t *testing.T) {
	raw := []any{
		map[string]any{
			"id":          "gpt-5.5",
			"providers":   []any{"codex"},
			"account_ids": []any{"codex:a"},
		},
	}
	rows := parseUnionModels(raw)
	if len(rows) != 1 || rows[0].ID != "gpt-5.5" || rows[0].AccountIDs[0] != "codex:a" {
		t.Fatalf("%#v", rows)
	}
}
