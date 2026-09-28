package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatUserSection(t *testing.T) {
	got := formatUserSection("goodtekxyz", "https://llms.goodtek.xyz", false)
	if !strings.Contains(got, "USER\n") || !strings.Contains(got, "goodtekxyz") || !strings.Contains(got, "https://llms.goodtek.xyz") {
		t.Fatalf("unexpected: %q", got)
	}
	got = formatUserSection("", "http://127.0.0.1:8080", true)
	if !strings.Contains(got, "LLMS_API_KEY from env") {
		t.Fatalf("expected env hint: %q", got)
	}
}

func TestClearCreds(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cfg := filepath.Join(dir, ".config", "llms")
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	cred := filepath.Join(cfg, "credentials.json")
	if err := os.WriteFile(cred, []byte(`{"login":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := clearCreds(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cred); !os.IsNotExist(err) {
		t.Fatalf("credentials still present: %v", err)
	}
	if err := clearCreds(); err != nil {
		t.Fatalf("second clear should be ok: %v", err)
	}
}

func TestResolveLoginAPIKey(t *testing.T) {
	key, unchanged, err := resolveLoginAPIKey(map[string]any{
		"api_key": "sk-gt-new", "login": "ops",
	}, "sk-gt-old")
	if err != nil || key != "sk-gt-new" || unchanged {
		t.Fatalf("new key: %q unchanged=%v err=%v", key, unchanged, err)
	}

	key, unchanged, err = resolveLoginAPIKey(map[string]any{
		"api_key_unchanged": true, "login": "ops",
	}, "sk-gt-old")
	if err != nil || key != "sk-gt-old" || !unchanged {
		t.Fatalf("keep local: %q unchanged=%v err=%v", key, unchanged, err)
	}

	_, _, err = resolveLoginAPIKey(map[string]any{
		"api_key_unchanged": true, "login": "ops",
	}, "")
	if err == nil {
		t.Fatal("expected error when no gateway key and no local key")
	}
}
