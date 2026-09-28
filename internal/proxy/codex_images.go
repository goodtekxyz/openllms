package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const codexImagesDefaultModel = "gpt-5"

const codexImagesInstructions = "Use the image_generation tool to generate exactly one image for the user request. Do not use any other tool."

func imagesToCodexResponses(imagesBody []byte) (responsesBody []byte, reasoningModel string, err error) {
	var in struct {
		Model      string `json:"model"`
		Prompt     string `json:"prompt"`
		Size       string `json:"size"`
		Quality    string `json:"quality"`
		Background string `json:"background"`
		N          int    `json:"n"`
	}
	if err := json.Unmarshal(imagesBody, &in); err != nil {
		return nil, "", err
	}
	prompt := strings.TrimSpace(in.Prompt)
	if prompt == "" {
		return nil, "", fmt.Errorf("codex images: prompt required")
	}

	tool := codexImageGenerationToolOpts(in.Size, in.Quality, in.Background)
	reasoningModel = codexImagesDefaultModel
	m := strings.TrimSpace(in.Model)
	if m != "" {
		if strings.HasPrefix(strings.ToLower(m), "gpt-image") {
			tool["model"] = m
		} else {
			reasoningModel = m
		}
	}

	out := map[string]any{
		"model":        reasoningModel,
		"instructions": codexImagesInstructions,
		"input": []map[string]any{{
			"type": "message", "role": "user",
			"content": []map[string]any{{"type": "input_text", "text": prompt}},
		}},
		"tools":       []any{tool},
		"tool_choice": map[string]any{"type": "image_generation"},
		"store":       false,
		"stream":      true,
	}
	b, err := json.Marshal(out)
	return b, reasoningModel, err
}

func imagesRequestCount(imagesBody []byte) int {
	var in struct {
		N int `json:"n"`
	}
	_ = json.Unmarshal(imagesBody, &in)
	if in.N < 1 {
		return 1
	}
	if in.N > 4 {
		return 4
	}
	return in.N
}

func aggregateCodexImagesSSE(r io.Reader) (body []byte, err error) {
	var b64s []string
	var lastErr string
	err = forEachCodexSSEEvent(r, func(typ string, obj map[string]any) error {
		switch typ {
		case "response.failed", "response.incomplete":
			if resp, ok := obj["response"].(map[string]any); ok {
				if e, ok := resp["error"].(map[string]any); ok {
					if msg, _ := e["message"].(string); msg != "" {
						lastErr = msg
					}
				}
			}
		case "response.output_item.done", "response.output_item.added":
			if item, ok := obj["item"].(map[string]any); ok {
				if b64 := imageGenerationB64(item); b64 != "" {
					b64s = append(b64s, b64)
				}
			}
		case "response.completed":
			if resp, ok := obj["response"].(map[string]any); ok {
				if out, ok := resp["output"].([]any); ok {
					for _, it := range out {
						item, ok := it.(map[string]any)
						if !ok {
							continue
						}
						if b64 := imageGenerationB64(item); b64 != "" {
							b64s = append(b64s, b64)
						}
					}
				}
			}
		default:
			if item, ok := obj["item"].(map[string]any); ok {
				if b64 := imageGenerationB64(item); b64 != "" {
					b64s = append(b64s, b64)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(b64s) == 0 {
		if lastErr != "" {
			return nil, fmt.Errorf("codex images: %s", lastErr)
		}
		return nil, fmt.Errorf("codex images: no image_generation_call result")
	}
	data := make([]map[string]string, 0, len(b64s))
	seen := map[string]struct{}{}
	for _, b := range b64s {
		if _, ok := seen[b]; ok {
			continue
		}
		seen[b] = struct{}{}
		data = append(data, map[string]string{"b64_json": b})
	}
	out := map[string]any{"created": time.Now().Unix(), "data": data}
	return json.Marshal(out)
}

func imageGenerationB64(item map[string]any) string {
	typ, _ := item["type"].(string)
	if typ != "image_generation_call" {
		return ""
	}
	result, _ := item["result"].(string)
	return strings.TrimSpace(result)
}
