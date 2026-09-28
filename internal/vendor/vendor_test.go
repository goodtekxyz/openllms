package vendor

import "testing"

func TestDefaultBaseURLFor(t *testing.T) {
	if DefaultBaseURL("codex") != "https://api.openai.com/v1" {
		t.Fatal(DefaultBaseURL("codex"))
	}
	if DefaultBaseURLFor("codex", "oauth") != "https://chatgpt.com/backend-api/codex" {
		t.Fatal(DefaultBaseURLFor("codex", "oauth"))
	}
	if DefaultBaseURLFor("claude", "oauth") != "https://api.anthropic.com" {
		t.Fatal(DefaultBaseURLFor("claude", "oauth"))
	}
}
