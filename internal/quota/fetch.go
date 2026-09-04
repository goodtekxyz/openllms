package quota

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Snapshot is remaining headroom for routing / status.
type Snapshot struct {
	RemainingPct float64
	ResetAt      *time.Time
	Source       string // primary window label
}

// HTTPClient is overridable in tests.
var HTTPClient *http.Client

func client() *http.Client {
	if HTTPClient != nil {
		return HTTPClient
	}
	return &http.Client{Timeout: 20 * time.Second}
}

var (
	CodexUsageURL  = "https://chatgpt.com/backend-api/wham/usage"
	ClaudeUsageURL = "https://api.anthropic.com/api/oauth/usage"
)

// FetchCodex calls ChatGPT Codex usage (undocumented; same as Codex CLI).
func FetchCodex(ctx context.Context, accessToken, accountID string) (Snapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, CodexUsageURL, nil)
	if err != nil {
		return Snapshot{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	if accountID != "" {
		req.Header.Set("ChatGPT-Account-Id", accountID)
	}
	req.Header.Set("User-Agent", "codex-cli")
	res, err := client().Do(req)
	if err != nil {
		return Snapshot{}, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return Snapshot{}, fmt.Errorf("codex usage: %s %s", res.Status, truncate(b, 200))
	}
	return ParseCodexUsage(b)
}

// FetchClaude calls Anthropic oauth usage (undocumented; same as Claude Code /usage).
func FetchClaude(ctx context.Context, accessToken string) (Snapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ClaudeUsageURL, nil)
	if err != nil {
		return Snapshot{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("User-Agent", "claude-code/2.1.72")
	req.Header.Set("Accept", "application/json")
	res, err := client().Do(req)
	if err != nil {
		return Snapshot{}, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return Snapshot{}, fmt.Errorf("claude usage: %s %s", res.Status, truncate(b, 200))
	}
	return ParseClaudeUsage(b)
}

func ParseCodexUsage(b []byte) (Snapshot, error) {
	var out struct {
		RateLimit struct {
			PrimaryWindow   *codexWindow `json:"primary_window"`
			SecondaryWindow *codexWindow `json:"secondary_window"`
		} `json:"rate_limit"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return Snapshot{}, err
	}
	type named struct {
		name string
		w    *codexWindow
	}
	cands := []named{{"5h", out.RateLimit.PrimaryWindow}, {"7d", out.RateLimit.SecondaryWindow}}
	var bestUsed float64 = -1
	var snap Snapshot
	for _, c := range cands {
		if c.w == nil {
			continue
		}
		used := c.w.UsedPercent
		if used > bestUsed {
			bestUsed = used
			snap = Snapshot{
				RemainingPct: clampPct(100 - used),
				ResetAt:      unixOrNil(c.w.ResetAt),
				Source:       "codex:" + c.name,
			}
		}
	}
	if bestUsed < 0 {
		return Snapshot{}, fmt.Errorf("codex usage: no rate_limit windows")
	}
	return snap, nil
}

type codexWindow struct {
	UsedPercent float64 `json:"used_percent"`
	ResetAt     int64   `json:"reset_at"`
}

func ParseClaudeUsage(b []byte) (Snapshot, error) {
	var out struct {
		FiveHour *claudeBucket `json:"five_hour"`
		SevenDay *claudeBucket `json:"seven_day"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return Snapshot{}, err
	}
	type named struct {
		name string
		w    *claudeBucket
	}
	cands := []named{{"5h", out.FiveHour}, {"7d", out.SevenDay}}
	var bestUsed float64 = -1
	var snap Snapshot
	for _, c := range cands {
		if c.w == nil {
			continue
		}
		used := c.w.Utilization
		if used > bestUsed {
			bestUsed = used
			snap = Snapshot{
				RemainingPct: clampPct(100 - used),
				ResetAt:      parseRFC3339(c.w.ResetsAt),
				Source:       "claude:" + c.name,
			}
		}
	}
	if bestUsed < 0 {
		return Snapshot{}, fmt.Errorf("claude usage: no buckets")
	}
	return snap, nil
}

type claudeBucket struct {
	Utilization float64 `json:"utilization"`
	ResetsAt    string  `json:"resets_at"`
}

func clampPct(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func unixOrNil(sec int64) *time.Time {
	if sec <= 0 {
		return nil
	}
	t := time.Unix(sec, 0).UTC()
	return &t
}

func parseRFC3339(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return nil
		}
	}
	t = t.UTC()
	return &t
}

func truncate(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		return s[:n]
	}
	return s
}
