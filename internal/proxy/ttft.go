package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

const maxTTFTBuf = 1 << 20 // 1 MiB — if no token yet, commit anyway to avoid stalling

// awaitCommit peeks a successful stream until the first content token (or [DONE]).
// On pre-commit failure returns retryable=true and closes rc.
// On commit returns a ReadCloser that replays prefix then the rest of rc.
func awaitCommit(rc io.ReadCloser) (out io.ReadCloser, retryable bool, err error) {
	if rc == nil {
		return nil, true, io.ErrUnexpectedEOF
	}
	var buf []byte
	tmp := make([]byte, 8*1024)
	for {
		n, readErr := rc.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if len(buf) > maxTTFTBuf {
				return &replayCloser{prefix: buf, rest: rc}, false, nil
			}
			if committed, fail := inspectTTFT(buf); fail {
				_ = rc.Close()
				return nil, true, io.ErrUnexpectedEOF
			} else if committed {
				return &replayCloser{prefix: buf, rest: rc}, false, nil
			}
		}
		if readErr == io.EOF {
			if len(buf) == 0 {
				_ = rc.Close()
				return nil, true, io.ErrUnexpectedEOF
			}
			// Stream ended before clear token — treat as committed empty success if any SSE data seen.
			if committed, fail := inspectTTFT(buf); fail {
				_ = rc.Close()
				return nil, true, io.ErrUnexpectedEOF
			} else if committed || bytes.Contains(buf, []byte("data:")) {
				return &replayCloser{prefix: buf, rest: io.NopCloser(bytes.NewReader(nil))}, false, nil
			}
			_ = rc.Close()
			return nil, true, io.ErrUnexpectedEOF
		}
		if readErr != nil {
			_ = rc.Close()
			return nil, true, readErr
		}
	}
}

// inspectTTFT returns (committed, failed).
func inspectTTFT(buf []byte) (committed bool, failed bool) {
	s := string(buf)
	// Split conservatively on newlines for SSE.
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			return true, false
		}
		if payload == "" {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(payload), &raw); err != nil {
			// Incomplete JSON — wait for more bytes.
			continue
		}
		if errObj, ok := raw["error"]; ok && errObj != nil {
			return false, true
		}
		// OpenAI-style delta content / tool_calls
		if choices, ok := raw["choices"].([]any); ok && len(choices) > 0 {
			ch, _ := choices[0].(map[string]any)
			if ch == nil {
				continue
			}
			if delta, ok := ch["delta"].(map[string]any); ok {
				if c, ok := delta["content"].(string); ok && c != "" {
					return true, false
				}
				if delta["tool_calls"] != nil {
					return true, false
				}
				if delta["role"] != nil {
					// role-only chunk: not yet content; keep waiting unless finish_reason set with empty
					continue
				}
			}
			if msg, ok := ch["message"].(map[string]any); ok {
				if c, ok := msg["content"].(string); ok && c != "" {
					return true, false
				}
			}
			if fr, ok := ch["finish_reason"].(string); ok && fr != "" && fr != "null" {
				return true, false
			}
		}
		// Anthropic SSE
		if typ, _ := raw["type"].(string); typ != "" {
			switch typ {
			case "error":
				return false, true
			case "content_block_delta":
				if delta, ok := raw["delta"].(map[string]any); ok {
					if t, ok := delta["text"].(string); ok && t != "" {
						return true, false
					}
					if delta["partial_json"] != nil || delta["thinking"] != nil {
						return true, false
					}
				}
			case "content_block_start", "message_start":
				// not yet token
			case "message_stop", "message_delta":
				return true, false
			}
		}
	}
	return false, false
}

type replayCloser struct {
	prefix []byte
	rest   io.ReadCloser
	off    int
}

func (r *replayCloser) Read(p []byte) (int, error) {
	if r.off < len(r.prefix) {
		n := copy(p, r.prefix[r.off:])
		r.off += n
		return n, nil
	}
	if r.rest == nil {
		return 0, io.EOF
	}
	return r.rest.Read(p)
}

func (r *replayCloser) Close() error {
	if r.rest != nil {
		return r.rest.Close()
	}
	return nil
}
