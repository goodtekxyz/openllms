package httpserver

import (
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"regexp"
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
		{"/", []string{"lang=\"ko\"", "이미 내고있는 구독비용", "API를 만드세요", "chat/completions", "web_search", "generate_image", "id=\"api\"", "/install", "id=\"self-host\"", "openllms", "셀프 호스티드</a>", "/assets/landing.css", "/assets/theme.js", "settings-fab", "data-theme-set", "og.png", "id=\"faq\"", "VibeCrew", "id=\"routing\"", "quota-first", "fill-first", "reset-soon", "id=\"honesty\"", "steward", "goodtek"}},
		{"/en", []string{"lang=\"en\"", "already pay", "chat/completions", "web_search", "generate_image", "id=\"api\"", "settings-fab", "data-theme-set", "Self-Hosted</a>", "Home</a>", "id=\"self-host\"", "openllms", "id=\"routing\"", "failover", "steward", "goodtek"}},
		{"/ja", []string{"lang=\"ja\"", "複数", "chat/completions", "web_search", "id=\"api\"", "settings-fab", "data-theme-set", "クラウド</a>", "id=\"self-host\"", "id=\"routing\"", "failover"}},
		{"/zh", []string{"lang=\"zh\"", "已经在付的订阅", "chat/completions", "web_search", "id=\"api\"", "settings-fab", "data-theme-set", "云端</a>", "id=\"self-host\"", "id=\"routing\"", "failover"}},
		{"/install", []string{"lang=\"ko\"", "llms login", "openllms", "web_search", "settings-fab", "data-theme-set", "/assets/theme.js", "/assets/copy.js", "시작하기</a>", "홈</a>"}},
		{"/en/install", []string{"lang=\"en\"", "llms login", "Get started</a>", "Home</a>", "settings-fab", "/assets/copy.js", "openllms"}},
		{"/ja/install", []string{"lang=\"ja\"", "llms login", "settings-fab", "/assets/copy.js", "はじめる</a>", "openllms"}},
		{"/zh/install", []string{"lang=\"zh\"", "llms login", "settings-fab", "/assets/copy.js", "开始使用</a>", "openllms"}},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("%s %d %s", tc.path, rec.Code, rec.Header().Get("Content-Type"))
		}
		body := rec.Body.String()
		if strings.Contains(body, `id="nav-login"`) {
			t.Fatalf("%s still shows public login CTA during soft-open", tc.path)
		}
		for _, want := range tc.want {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing %q", tc.path, want)
			}
		}
	}

	for _, path := range []string{"/login", "/en/login", "/ja/login", "/zh/login"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("%s want 302 soft-open redirect, got %d", path, rec.Code)
		}
		loc := rec.Header().Get("Location")
		if !strings.Contains(loc, "install") {
			t.Fatalf("%s Location=%q want install", path, loc)
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
	for _, want := range []string{"html.dark", "--footer-bg", "word-break: keep-all", "html:lang(ko)"} {
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
	for _, want := range []string{".settings-fab", ".hide-sm", ".site-footer", ".nav-account", ".nav-login", "var(--chrome-max)"} {
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
	if !strings.Contains(rec.Body.String(), `KEY = "theme"`) || !strings.Contains(rec.Body.String(), "data-theme-set") {
		t.Fatal("theme.js missing storage key or data-theme-set bind")
	}

	req = httptest.NewRequest(http.MethodGet, "/assets/theme-boot.js", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("theme-boot.js %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	boot := rec.Body.String()
	for _, want := range []string{`localStorage.getItem("theme")`, `localStorage.getItem("llms_lang")`, "location.replace"} {
		if !strings.Contains(boot, want) {
			t.Fatalf("theme-boot.js missing %q", want)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/assets/site-chrome.js", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("site-chrome.js %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	chromeJS := rec.Body.String()
	for _, want := range []string{`LANG_KEY = "llms_lang"`, "localStorage.setItem", "lang-seg"} {
		if !strings.Contains(chromeJS, want) {
			t.Fatalf("site-chrome.js missing %q", want)
		}
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
	if rec.Code != http.StatusNotFound {
		t.Fatalf("/billing want 404 got %d", rec.Code)
	}
	for _, path := range []string{"/en/billing", "/ja/billing", "/zh/billing", "/ko/billing"} {
		req = httptest.NewRequest(http.MethodGet, path, nil)
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s want 404 got %d", path, rec.Code)
		}
	}
	// Landing must not advertise the retired billing HTML page.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if strings.Contains(rec.Body.String(), `href="/billing"`) || strings.Contains(rec.Body.String(), "/billing#") {
		t.Fatal("landing must not link to /billing")
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
		"llms.goodtek.xyz/install", // console + install_html (web console retired)
	} {
		if !strings.Contains(b, want) {
			t.Fatalf("meta missing %q: %s", want, b)
		}
	}
	if strings.Contains(b, `"console":"https://llms.goodtek.xyz/console"`) {
		t.Fatalf("meta console still points at retired /console: %s", b)
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

func TestSitemapHEAD(t *testing.T) {
	s := &Server{}
	r := s.Router()
	for _, path := range []string{"/sitemap.xml", "/robots.txt", "/"} {
		req := httptest.NewRequest(http.MethodHead, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("HEAD %s %d", path, rec.Code)
		}
		if len(rec.Body.Bytes()) != 0 {
			t.Fatalf("HEAD %s should have empty body, got %d bytes", path, rec.Body.Len())
		}
	}
	req := httptest.NewRequest(http.MethodHead, "/sitemap.xml", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if !strings.Contains(rec.Header().Get("Content-Type"), "xml") {
		t.Fatalf("HEAD sitemap ct %s", rec.Header().Get("Content-Type"))
	}
	get := httptest.NewRecorder()
	r.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	if strings.Count(get.Body.String(), "<loc>") != 15 {
		t.Fatalf("GET sitemap loc count %d", strings.Count(get.Body.String(), "<loc>"))
	}
}

func TestPublicLandingDiscovery(t *testing.T) {
	s := &Server{}
	r := s.Router()
	for _, path := range []string{"/", "/en", "/ja", "/zh"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("%s %d", path, rec.Code)
		}
		body := rec.Body.String()
		for _, want := range []string{
			"FAQPage",
			"https://llms.goodtek.xyz/#app",
			"https://goodtek.xyz/#organization",
			`hreflang="x-default"`,
			"og:image",
			"index,follow",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing %q", path, want)
			}
		}
		assertFAQSchemaMatchesVisible(t, path, body)
	}
	for _, path := range []string{"/api", "/en/api", "/ja/api", "/zh/api"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("%s %d", path, rec.Code)
		}
		assertTwitterCardMatchesOG(t, path, rec.Body.String())
	}
	req := httptest.NewRequest(http.MethodGet, "/en/install", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `hreflang="ja"`) || !strings.Contains(rec.Body.String(), "og:image") {
		t.Fatal("en install missing hreflang or og:image")
	}
	req = httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	site := rec.Body.String()
	for _, want := range []string{"llms.goodtek.xyz/en/api", "llms.goodtek.xyz/LLMS_API.md"} {
		if !strings.Contains(site, want) {
			t.Fatalf("sitemap missing %q", want)
		}
	}
	req = httptest.NewRequest(http.MethodGet, "/llms.txt", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "What is llms?") {
		t.Fatal("llms.txt missing answer heading")
	}
}

var (
	faqItemRE = regexp.MustCompile(`(?s)<details class="faq-item">\s*<summary>(.*?)</summary>\s*<p>(.*?)</p>`)
	htmlTagRE = regexp.MustCompile(`<[^>]+>`)
	metaRE    = regexp.MustCompile(`<meta[^>]+>`)
	spaceRE   = regexp.MustCompile(`\s+`)
)

func visibleText(raw string) string {
	return strings.TrimSpace(spaceRE.ReplaceAllString(html.UnescapeString(htmlTagRE.ReplaceAllString(raw, "")), " "))
}

func metaAttr(tag, key string) string {
	re := regexp.MustCompile(regexp.QuoteMeta(key) + `="([^"]*)"`)
	m := re.FindStringSubmatch(tag)
	if m == nil {
		return ""
	}
	return m[1]
}

func metaContent(body, attr, name string) string {
	for _, tag := range metaRE.FindAllString(body, -1) {
		if metaAttr(tag, attr) == name {
			return metaAttr(tag, "content")
		}
	}
	return ""
}

func assertTwitterCardMatchesOG(t *testing.T, path, body string) {
	t.Helper()
	if got := metaContent(body, "name", "twitter:card"); got != "summary_large_image" {
		t.Fatalf("%s twitter:card %q", path, got)
	}
	for _, pair := range [][2]string{
		{"twitter:title", "og:title"},
		{"twitter:description", "og:description"},
		{"twitter:image", "og:image"},
	} {
		tw := metaContent(body, "name", pair[0])
		og := metaContent(body, "property", pair[1])
		if tw == "" || tw != og {
			t.Fatalf("%s %s %q != %s %q", path, pair[0], tw, pair[1], og)
		}
	}
}

func assertFAQSchemaMatchesVisible(t *testing.T, path, body string) {
	t.Helper()
	visible := map[string]string{}
	for _, m := range faqItemRE.FindAllStringSubmatch(body, -1) {
		visible[visibleText(m[1])] = visibleText(m[2])
	}
	if len(visible) == 0 {
		t.Fatalf("%s has no visible FAQ", path)
	}
	start := strings.Index(body, `<script type="application/ld+json">`)
	if start < 0 {
		t.Fatalf("%s missing JSON-LD", path)
	}
	rest := body[start+len(`<script type="application/ld+json">`):]
	end := strings.Index(rest, "</script>")
	if end < 0 {
		t.Fatalf("%s missing JSON-LD close", path)
	}
	raw := rest[:end]
	var doc struct {
		Graph []map[string]any `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("%s JSON-LD: %v", path, err)
	}
	var questions []any
	for _, node := range doc.Graph {
		if node["@type"] == "FAQPage" {
			questions, _ = node["mainEntity"].([]any)
		}
	}
	if len(questions) != len(visible) {
		t.Fatalf("%s FAQ count schema %d visible %d", path, len(questions), len(visible))
	}
	for _, q := range questions {
		item, _ := q.(map[string]any)
		name, _ := item["name"].(string)
		answer, _ := item["acceptedAnswer"].(map[string]any)
		text, _ := answer["text"].(string)
		want, ok := visible[name]
		if !ok {
			t.Fatalf("%s schema question %q is not in the visible FAQ", path, name)
		}
		if text != want {
			t.Fatalf("%s FAQ %q\nschema:  %q\nvisible: %q", path, name, text, want)
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
		"settings-fab",
		"data-settings-toggle",
		"data-theme-set",
		"lang-seg",
		"https://goodtek.xyz/",
		"https://vibepulse.goodtek.xyz/",
		"https://vibecrew.kr/",
		">VibeCrew</a>",
		"https://goodtek.xyz/privacy",
		"mailto:hello@goodtek.xyz",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("footer missing %q", want)
		}
	}
	if strings.Contains(body, `id="nav-login"`) {
		t.Fatal("public chrome still shows Login CTA during soft-open")
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
	if strings.Contains(body, "lang-dd") {
		t.Fatal("header still uses lang-dd dropdown")
	}
	if strings.Contains(body, "data-theme-toggle") {
		t.Fatal("header still uses data-theme-toggle")
	}
}
