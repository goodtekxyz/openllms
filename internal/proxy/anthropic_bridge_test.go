package proxy

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func testAnthropicBridgeOpts(model, id string) anthropicBridgeOpts {
	return anthropicBridgeOpts{model: model, id: id, accountID: "acc-claude", provider: "claude"}
}

func TestChatToAnthropicMessagesWebSearchTool(t *testing.T) {
	in := []byte(`{
	  "model":"claude-sonnet-4-5",
	  "messages":[{"role":"user","content":"CEO?"}],
	  "tools":[{"type":"function","function":{"name":"web_search","description":"search","parameters":{"type":"object","properties":{"query":{"type":"string"}}}}}],
	  "llms":{"web_search":{"mode":"live"}}
	}`)
	out, clientStream, model, err := chatToAnthropicMessages(in)
	if err != nil {
		t.Fatal(err)
	}
	if clientStream || model != "claude-sonnet-4-5" {
		t.Fatalf("stream=%v model=%s", clientStream, model)
	}
	s := string(out)
	if !strings.Contains(s, `"type":"web_search_20250305"`) || !strings.Contains(s, `"name":"web_search"`) {
		t.Fatalf("web_search mapping: %s", s)
	}
	if !strings.Contains(s, `"max_tokens":`) {
		t.Fatal("missing max_tokens")
	}
}

func TestChatToAnthropicMessagesFunctionTool(t *testing.T) {
	in := []byte(`{
	  "model":"claude-sonnet-4-5",
	  "messages":[{"role":"user","content":"go"}],
	  "tools":[{"type":"function","function":{"name":"browser_search","description":"browser","parameters":{"type":"object","properties":{"query":{"type":"string"}}}}}]
	}`)
	out, _, _, err := chatToAnthropicMessages(in)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `"name":"browser_search"`) || !strings.Contains(s, `"input_schema"`) {
		t.Fatalf("function tool: %s", s)
	}
}

func TestChatToAnthropicMessagesWebSearchDisabled(t *testing.T) {
	in := []byte(`{
	  "model":"claude-sonnet-4-5",
	  "messages":[{"role":"user","content":"hi"}],
	  "tools":[{"type":"function","function":{"name":"web_search","description":"search","parameters":{"type":"object"}}}],
	  "llms":{"web_search":{"mode":"disabled"}}
	}`)
	out, _, _, err := chatToAnthropicMessages(in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "web_search_20250305") {
		t.Fatal("web_search should be omitted when disabled")
	}
}

func TestConvertAnthropicMessageToChatWebSearch(t *testing.T) {
	msg := map[string]any{
		"role":        "assistant",
		"stop_reason": "end_turn",
		"usage":       map[string]any{"input_tokens": 10, "output_tokens": 20},
		"content": []any{
			map[string]any{"type": "text", "text": "Answer."},
			map[string]any{
				"type":  "server_tool_use",
				"id":    "srvtoolu_1",
				"name":  "web_search",
				"input": map[string]any{"query": "CEO DB Asset"},
			},
			map[string]any{
				"type":        "web_search_tool_result",
				"tool_use_id": "srvtoolu_1",
				"content":     []any{map[string]any{"type": "web_search_result", "url": "https://example.com"}},
			},
		},
	}
	out, tin, tout, err := convertAnthropicMessageToChat(msg, testAnthropicBridgeOpts("claude-sonnet-4-5", "id1"))
	if err != nil {
		t.Fatal(err)
	}
	if tin != 10 || tout != 20 {
		t.Fatalf("usage tin=%d tout=%d", tin, tout)
	}
	var parsed map[string]any
	if json.Unmarshal(out, &parsed) != nil {
		t.Fatal("bad json")
	}
	llms, ok := parsed["llms"].(map[string]any)
	if !ok {
		t.Fatal("missing llms")
	}
	hosted, ok := llms["hosted_tools"].([]any)
	if !ok || len(hosted) == 0 {
		t.Fatalf("hosted_tools: %+v", llms)
	}
	ht, _ := hosted[0].(map[string]any)
	if ht["name"] != "web_search" || ht["status"] != "completed" {
		t.Fatalf("hosted tool: %+v", ht)
	}
	choices, _ := parsed["choices"].([]any)
	ch, _ := choices[0].(map[string]any)
	message, _ := ch["message"].(map[string]any)
	if message["content"] != "Answer." {
		t.Fatalf("content: %+v", message)
	}
}

func TestConvertAnthropicMessageToChatToolUse(t *testing.T) {
	msg := map[string]any{
		"role":        "assistant",
		"stop_reason": "tool_use",
		"usage":       map[string]any{"input_tokens": 5, "output_tokens": 7},
		"content": []any{
			map[string]any{
				"type":  "tool_use",
				"id":    "toolu_1",
				"name":  "browser_search",
				"input": map[string]any{"query": "cats"},
			},
		},
	}
	out, _, _, err := convertAnthropicMessageToChat(msg, testAnthropicBridgeOpts("claude-sonnet-4-5", "id2"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	_ = json.Unmarshal(out, &parsed)
	choices, _ := parsed["choices"].([]any)
	ch, _ := choices[0].(map[string]any)
	if ch["finish_reason"] != "tool_calls" {
		t.Fatalf("finish_reason: %+v", ch)
	}
	message, _ := ch["message"].(map[string]any)
	tcs, ok := message["tool_calls"].([]any)
	if !ok || len(tcs) != 1 {
		t.Fatalf("tool_calls: %+v", message)
	}
}

func TestConvertAnthropicSSEToChat(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"type":"message_start","message":{"usage":{"input_tokens":4}}}`,
		``,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}`,
		``,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`,
		``,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")
	var buf bytes.Buffer
	text, tin, tout, err := convertAnthropicSSEToChat(strings.NewReader(sse), &buf, testAnthropicBridgeOpts("claude-sonnet-4-5", "id3"))
	if err != nil {
		t.Fatal(err)
	}
	if text != "Hi" || tin != 4 || tout != 2 {
		t.Fatalf("text=%q tin=%d tout=%d", text, tin, tout)
	}
	if !strings.Contains(buf.String(), `"object":"chat.completion.chunk"`) || !strings.Contains(buf.String(), `[DONE]`) {
		t.Fatalf("stream output: %s", buf.String())
	}
}
