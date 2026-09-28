package proxy

import (
	"encoding/json"
	"strings"
)

// chatImageRef is an OpenAI chat image_url part resolved to a URL (http(s) or data URI).
type chatImageRef struct {
	URL    string
	Detail string
}

// parseChatContentParts walks OpenAI chat message content (string or parts array).
// Text parts and image_url parts are preserved; unknown types are skipped.
func parseChatContentParts(raw json.RawMessage) (texts []string, images []chatImageRef) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if s != "" {
			return []string{s}, nil
		}
		return nil, nil
	}
	var parts []json.RawMessage
	if json.Unmarshal(raw, &parts) != nil {
		if t := strings.TrimSpace(string(raw)); t != "" && t != "null" {
			return []string{t}, nil
		}
		return nil, nil
	}
	for _, p := range parts {
		var tip struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(p, &tip) != nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(tip.Type)) {
		case "text", "input_text", "output_text", "":
			if tip.Text != "" {
				texts = append(texts, tip.Text)
			}
		case "image_url":
			if img, ok := parseChatImageURLPart(p); ok {
				images = append(images, img)
			}
		}
	}
	return texts, images
}

func parseChatImageURLPart(raw json.RawMessage) (chatImageRef, bool) {
	var obj struct {
		ImageURL json.RawMessage `json:"image_url"`
	}
	if json.Unmarshal(raw, &obj) != nil || len(obj.ImageURL) == 0 {
		return chatImageRef{}, false
	}
	var url string
	if json.Unmarshal(obj.ImageURL, &url) == nil && strings.TrimSpace(url) != "" {
		return chatImageRef{URL: strings.TrimSpace(url)}, true
	}
	var nested struct {
		URL    string `json:"url"`
		Detail string `json:"detail"`
	}
	if json.Unmarshal(obj.ImageURL, &nested) != nil || strings.TrimSpace(nested.URL) == "" {
		return chatImageRef{}, false
	}
	return chatImageRef{URL: strings.TrimSpace(nested.URL), Detail: strings.TrimSpace(nested.Detail)}, true
}

// chatContentToCodexParts maps OpenAI chat content to Responses message content parts.
// forAssistant selects output_text vs input_text; images are always input_image (user-side).
func chatContentToCodexParts(raw json.RawMessage, forAssistant bool) []map[string]any {
	texts, images := parseChatContentParts(raw)
	textType := "input_text"
	if forAssistant {
		textType = "output_text"
	}
	var out []map[string]any
	for _, t := range texts {
		out = append(out, map[string]any{"type": textType, "text": t})
	}
	if forAssistant {
		return out // assistant history: text/tool_calls only
	}
	for _, img := range images {
		part := map[string]any{"type": "input_image", "image_url": img.URL}
		if img.Detail != "" {
			part["detail"] = img.Detail
		}
		out = append(out, part)
	}
	return out
}

// chatContentToAnthropicBlocks maps OpenAI chat content to Anthropic content blocks.
func chatContentToAnthropicBlocks(raw json.RawMessage) []map[string]any {
	texts, images := parseChatContentParts(raw)
	var out []map[string]any
	for _, t := range texts {
		out = append(out, map[string]any{"type": "text", "text": t})
	}
	for _, img := range images {
		out = append(out, anthropicImageBlock(img))
	}
	return out
}

func anthropicImageBlock(img chatImageRef) map[string]any {
	url := img.URL
	if strings.HasPrefix(url, "data:") {
		mediaType, data, ok := parseDataURI(url)
		if ok {
			return map[string]any{
				"type": "image",
				"source": map[string]any{
					"type":       "base64",
					"media_type": mediaType,
					"data":       data,
				},
			}
		}
	}
	return map[string]any{
		"type": "image",
		"source": map[string]any{
			"type": "url",
			"url":  url,
		},
	}
}

func parseDataURI(uri string) (mediaType, data string, ok bool) {
	// data:[<mediatype>][;base64],<data>
	if !strings.HasPrefix(uri, "data:") {
		return "", "", false
	}
	rest := strings.TrimPrefix(uri, "data:")
	comma := strings.IndexByte(rest, ',')
	if comma < 0 {
		return "", "", false
	}
	meta := rest[:comma]
	data = rest[comma+1:]
	if data == "" {
		return "", "", false
	}
	mediaType = "image/jpeg"
	if meta != "" {
		parts := strings.Split(meta, ";")
		if parts[0] != "" {
			mediaType = parts[0]
		}
		base64 := false
		for _, p := range parts[1:] {
			if p == "base64" {
				base64 = true
			}
		}
		if !base64 {
			return "", "", false
		}
	}
	return mediaType, data, true
}
