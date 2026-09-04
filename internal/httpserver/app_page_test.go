package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAppPagesInjectChrome(t *testing.T) {
	s := &Server{}
	r := s.Router()

	cases := []struct {
		path string
		want []string
	}{
		{
			"/console",
			[]string{
				"site-footer", "footer-cols", "data-theme-toggle", "theme-icon-system",
				"/assets/theme-boot.js", "/assets/app-shell.css", "nav-account",
				"aria-current=\"page\"", "콘솔</a>", "셀프호스트</a>", "시작하기</a>",
				"홈</a>", "API</a>", "lang-dd",
				"테마: 시스템", "GitHub로 로그인", "lang=\"ko\"",
			},
		},
		{
			"/billing",
			[]string{
				"site-footer", "/assets/billing.css", "/assets/app-shell.css",
				"id=\"btn-signin\"", "셀프호스트</a>", "콘솔</a>", "시작하기</a>",
				"홈</a>", "API</a>",
				"GitHub로 로그인", "플랜 상태", "테마: 시스템", "한국어",
				"Claude·Codex 구독과 API 키를",
			},
		},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("%s %d %s", tc.path, rec.Code, rec.Header().Get("Content-Type"))
		}
		body := rec.Body.String()
		if strings.Contains(body, "<!--LLMS_APP_HEADER-->") || strings.Contains(body, "<!--LLMS_APP_FOOTER-->") {
			t.Fatalf("%s markers not replaced", tc.path)
		}
		for _, want := range tc.want {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing %q", tc.path, want)
			}
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/console", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if strings.Count(rec.Body.String(), "aria-current=\"page\"") != 1 {
		t.Fatal("console should have exactly one aria-current=page")
	}

	req = httptest.NewRequest(http.MethodGet, "/billing", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	billingBody := rec.Body.String()
	if strings.Count(billingBody, "aria-current=\"page\"") != 0 {
		t.Fatal("billing page has no public nav item; should not mark aria-current=page")
	}
	if strings.Contains(billingBody, "app-banner warn") {
		t.Fatal("billing should not use yellow warn banner for login hint")
	}
	if strings.Contains(billingBody, ">요금</a>") {
		t.Fatal("public chrome must not advertise billing nav")
	}
}

func TestAppAssetsServe(t *testing.T) {
	s := &Server{}
	r := s.Router()
	for _, path := range []string{
		"/assets/theme-boot.js",
		"/assets/app-shell.css",
		"/assets/site-chrome.js",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("%s %d", path, rec.Code)
		}
	}
}
