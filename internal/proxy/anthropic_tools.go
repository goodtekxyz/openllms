package proxy

import (
	"encoding/json"
	"strings"
)

const anthropicWebSearchToolType = "web_search_20250305"

// mapClientToolsToAnthropic converts OpenAI function tools into Anthropic Messages tools.
func mapClientToolsToAnthropic(toolsRaw json.RawMessage, toolChoiceRaw json.RawMessage, llmsRaw json.RawMessage) (upstreamTools []any, upstreamToolChoice any, err error) {
	if len(toolsRaw) == 0 || string(toolsRaw) == "null" {
		return nil, nil, nil
	}
	var tools []struct {
		Type     string `json:"type"`
		Function struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Parameters  json.RawMessage `json:"parameters"`
		} `json:"function"`
	}
	if err := json.Unmarshal(toolsRaw, &tools); err != nil {
		return nil, nil, err
	}
	if !anthropicWebSearchEnabled(llmsRaw) {
		filtered := make([]struct {
			Type     string `json:"type"`
			Function struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				Parameters  json.RawMessage `json:"parameters"`
			} `json:"function"`
		}, 0, len(tools))
		for _, t := range tools {
			if strings.TrimSpace(t.Function.Name) != "web_search" {
				filtered = append(filtered, t)
			}
		}
		tools = filtered
	}
	var out []any
	hasWebSearch := false
	for _, t := range tools {
		if t.Type != "" && t.Type != "function" {
			continue
		}
		name := strings.TrimSpace(t.Function.Name)
		if name == "" {
			continue
		}
		switch name {
		case "web_search":
			out = append(out, map[string]any{
				"type": anthropicWebSearchToolType,
				"name": "web_search",
			})
			hasWebSearch = true
		default:
			out = append(out, anthropicCustomTool(name, t.Function.Description, t.Function.Parameters))
		}
	}
	if len(out) == 0 {
		return nil, nil, nil
	}
	tc := mapAnthropicToolChoice(toolChoiceRaw, hasWebSearch)
	return out, tc, nil
}

func anthropicWebSearchEnabled(llmsRaw json.RawMessage) bool {
	return !llmsWebSearchDisabled(llmsRaw)
}

func anthropicCustomTool(name, description string, params json.RawMessage) map[string]any {
	t := map[string]any{"name": name}
	if description != "" {
		t["description"] = description
	}
	schema := map[string]any{"type": "object", "properties": map[string]any{}}
	if len(params) > 0 && string(params) != "null" {
		var p map[string]any
		if json.Unmarshal(params, &p) == nil && p != nil {
			schema = p
		}
	}
	t["input_schema"] = schema
	return t
}

func mapAnthropicToolChoice(raw json.RawMessage, hasWebSearch bool) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		switch strings.ToLower(s) {
		case "none":
			return map[string]any{"type": "none"}
		case "required":
			if hasWebSearch {
				return map[string]any{"type": "tool", "name": "web_search"}
			}
			return map[string]any{"type": "any"}
		case "auto", "":
			return map[string]any{"type": "auto"}
		}
	}
	var obj map[string]any
	if json.Unmarshal(raw, &obj) == nil {
		if fn, ok := obj["function"].(map[string]any); ok {
			if name, _ := fn["name"].(string); name != "" {
				switch name {
				case "web_search":
					return map[string]any{"type": "tool", "name": "web_search"}
				default:
					return map[string]any{"type": "tool", "name": name}
				}
			}
		}
	}
	return nil
}

func anthropicToolCallFromBlock(block map[string]any) (map[string]any, bool) {
	typ, _ := block["type"].(string)
	if typ != "tool_use" {
		return nil, false
	}
	id, _ := block["id"].(string)
	name, _ := block["name"].(string)
	if name == "" {
		return nil, false
	}
	args := "{}"
	if input, ok := block["input"]; ok && input != nil {
		if b, err := json.Marshal(input); err == nil {
			args = string(b)
		}
	}
	return map[string]any{
		"id":   id,
		"type": "function",
		"function": map[string]any{
			"name":      name,
			"arguments": args,
		},
	}, true
}

func anthropicHostedToolFromServerUse(block map[string]any) (map[string]any, bool) {
	typ, _ := block["type"].(string)
	if typ != "server_tool_use" {
		return nil, false
	}
	name, _ := block["name"].(string)
	if name != "web_search" {
		return nil, false
	}
	entry := map[string]any{"name": "web_search", "status": "in_progress"}
	if id, _ := block["id"].(string); id != "" {
		entry["id"] = id
	}
	if action := searchActionFromInput(block["input"]); action != nil {
		entry["action"] = action
	}
	return entry, true
}

func anthropicHostedToolFromSearchResult(block map[string]any, pending map[string]map[string]any) (map[string]any, bool) {
	typ, _ := block["type"].(string)
	if typ != "web_search_tool_result" {
		return nil, false
	}
	toolUseID, _ := block["tool_use_id"].(string)
	entry := map[string]any{"name": "web_search", "status": "completed"}
	if toolUseID != "" {
		entry["id"] = toolUseID
		if prev, ok := pending[toolUseID]; ok {
			if action, ok := prev["action"]; ok {
				entry["action"] = action
			}
		}
	}
	return entry, true
}

func searchActionFromInput(input any) map[string]any {
	switch v := input.(type) {
	case map[string]any:
		if q, _ := v["query"].(string); strings.TrimSpace(q) != "" {
			return map[string]any{"type": "search", "query": q}
		}
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		var m map[string]any
		if json.Unmarshal([]byte(v), &m) == nil {
			return searchActionFromInput(m)
		}
	}
	return nil
}

func mergePartialJSON(existing string, delta string) string {
	if delta == "" {
		return existing
	}
	return existing + delta
}
