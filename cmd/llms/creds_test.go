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
