package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicInstallRoutes(t *testing.T) {
	s := &Server{}
	r := s.Router()

	for _, path := range []string{"/install.md", "/docs/install", "/docs/install.md"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("%s status %d", path, rec.Code)
		}
		ct := rec.Header().Get("Content-Type")
		if !strings.Contains(ct, "markdown") {
			t.Fatalf("%s content-type %q", path, ct)
		}
		if !strings.Contains(rec.Body.String(), "llms.goodtek.xyz") {
			t.Fatalf("%s missing gateway mention", path)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/install.sh", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.HasPrefix(rec.Body.String(), "#!") {
		t.Fatalf("install.sh bad response %d", rec.Code)
	}
}

func TestPublicHTMLPagesI18n(t *testing.T) {
	s := &Server{}
	r := s.Router()

	cases := []struct {
		path string
		want []string
	}{
		{"/", []string{"lang=\"ko\"", "구독 여러 개", "chat/completions", "web_search", "generate_image", "id=\"api\"", "/console", "id=\"self-host\"", "openllms", "셀프호스트</a>", "/assets/landing.css", "/assets/theme.js", "data-theme-toggle", "lang-dd", "og.png", "id=\"faq\"", "VibeCrew"}},
		{"/en", []string{"lang=\"en\"", "Many accounts", "chat/completions", "web_search", "generate_image", "id=\"api\"", "lang-dd", "data-theme-toggle", "Self-host</a>", "Home</a>", "id=\"self-host\"", "openllms"}},
		{"/ja", []string{"lang=\"ja\"", "複数アカウント", "chat/completions", "web_search", "id=\"api\"", "lang-dd", "data-theme-toggle", "コンソール</a>", "id=\"self-host\""}},
		{"/zh", []string{"lang=\"zh\"", "多个账户", "chat/completions", "web_search", "id=\"api\"", "lang-dd", "data-theme-toggle", "控制台</a>", "id=\"self-host\""}},
		{"/install", []string{"lang=\"ko\"", "llms login", "openllms", "web_search", "lang-dd", "data-theme-toggle", "/assets/theme.js", "/assets/copy.js", "시작하기</a>", "홈</a>"}},
		{"/en/install", []string{"lang=\"en\"", "llms login", "Get started</a>", "Home</a>", "lang-dd", "/assets/copy.js", "openllms"}},
		{"/ja/install", []string{"lang=\"ja\"", "llms login", "lang-dd", "/assets/copy.js", "はじめる</a>", "openllms"}},
		{"/zh/install", []string{"lang=\"zh\"", "llms login", "lang-dd", "/assets/copy.js", "开始使用</a>", "openllms"}},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("%s %d %s", tc.path, rec.Code, rec.Header().Get("Content-Type"))
		}
		body := rec.Body.String()
		for _, want := range tc.want {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing %q", tc.path, want)
			}
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/ko", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/" {
		t.Fatalf("/ko redirect got %d %s", rec.Code, rec.Header().Get("Location"))
	}

	req = httptest.NewRequest(http.MethodGet, "/assets/tokens.css", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("tokens.css %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	tokens := rec.Body.String()
	for _, want := range []string{"html.dark", "--footer-bg"} {
		if !strings.Contains(tokens, want) {
			t.Fatalf("tokens.css missing %q", want)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/assets/chrome.css", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("chrome.css %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	chrome := rec.Body.String()
	for _, want := range []string{".theme-toggle", ".hide-sm", ".site-footer", ".nav-account", "var(--chrome-max)"} {
		if !strings.Contains(chrome, want) {
			t.Fatalf("chrome.css missing %q", want)
		}
	}
	if strings.Contains(chrome, "var(--max)") {
		t.Fatal("chrome.css must not use --max (page content width); use --chrome-max")
	}

	req = httptest.NewRequest(http.MethodGet, "/assets/landing.css", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("landing.css %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	css := rec.Body.String()
	for _, want := range []string{`@import url("tokens.css")`, `@import url("chrome.css")`, "overflow-x: auto", "min-width: 0", "width: max-content"} {
		if !strings.Contains(css, want) {
			t.Fatalf("landing.css missing %q", want)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/assets/theme.js", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("theme.js %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), `KEY = "theme"`) {
		t.Fatal("theme.js missing storage key")
	}

	req = httptest.NewRequest(http.MethodGet, "/assets/billing.css", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("billing.css %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), ".status-card") {
		t.Fatal("billing.css missing .status-card")
	}

	req = httptest.NewRequest(http.MethodGet, "/assets/app-shell.css", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("app-shell.css %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), ".app-page") {
		t.Fatal("app-shell.css missing .app-page")
	}

	req = httptest.NewRequest(http.MethodGet, "/billing", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("/billing %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	for _, want := range []string{"/assets/billing.css", "/assets/landing.css", "/assets/app-shell.css", "/assets/theme-boot.js", "/assets/theme.js", "data-theme-toggle", "site-footer", "GitHub로 로그인", "플랜 상태"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("/billing missing %q", want)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/xx", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("unknown lang want 404 got %d", rec.Code)
	}
}

func TestMetaIncludesInstallURLs(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/control/v1/meta", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("meta %d", rec.Code)
	}
	b := rec.Body.String()
	for _, want := range []string{
		"install_sh", "install_doc", "install_html", "landing", "admin", "console",
		"landing_i18n", "install_html_i18n", "robots", "sitemap", "llms_txt",
		"unified_api_doc", "api_surface", "chat/completions",
		"llms.goodtek.xyz/en", "llms.goodtek.xyz/ja", "llms.goodtek.xyz/zh",
		"cli_dist", "cli_dist_ready",
	} {
		if !strings.Contains(b, want) {
			t.Fatalf("meta missing %q: %s", want, b)
		}
	}
}

func TestPublicSEOFiles(t *testing.T) {
	s := &Server{}
	r := s.Router()
	cases := []struct {
		path string
		ct   string
		want string
	}{
		{"/robots.txt", "text/plain", "Sitemap:"},
		{"/sitemap.xml", "xml", "llms.goodtek.xyz/en"},
		{"/llms.txt", "text/plain", "Hosted multi-account"},
		{"/LLMS_API.md", "markdown", "chat/completions"},
		{"/assets/og.png", "image/png", ""},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("%s %d", tc.path, rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), tc.ct) {
			t.Fatalf("%s ct %s", tc.path, rec.Header().Get("Content-Type"))
		}
		if tc.want != "" && !strings.Contains(rec.Body.String(), tc.want) {
			t.Fatalf("%s missing %q", tc.path, tc.want)
		}
	}
}

func TestPublicFooterLinks(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		"site-footer",
		"footer-cols",
		"lang-dd",
		"class=\"chev\"",
		"https://goodtek.xyz/",
		"https://vibepulse.goodtek.xyz/",
		"https://vibecrew.kr/",
		">VibeCrew</a>",
		"data-theme-toggle",
		"https://goodtek.xyz/privacy",
		"mailto:hello@goodtek.xyz",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("footer missing %q", want)
		}
	}
	if strings.Contains(body, "AI Vibe Crew") {
		t.Fatal("footer still says AI Vibe Crew")
	}
	if strings.Contains(body, "vibecrew.ai") {
		t.Fatal("footer still links vibecrew.ai")
	}
	if strings.Contains(body, "footer-dot") {
		t.Fatal("footer still uses glow-like footer-dot")
	}
	if strings.Contains(body, "llms status -w") {
		t.Fatal("landing still claims unimplemented status -w")
	}
	if strings.Contains(body, `class="code">KO`) {
		t.Fatal("lang dropdown still uses dense code badges")
	}
	if strings.Contains(body, "lang-dd") && strings.Contains(body, `summary style=`) {
		t.Fatal("lang dropdown still uses inline styles")
	}
}
