package proxy

import "testing"

func TestLLMSWebSearchMode(t *testing.T) {
	if llmsWebSearchMode(nil) != "" {
		t.Fatal("empty default")
	}
	disabled := []byte(`{"web_search":{"mode":"disabled"}}`)
	if !llmsWebSearchDisabled(disabled) {
		t.Fatal("disabled")
	}
	if codexWebSearchLive(disabled) != nil {
		t.Fatal("codex disabled")
	}
	cached := []byte(`{"web_search":{"mode":"cached"}}`)
	if llmsWebSearchDisabled(cached) {
		t.Fatal("cached not disabled")
	}
	live := codexWebSearchLive(cached)
	if live == nil || *live {
		t.Fatalf("cached external=%v", live)
	}
}
