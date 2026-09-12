package main

import (
	"fmt"
	"strings"
)

type modelRow struct {
	ID         string
	Providers  []string
	AccountIDs []string
}

func parseUnionModels(raw any) []modelRow {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]modelRow, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := strings.TrimSpace(fmt.Sprint(m["id"]))
		if id == "" || id == "<nil>" {
			continue
		}
		out = append(out, modelRow{
			ID:         id,
			Providers:  anyStringSlice(m["providers"]),
			AccountIDs: anyStringSlice(m["account_ids"]),
		})
	}
	return out
}

func anyStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		s := strings.TrimSpace(fmt.Sprint(x))
		if s == "" || s == "<nil>" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func filterModelRows(rows []modelRow, filter string) []modelRow {
	f := strings.ToLower(strings.TrimSpace(filter))
	if f == "" {
		return rows
	}
	out := make([]modelRow, 0, len(rows))
	for _, r := range rows {
		if modelRowMatches(r, f) {
			out = append(out, r)
		}
	}
	return out
}

func modelRowMatches(r modelRow, filterLower string) bool {
	if strings.Contains(strings.ToLower(r.ID), filterLower) {
		return true
	}
	for _, p := range r.Providers {
		if strings.Contains(strings.ToLower(p), filterLower) {
			return true
		}
	}
	for _, a := range r.AccountIDs {
		if strings.Contains(strings.ToLower(a), filterLower) {
			return true
		}
	}
	return false
}

func formatModelLine(r modelRow, idWidth int) string {
	prov := strings.Join(r.Providers, ",")
	if prov == "" {
		prov = "-"
	}
	acc := strings.Join(r.AccountIDs, ",")
	if acc == "" {
		acc = "-"
	}
	return fmt.Sprintf("  %-*s  providers=%-12s  accounts=%s", idWidth, r.ID, prov, acc)
}

func maxModelIDWidth(rows []modelRow) int {
	w := 12
	for _, r := range rows {
		if n := len(r.ID); n > w {
			w = n
		}
	}
	return w
}

func formatModelsBoard(route, strategy, suggested string, rows []modelRow, accounts []any) string {
	var b strings.Builder
	b.WriteString("ROUTE  ")
	b.WriteString(route)
	if strategy != "" {
		b.WriteString("  ")
		b.WriteString(strategy)
	}
	b.WriteByte('\n')
	if suggested != "" {
		b.WriteString("SUGGESTED  ")
		b.WriteString(suggested)
		b.WriteByte('\n')
	}
	b.WriteString("MODELS\n")
	if len(rows) == 0 {
		b.WriteString("  (none)\n")
	} else {
		w := maxModelIDWidth(rows)
		for _, r := range rows {
			b.WriteString(formatModelLine(r, w))
			b.WriteByte('\n')
		}
	}
	b.WriteString("ACCOUNTS\n")
	if len(accounts) == 0 {
		b.WriteString("  (none)\n")
	} else {
		for _, raw := range accounts {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			ref := fmt.Sprint(m["id"])
			status := fmt.Sprint(m["status"])
			line := fmt.Sprintf("  %-20s  %s", ref, status)
			if errMsg, ok := m["error"].(string); ok && errMsg != "" {
				line += "  " + errMsg
			}
			n := 0
			if models, ok := m["models"].([]any); ok {
				n = len(models)
			}
			line += fmt.Sprintf("  models=%d", n)
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func looksLikeAccountRefID(id string) bool {
	// Defense for display: union model ids must not be vendor:account style.
	if !strings.Contains(id, ":") {
		return false
	}
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	switch strings.ToLower(parts[0]) {
	case "claude", "codex", "openai", "deepseek", "kimi", "glm":
		return true
	default:
		return false
	}
}
