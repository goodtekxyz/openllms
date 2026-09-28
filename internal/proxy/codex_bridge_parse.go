package proxy

import (
	"io"
	"time"
)

// parseCodexSSE drives chat-bridge state from upstream Responses SSE.
func parseCodexSSE(r io.Reader, state *codexParseState, w io.Writer, opts codexBridgeOpts) error {
	streaming := w != nil
	return forEachCodexSSEEvent(r, func(typ string, obj map[string]any) error {
		switch typ {
		case "response.output_text.delta":
			delta, _ := obj["delta"].(string)
			if delta == "" {
				return nil
			}
			state.text.WriteString(delta)
			if streaming {
				state.contentEmitted = true
				return state.streamWriter.WriteChunk(map[string]any{
					"id": opts.id, "object": "chat.completion.chunk",
					"created": time.Now().Unix(), "model": opts.model,
					"choices": []map[string]any{{
						"index": 0, "delta": map[string]any{"content": delta},
					}},
				})
			}
		case "response.output_item.added":
			// Ingest for hosted-tool "started" + tool_calls state merge only.
			// Do not emit OpenAI tool_calls chunks here — Arguments are often
			// incomplete on added, and done would double-append for SDKs that
			// concatenate function.arguments.
			if item, ok := obj["item"].(map[string]any); ok {
				state.ingestOutputItem(item, streaming)
			}
		case "response.output_item.done":
			if item, ok := obj["item"].(map[string]any); ok {
				state.ingestOutputItem(item, streaming)
				if streaming {
					if tc, ok := codexToolCallFromItem(item); ok {
						idx := toolCallStreamIndex(state.toolCalls, tc)
						return state.streamWriter.WriteChunk(map[string]any{
							"id": opts.id, "object": "chat.completion.chunk",
							"created": time.Now().Unix(), "model": opts.model,
							"choices": []map[string]any{{
								"index": 0,
								"delta": map[string]any{"tool_calls": []map[string]any{{
									"index": idx, "id": tc["id"], "type": "function",
									"function": tc["function"],
								}}},
							}},
						})
					}
				}
			}
		case "response.completed", "response.incomplete":
			if resp, ok := obj["response"].(map[string]any); ok {
				if u, ok := resp["usage"].(map[string]any); ok {
					state.tin = anyInt(u["input_tokens"])
					state.tout = anyInt(u["output_tokens"])
				}
				parseCompletedOutput(resp, state)
			}
		default:
			if item, ok := obj["item"].(map[string]any); ok {
				state.ingestOutputItem(item, streaming)
			}
		}
		return nil
	})
}

func writeCodexStreamFinale(state *codexParseState, w io.Writer, opts codexBridgeOpts) error {
	// If upstream sent completed.output text but no output_text.delta events,
	// backfill one content chunk so stream clients match aggregate.
	if !state.contentEmitted {
		if final := state.text.String(); final != "" {
			state.contentEmitted = true
			if err := state.streamWriter.WriteChunk(map[string]any{
				"id": opts.id, "object": "chat.completion.chunk",
				"created": time.Now().Unix(), "model": opts.model,
				"choices": []map[string]any{{
					"index": 0, "delta": map[string]any{"content": final},
				}},
			}); err != nil {
				return err
			}
		}
	}
	finish := "stop"
	if len(state.toolCalls) > 0 {
		finish = "tool_calls"
	}
	fin := map[string]any{
		"id": opts.id, "object": "chat.completion.chunk",
		"created": time.Now().Unix(), "model": opts.model,
		"choices": []map[string]any{{
			"index": 0, "delta": map[string]any{}, "finish_reason": finish,
		}},
	}
	if state.tin+state.tout > 0 {
		fin["usage"] = map[string]any{
			"prompt_tokens": state.tin, "completion_tokens": state.tout,
			"total_tokens": state.tin + state.tout,
		}
	}
	if meta := state.llmsMeta(); len(meta) > 0 {
		fin["llms"] = meta
	}
	if err := state.streamWriter.WriteChunk(fin); err != nil {
		return err
	}
	_, err := io.WriteString(w, "data: [DONE]\n\n")
	return err
}

func newCodexParseState(opts codexBridgeOpts, w io.Writer) *codexParseState {
	st := &codexParseState{
		accountID: opts.accountID,
		provider:  opts.provider,
		model:     opts.model,
		id:        opts.id,
	}
	if w != nil {
		st.streamWriter = &sseWriter{w: w, model: opts.model, id: opts.id}
	}
	return st
}

func (st *codexParseState) llmsMeta() map[string]any {
	meta := map[string]any{}
	if st.accountID != "" {
		meta["account_id"] = st.accountID
	}
	if st.provider != "" {
		meta["provider"] = st.provider
	}
	if len(st.hostedTools) > 0 {
		meta["hosted_tools"] = st.hostedTools
	}
	return meta
}
