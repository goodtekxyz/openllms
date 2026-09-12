package proxy

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// forEachCodexSSEEvent scans Codex Responses SSE lines and calls handle for each JSON data event.
func forEachCodexSSEEvent(r io.Reader, handle func(typ string, obj map[string]any) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var eventName string
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			if line == "" {
				eventName = ""
			}
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(payload), &obj) != nil {
			continue
		}
		typ, _ := obj["type"].(string)
		if typ == "" {
			typ = eventName
		}
		if err := handle(typ, obj); err != nil {
			return err
		}
	}
	return sc.Err()
}
