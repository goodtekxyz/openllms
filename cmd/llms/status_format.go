package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const quotaBarWidth = 10

func quotaBar(remainingPct float64) string {
	pct := remainingPct
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(math.Round(pct / 100 * float64(quotaBarWidth)))
	if filled < 0 {
		filled = 0
	}
	if filled > quotaBarWidth {
		filled = quotaBarWidth
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", quotaBarWidth-filled)
}

func parseQuotaPct(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case jsonNumber:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	default:
		// encoding/json numbers are float64; also handle json.Number via fmt
		s := fmt.Sprint(v)
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil
	}
}

// jsonNumber mirrors encoding/json.Number without importing for the type switch path.
type jsonNumber interface {
	Float64() (float64, error)
}

func formatReset(v any, now time.Time) string {
	if v == nil {
		return ""
	}
	t, ok := parseTimeAny(v)
	if !ok {
		return fmt.Sprint(v)
	}
	d := t.Sub(now)
	if d < 0 {
		return "reset past"
	}
	if d < time.Minute {
		return "reset ~now"
	}
	if d < time.Hour {
		return fmt.Sprintf("reset ~%dm", int(d.Minutes()))
	}
	if d < 48*time.Hour {
		h := int(d.Hours())
		if h < 1 {
			h = 1
		}
		return fmt.Sprintf("reset ~%dh", h)
	}
	days := int(d.Hours() / 24)
	if days < 1 {
		days = 1
	}
	return fmt.Sprintf("reset ~%dd", days)
}

func parseTimeAny(v any) (time.Time, bool) {
	switch x := v.(type) {
	case time.Time:
		return x, true
	case string:
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
			if t, err := time.Parse(layout, x); err == nil {
				return t, true
			}
		}
	}
	s := fmt.Sprint(v)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func formatAccountLine(m map[string]any, refWidth int, now time.Time) string {
	glyph := fmt.Sprint(m["glyph"])
	ref := fmt.Sprint(m["ref"])
	health := fmt.Sprint(m["health"])
	line := fmt.Sprintf("  %s  %-*s  %-8s", glyph, refWidth, ref, health)
	hasDual := false
	if q, ok := m["quota_5h_remaining_pct"]; ok && q != nil {
		if pct, ok := parseQuotaPct(q); ok {
			hasDual = true
			line += fmt.Sprintf("  5h %3.0f%% %s", pct, quotaBar(pct))
			if ra, ok := m["quota_5h_reset_at"]; ok && ra != nil {
				if rs := formatReset(ra, now); rs != "" {
					line += " " + rs
				}
			}
		}
	}
	if q, ok := m["quota_7d_remaining_pct"]; ok && q != nil {
		if pct, ok := parseQuotaPct(q); ok {
			hasDual = true
			line += fmt.Sprintf("  7d %3.0f%% %s", pct, quotaBar(pct))
			if ra, ok := m["quota_7d_reset_at"]; ok && ra != nil {
				if rs := formatReset(ra, now); rs != "" {
					line += " " + rs
				}
			}
		}
	}
	if !hasDual {
		if q, ok := m["quota_remaining_pct"]; ok && q != nil {
			if pct, ok := parseQuotaPct(q); ok {
				line += fmt.Sprintf("  quota %3.0f%% %s", pct, quotaBar(pct))
			}
		} else {
			refS := ref
			if strings.HasPrefix(refS, "codex:") || strings.HasPrefix(refS, "claude:") {
				line += "  quota  — (try: llms status --refresh)"
			}
		}
		if ra, ok := m["quota_reset_at"]; ok && ra != nil {
			if rs := formatReset(ra, now); rs != "" {
				line += "  " + rs
			}
		}
	}
	return line
}

func maxRefWidth(accounts []any) int {
	w := 12
	for _, raw := range accounts {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		n := len(fmt.Sprint(m["ref"]))
		if n > w {
			w = n
		}
	}
	return w
}

func parseUsageInt(v any) (int64, bool) {
	if v == nil {
		return 0, false
	}
	switch x := v.(type) {
	case float64:
		return int64(math.Round(x)), true
	case float32:
		return int64(math.Round(float64(x))), true
	case int:
		return int64(x), true
	case int64:
		return x, true
	case int32:
		return int64(x), true
	case uint64:
		return int64(x), true
	case jsonNumber:
		f, err := x.Float64()
		if err != nil {
			return 0, false
		}
		return int64(math.Round(f)), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err != nil {
			return 0, false
		}
		return int64(math.Round(f)), true
	default:
		f, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
		if err != nil {
			return 0, false
		}
		return int64(math.Round(f)), true
	}
}

// formatUsageCount renders API usage integers with thousand separators; nil → "—".
func formatUsageCount(v any) string {
	if v == nil {
		return "—"
	}
	n, ok := parseUsageInt(v)
	if !ok {
		return fmt.Sprint(v)
	}
	return formatCommaInt(n)
}

func formatCommaInt(n int64) string {
	if n < 0 {
		if n == math.MinInt64 {
			return "-" + formatCommaInt(math.MaxInt64) + ",922,337,203,685,477,580"
		}
		return "-" + formatCommaInt(-n)
	}
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	nMod := len(s) % 3
	if nMod == 0 {
		nMod = 3
	}
	var b strings.Builder
	b.WriteString(s[:nMod])
	for i := nMod; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func formatUsageMonthLine(u map[string]any) string {
	return fmt.Sprintf("  requests=%s tokens=%s soft_cap_tokens=%s",
		formatUsageCount(u["requests"]),
		formatUsageCount(u["tokens_total"]),
		formatUsageCount(u["soft_cap_tokens"]),
	)
}
