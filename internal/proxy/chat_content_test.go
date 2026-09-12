package proxy

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestChatContentToCodexPartsMultimodal(t *testing.T) {
	raw := json.RawMessage(`[
		{"type":"text","text":"what is this?"},
		{"type":"image_url","image_url":{"url":"https://example.com/a.png","detail":"high"}}
	]`)
	parts := chatContentToCodexParts(raw, false)
	if len(parts) != 2 {
		t.Fatalf("parts=%d %#v", len(parts), parts)
	}
	if parts[0]["type"] != "input_text" || parts[0]["text"] != "what is this?" {
		t.Fatalf("text part: %#v", parts[0])
	}
	if parts[1]["type"] != "input_image" || parts[1]["image_url"] != "https://example.com/a.png" {
		t.Fatalf("image part: %#v", parts[1])
	}
	if parts[1]["detail"] != "high" {
		t.Fatalf("detail: %#v", parts[1]["detail"])
	}
}

func TestChatContentToCodexPartsImageOnlyUser(t *testing.T) {
	raw := json.RawMessage(`[{"type":"image_url","image_url":"https://example.com/b.jpg"}]`)
	parts := chatContentToCodexParts(raw, false)
	if len(parts) != 1 || parts[0]["type"] != "input_image" {
		t.Fatalf("%#v", parts)
	}
}

func TestChatToCodexResponsesKeepsImageURL(t *testing.T) {
	in := []byte(`{
	  "model":"gpt-5",
	  "messages":[{"role":"user","content":[
	    {"type":"text","text":"describe"},
	    {"type":"image_url","image_url":{"url":"https://cdn.example/x.png"}}
	  ]}]
	}`)
	out, _, _, err := chatToCodexResponses(in)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `"input_image"`) || !strings.Contains(s, "https://cdn.example/x.png") {
		t.Fatalf("missing image: %s", s)
	}
	if !strings.Contains(s, `"input_text"`) || !strings.Contains(s, "describe") {
		t.Fatalf("missing text: %s", s)
	}
}

func TestChatToAnthropicMessagesKeepsImageURL(t *testing.T) {
	in := []byte(`{
	  "model":"claude-sonnet-4-5",
	  "messages":[{"role":"user","content":[
	    {"type":"text","text":"see"},
	    {"type":"image_url","image_url":{"url":"https://cdn.example/y.png"}}
	  ]}]
	}`)
	out, _, _, err := chatToAnthropicMessages(in)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `"type":"image"`) || !strings.Contains(s, "https://cdn.example/y.png") {
		t.Fatalf("missing image: %s", s)
	}
}

func TestChatToAnthropicMessagesDataURI(t *testing.T) {
	in := []byte(`{
	  "model":"claude-sonnet-4-5",
	  "messages":[{"role":"user","content":[
	    {"type":"image_url","image_url":{"url":"data:image/png;base64,aaaBBB"}}
	  ]}]
	}`)
	out, _, _, err := chatToAnthropicMessages(in)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `"type":"base64"`) || !strings.Contains(s, `"media_type":"image/png"`) || !strings.Contains(s, "aaaBBB") {
		t.Fatalf("%s", s)
	}
}

func TestMessageItemTextConcatenatesParts(t *testing.T) {
	m := map[string]any{
		"type": "message",
		"content": []any{
			map[string]any{"type": "output_text", "text": "A"},
			map[string]any{"type": "output_text", "text": "B"},
		},
	}
	if got := messageItemText(m); got != "AB" {
		t.Fatalf("%q", got)
	}
}

func TestConvertCodexSSEStreamBackfillsCompletedText(t *testing.T) {
	// No deltas — only completed.output message text.
	sse := strings.Join([]string{
		`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1},"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"only-final"}]}]}}`,
		``,
	}, "\n")
	var buf bytes.Buffer
	text, _, _, err := convertCodexSSEToChat(strings.NewReader(sse), &buf, testBridgeOpts("gpt-5", "id1"))
	if err != nil {
		t.Fatal(err)
	}
	if text != "only-final" {
		t.Fatalf("text %q", text)
	}
	out := buf.String()
	if !strings.Contains(out, `"content":"only-final"`) {
		t.Fatalf("stream missing backfill: %s", out)
	}
	if strings.Count(out, `"content":"only-final"`) != 1 {
		t.Fatalf("backfill should be once: %s", out)
	}
}
