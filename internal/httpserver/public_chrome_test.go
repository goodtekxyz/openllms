package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicChromeInjection(t *testing.T) {
	s := &Server{}
	r := s.Router()

	cases := []struct {
		path       string
		activeWant string
		want       []string
		notWant    []string
	}{
		{
			"/",
			`aria-current="page"`,
			[]string{
				"lang=\"ko\"", "이미 내고있는 구독비용", "API를 만드세요", "chat/completions", "web_search",
				"generate_image", "id=\"api\"", "홈</a>", "API</a>", "시작하기</a>",
				"셀프 호스티드</a>", "클라우드</a>", "site-footer", "VibeCrew", "settings-fab", "data-theme-set",
				"id=\"self-host\"", "openllms", "id=\"routing\"", "quota-first", "goodtek",
			},
			[]string{"<!--LLMS_PUBLIC_HEADER-->", "<!--LLMS_PUBLIC_FOOTER-->", "기계용", "CLI 가이드", "id=\"pricing\""},
		},
		{
			"/en",
			`aria-current="page"`,
			[]string{
				"lang=\"en\"", "ChatGPT", "already pay", "chat/completions",
				"web_search", "Home</a>", "Get started</a>", "Self-Hosted</a>", "Self-Hosted</a>", "id=\"self-host\"",
				"id=\"routing\"", "quota-first", "goodtek",
			},
			[]string{"<!--LLMS_PUBLIC_HEADER-->", "id=\"pricing\""},
		},
		{
			"/install",
			`aria-current="page"`,
			[]string{
				"lang=\"ko\"", "llms login", "web_search", "generate_image",
				"홈</a>", "API</a>", "시작하기</a>", "aria-current=\"page\"",
			},
			[]string{"<!--LLMS_PUBLIC_HEADER-->", "기계용", "에이전트", "CLI 가이드"},
		},
		{
			"/en/install",
			"",
			[]string{"lang=\"en\"", "llms login", "Get started</a>", "Home</a>", "API</a>", "Self-Hosted</a>"},
			[]string{"<!--LLMS_PUBLIC_HEADER-->"},
		},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("%s status %d", tc.path, rec.Code)
		}
		body := rec.Body.String()
		for _, want := range tc.want {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing %q", tc.path, want)
			}
		}
		for _, not := range tc.notWant {
			if strings.Contains(body, not) {
				t.Fatalf("%s should not contain %q", tc.path, not)
			}
		}
		if tc.activeWant != "" && !strings.Contains(body, tc.activeWant) {
			t.Fatalf("%s missing active nav marker", tc.path)
		}
	}

	// install and landing share same nav link set
	homeBody := fetchHTML(t, r, "/")
	installBody := fetchHTML(t, r, "/install")
	for _, link := range []string{">홈</a>", ">API</a>", ">시작하기</a>", ">셀프 호스티드</a>"} {
		if !strings.Contains(homeBody, link) || !strings.Contains(installBody, link) {
			t.Fatalf("nav links mismatch for %s", link)
		}
	}
}

func fetchHTML(t *testing.T, r http.Handler, path string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("%s status %d", path, rec.Code)
	}
	return rec.Body.String()
}
