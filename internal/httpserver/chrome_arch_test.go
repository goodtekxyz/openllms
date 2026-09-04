package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChromeArchitectureSingleSource(t *testing.T) {
	s := &Server{}
	r := s.Router()

	pages := []string{"/", "/install", "/console", "/billing"}
	for _, path := range pages {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("%s status %d", path, rec.Code)
		}
		body := rec.Body.String()
		if strings.Contains(body, "<!--LLMS_") {
			t.Fatalf("%s left chrome markers unreplaced", path)
		}
		for _, want := range []string{`class="nav"`, `id="nav-account"`, `data-theme-toggle`, `lang-dd`, `site-footer`} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing shared chrome %q", path, want)
			}
		}
	}

	tokens := fetchAsset(t, r, "/assets/tokens.css")
	for _, want := range []string{"--chrome-max:", "--content-max:", "--page-gutter:", "--measure:"} {
		if !strings.Contains(tokens, want) {
			t.Fatalf("tokens.css missing layout token %q", want)
		}
	}
	if !strings.Contains(tokens, "min-height: 100dvh") || !strings.Contains(tokens, "flex-direction: column") {
		t.Fatal("tokens.css body must be a column flex shell for sticky footer")
	}

	installCSS := fetchAsset(t, r, "/assets/install.css")
	if !strings.Contains(installCSS, `@import url("chrome.css")`) {
		t.Fatal("install.css must import chrome.css")
	}
	if strings.Contains(installCSS, "--max: 820px") {
		t.Fatal("install must not override --max; that shrinks the shared header")
	}
	if !strings.Contains(installCSS, "--content-max: 820px") {
		t.Fatal("install should narrow content via --content-max only")
	}
	for _, forbid := range []string{".btn {", ".btn-sm", ".btn-primary", ".nav-links a[aria-current"} {
		if strings.Contains(installCSS, forbid) {
			t.Fatalf("install.css must not redefine chrome rule %q", forbid)
		}
	}
	if !strings.Contains(installCSS, "var(--measure)") {
		t.Fatal("install content columns should use shared --measure")
	}

	landingCSS := fetchAsset(t, r, "/assets/landing.css")
	if !strings.Contains(landingCSS, `@import url("chrome.css")`) {
		t.Fatal("landing.css must import chrome.css")
	}
	if !strings.Contains(landingCSS, "var(--measure)") {
		t.Fatal("landing content columns should use shared --measure")
	}

	chrome := fetchAsset(t, r, "/assets/chrome.css")
	if !strings.Contains(chrome, ".hide-sm") {
		t.Fatal("chrome.css must define .hide-sm for shared header")
	}
	if !strings.Contains(chrome, "margin-top: auto") {
		t.Fatal("chrome footer should stick to bottom via margin-top: auto")
	}
	if !strings.Contains(chrome, "var(--measure)") {
		t.Fatal("chrome bands should use shared --measure")
	}
	if strings.Contains(chrome, "calc(100% - 2rem)") {
		t.Fatal("chrome should use --measure, not hardcoded 2rem gutters")
	}

	appShell := fetchAsset(t, r, "/assets/app-shell.css")
	if !strings.Contains(appShell, "var(--measure)") {
		t.Fatal("app-shell main should use shared --measure")
	}
	billing := fetchAsset(t, r, "/assets/billing.css")
	if strings.Contains(billing, ".billing-main") && strings.Contains(billing, "width:") {
		// billing-main must not redeclare the shared content width
		if strings.Contains(billing, ".billing-main {\n  width:") || strings.Contains(billing, ".billing-main {\r\n  width:") {
			t.Fatal("billing.css must not redeclare .billing-main width; use .app-main")
		}
	}
}

func fetchAsset(t *testing.T, r http.Handler, path string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("%s status %d", path, rec.Code)
	}
	return rec.Body.String()
}
