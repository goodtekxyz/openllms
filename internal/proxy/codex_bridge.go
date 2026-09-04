package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const codexDefaultInstructions = "You are a helpful assistant."

// chatToCodexResponses converts an OpenAI chat.completions body to Codex Responses API body.
func chatToCodexResponses(chatBody []byte) (responsesBody []byte, clientStream bool, model string, err error) {
	var in struct {
		Model      string          `json:"model"`
		Stream     bool            `json:"stream"`
		Tools      json.RawMessage `json:"tools"`
		ToolChoice json.RawMessage `json:"tool_choice"`
		LLMS       json.RawMessage `json:"llms"`
		Messages   []struct {
			Role       string          `json:"role"`
			Content    json.RawMessage `json:"content"`
			ToolCalls  json.RawMessage `json:"tool_calls"`
			ToolCallID string          `json:"tool_call_id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(chatBody, &in); err != nil {
		return nil, false, "", err
	}
	model = in.Model
	if model == "" {
		model = "gpt-5"
	}
	var instructions strings.Builder
	var input []map[string]any
	for _, m := range in.Messages {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		switch role {
		case "system", "developer":
			text := flattenChatContent(m.Content)
			if text == "" {
				continue
			}
			if instructions.Len() > 0 {
				instructions.WriteByte('\n')
			}
			instructions.WriteString(text)
		case "tool":
			callID := strings.TrimSpace(m.ToolCallID)
			if callID == "" {
				continue
			}
			input = append(input, map[string]any{
				"type":    "function_call_output",
				"call_id": callID,
				"output":  flattenChatContent(m.Content),
			})
		case "assistant":
			text := flattenChatContent(m.Content)
			if text != "" {
				input = append(input, map[string]any{
					"type": "message", "role": "assistant",
					"content": []map[string]any{{"type": "output_text", "text": text}},
				})
			}
			for _, fc := range responsesFunctionCallsFromToolCalls(m.ToolCalls) {
				input = append(input, fc)
			}
		default:
			text := flattenChatContent(m.Content)
			if text == "" {
				continue
			}
			input = append(input, map[string]any{
				"type": "message", "role": "user",
				"content": []map[string]any{{"type": "input_text", "text": text}},
			})
		}
	}
	instr := strings.TrimSpace(instructions.String())
	if instr == "" {
		instr = codexDefaultInstructions
	}
	if len(input) == 0 {
		return nil, false, "", fmt.Errorf("codex bridge: no user/assistant messages")
	}
	out := map[string]any{
		"model": model, "instructions": instr, "input": input,
		"store": false, "stream": true,
	}
	if tools, tc, err := mapClientToolsToCodex(in.Tools, in.ToolChoice, in.LLMS); err != nil {
		return nil, false, "", err
	} else if len(tools) > 0 {
		out["tools"] = tools
		if tc != nil {
			out["tool_choice"] = tc
		}
	}
	b, err := json.Marshal(out)
	return b, in.Stream, model, err
}

// chatRequestHasTools reports whether the client sent a non-empty tools array.
func chatRequestHasTools(body []byte) bool {
	var peek struct {
		Tools json.RawMessage `json:"tools"`
	}
	if json.Unmarshal(body, &peek) != nil || len(peek.Tools) == 0 || string(peek.Tools) == "null" {
		return false
	}
	var tools []json.RawMessage
	if json.Unmarshal(peek.Tools, &tools) != nil {
		return false
	}
	return len(tools) > 0
}

func responsesFunctionCallsFromToolCalls(raw json.RawMessage) []map[string]any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var calls []struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
	if json.Unmarshal(raw, &calls) != nil {
		return nil
	}
	var out []map[string]any
	for _, c := range calls {
		name := strings.TrimSpace(c.Function.Name)
		callID := strings.TrimSpace(c.ID)
		if name == "" || callID == "" {
			continue
		}
		args := strings.TrimSpace(c.Function.Arguments)
		if args == "" {
			args = "{}"
		}
		out = append(out, map[string]any{
			"type":      "function_call",
			"call_id":   callID,
			"name":      name,
			"arguments": args,
		})
	}
	return out
}

func flattenChatContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		var b strings.Builder
		for _, p := range parts {
			if p.Text != "" {
				b.WriteString(p.Text)
			}
		}
		return b.String()
	}
	return strings.TrimSpace(string(raw))
}

type codexBridgeOpts struct {
	model     string
	id        string
	accountID string
	provider  string
}

func convertCodexSSEToChat(r io.Reader, w io.Writer, opts codexBridgeOpts) (text string, tin, tout int, err error) {
	if opts.id == "" {
		opts.id = "chatcmpl-codex"
	}
	state := newCodexParseState(opts, w)
	if err := parseCodexSSE(r, state, w, opts); err != nil {
		return state.text.String(), state.tin, state.tout, err
	}
	if w != nil {
		if err := writeCodexStreamFinale(state, w, opts); err != nil {
			return state.text.String(), state.tin, state.tout, err
		}
	}
	return state.text.String(), state.tin, state.tout, nil
}

func extractCompletedText(resp map[string]any) string {
	out, _ := resp["output"].([]any)
	var b strings.Builder
	for _, item := range out {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if t := messageItemText(m); t != "" {
			b.WriteString(t)
		}
	}
	return b.String()
}

func messageItemText(m map[string]any) string {
	content, _ := m["content"].([]any)
	for _, c := range content {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := cm["text"].(string); t != "" {
			return t
		}
	}
	return ""
}

func anyInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case json.Number:
		i, _ := x.Int64()
		return int(i)
	default:
		return 0
	}
}

type pipeCodexToChat struct {
	pr   *io.PipeReader
	done chan struct{}
}

func startCodexChatPipe(upstream io.ReadCloser, opts codexBridgeOpts) *pipeCodexToChat {
	pr, pw := io.Pipe()
	p := &pipeCodexToChat{pr: pr, done: make(chan struct{})}
	go func() {
		defer close(p.done)
		defer upstream.Close()
		defer pw.Close()
		_, _, _, err := convertCodexSSEToChat(upstream, pw, opts)
		if err != nil {
			_ = pw.CloseWithError(err)
		}
	}()
	return p
}

func (p *pipeCodexToChat) Read(b []byte) (int, error) { return p.pr.Read(b) }
func (p *pipeCodexToChat) Close() error {
	err := p.pr.Close()
	<-p.done
	return err
}

func aggregateCodexSSE(r io.Reader, opts codexBridgeOpts) (body []byte, tin, tout int, err error) {
	if opts.id == "" {
		opts.id = "chatcmpl-codex"
	}
	state := newCodexParseState(opts, nil)
	if err := parseCodexSSE(r, state, nil, opts); err != nil {
		return nil, state.tin, state.tout, err
	}
	return state.buildCompletionJSON(), state.tin, state.tout, nil
}
