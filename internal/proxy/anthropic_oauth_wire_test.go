package proxy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEnsureClaudeOAuthWireNoSystem(t *testing.T) {
	in := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`)
	out := ensureClaudeOAuthWire(in)
	var root map[string]any
	if err := json.Unmarshal(out, &root); err != nil {
		t.Fatal(err)
	}
	sys, ok := root["system"].([]any)
	if !ok || len(sys) != 2 {
		t.Fatalf("system=%v", root["system"])
	}
	assertTextBlock(t, sys[0], claudeCodeBillingHeader)
	assertTextBlock(t, sys[1], claudeCodeIdentity)
}

func TestEnsureClaudeOAuthWirePreservesCallerSystem(t *testing.T) {
	in := []byte(`{"model":"m","system":"Be terse.","messages":[{"role":"user","content":"hi"}]}`)
	out := ensureClaudeOAuthWire(in)
	var root map[string]any
	_ = json.Unmarshal(out, &root)
	sys := root["system"].([]any)
	if len(sys) != 3 {
		t.Fatalf("len=%d %v", len(sys), sys)
	}
	assertTextBlock(t, sys[0], claudeCodeBillingHeader)
	assertTextBlock(t, sys[1], claudeCodeIdentity)
	assertTextBlock(t, sys[2], "Be terse.")
}

func TestEnsureClaudeOAuthWireSplitsConcatenatedIdentity(t *testing.T) {
	in := []byte(`{"model":"m","system":"You are Claude Code, Anthropic's official CLI for Claude.\n\nBe terse.","messages":[{"role":"user","content":"hi"}]}`)
	out := ensureClaudeOAuthWire(in)
	var root map[string]any
	_ = json.Unmarshal(out, &root)
	sys := root["system"].([]any)
	if len(sys) != 3 {
		t.Fatalf("len=%d %v", len(sys), sys)
	}
	assertTextBlock(t, sys[1], claudeCodeIdentity)
	assertTextBlock(t, sys[2], "Be terse.")
}

func TestEnsureClaudeOAuthWireIdempotent(t *testing.T) {
	in := []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`)
	once := ensureClaudeOAuthWire(in)
	twice := ensureClaudeOAuthWire(once)
	var a, b map[string]any
	_ = json.Unmarshal(once, &a)
	_ = json.Unmarshal(twice, &b)
	as := a["system"].([]any)
	bs := b["system"].([]any)
	if len(as) != 2 || len(bs) != 2 {
		t.Fatalf("once=%d twice=%d", len(as), len(bs))
	}
	assertTextBlock(t, bs[0], claudeCodeBillingHeader)
	assertTextBlock(t, bs[1], claudeCodeIdentity)
}

func TestEnsureClaudeOAuthWireDropsDuplicateCallerBlocks(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"model": "m",
		"system": []any{
			map[string]any{"type": "text", "text": claudeCodeBillingHeader},
			map[string]any{"type": "text", "text": claudeCodeIdentity},
			map[string]any{"type": "text", "text": "custom"},
		},
		"messages": []any{map[string]any{"role": "user", "content": "hi"}},
	})
	out := ensureClaudeOAuthWire(body)
	var root map[string]any
	_ = json.Unmarshal(out, &root)
	sys := root["system"].([]any)
	if len(sys) != 3 {
		t.Fatalf("len=%d", len(sys))
	}
	assertTextBlock(t, sys[2], "custom")
	joined, _ := json.Marshal(sys)
	if strings.Count(string(joined), claudeCodeIdentity) != 1 {
		t.Fatalf("duplicate identity: %s", joined)
	}
}

func assertTextBlock(t *testing.T, block any, want string) {
	t.Helper()
	m, ok := block.(map[string]any)
	if !ok {
		t.Fatalf("block type %T", block)
	}
	if m["type"] != "text" || m["text"] != want {
		t.Fatalf("got %+v want text=%q", m, want)
	}
}
