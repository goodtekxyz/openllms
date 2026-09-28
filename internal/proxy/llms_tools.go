package proxy

import (
	"encoding/json"
	"strings"
)

// llmsWebSearchMode returns live, cached, disabled, or "" (default live).
func llmsWebSearchMode(llmsRaw json.RawMessage) string {
	if len(llmsRaw) == 0 {
		return ""
	}
	var ext struct {
		WebSearch struct {
			Mode string `json:"mode"`
		} `json:"web_search"`
	}
	if json.Unmarshal(llmsRaw, &ext) != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(ext.WebSearch.Mode))
}

func llmsWebSearchDisabled(llmsRaw json.RawMessage) bool {
	return llmsWebSearchMode(llmsRaw) == "disabled"
}

// codexWebSearchLive returns external_web_access for Codex web_search, or nil when disabled.
func codexWebSearchLive(llmsRaw json.RawMessage) *bool {
	switch llmsWebSearchMode(llmsRaw) {
	case "disabled":
		return nil
	case "cached":
		v := false
		return &v
	default:
		v := true
		return &v
	}
}
