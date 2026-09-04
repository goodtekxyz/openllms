package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLanguageSwitchBackToKorean(t *testing.T) {
	s := &Server{}
	r := s.Router()

	// English cookie + Accept-Language en: "/" redirects to /en
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: publicLangCookie, Value: "en"})
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/en" {
		t.Fatalf("stale en cookie on /: got %d %s", rec.Code, rec.Header().Get("Location"))
	}

	// Switching via /ko must set cookie and land on Korean home
	req = httptest.NewRequest(http.MethodGet, "/ko", nil)
	req.AddCookie(&http.Cookie{Name: publicLangCookie, Value: "en"})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/" {
		t.Fatalf("/ko redirect: got %d %s", rec.Code, rec.Header().Get("Location"))
	}
	var setCookie string
	for _, c := range rec.Result().Cookies() {
		if c.Name == publicLangCookie {
			setCookie = c.Value
		}
	}
	if setCookie != "ko" {
		t.Fatalf("/ko did not set ko cookie, got %q", setCookie)
	}

	// After ko cookie, /install stays Korean even with English Accept-Language
	req = httptest.NewRequest(http.MethodGet, "/install", nil)
	req.AddCookie(&http.Cookie{Name: publicLangCookie, Value: "ko"})
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("/install with ko cookie status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `lang="ko"`) {
		t.Fatal("/install should stay Korean when cookie is ko")
	}

	// /ko/install sets cookie then redirects to /install
	req = httptest.NewRequest(http.MethodGet, "/ko/install", nil)
	req.AddCookie(&http.Cookie{Name: publicLangCookie, Value: "en"})
	req.Header.Set("Accept-Language", "en")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/install" {
		t.Fatalf("/ko/install redirect: got %d %s", rec.Code, rec.Header().Get("Location"))
	}
}

func TestSharedHeaderAcrossHomeAndConsole(t *testing.T) {
	s := &Server{}
	r := s.Router()

	home := fetchHTML(t, r, "/")
	console := fetchHTML(t, r, "/console")

	for _, link := range []string{">홈</a>", ">API</a>", ">시작하기</a>", ">셀프호스트</a>", ">콘솔</a>"} {
		if !strings.Contains(home, link) {
			t.Fatalf("home missing %s", link)
		}
		if !strings.Contains(console, link) {
			t.Fatalf("console missing %s", link)
		}
	}
	if !strings.Contains(home, `id="nav-account"`) || !strings.Contains(console, `id="nav-account"`) {
		t.Fatal("home and console must share nav-account slot")
	}
	if !strings.Contains(home, `href="/ko"`) {
		t.Fatal("home lang menu must link to /ko so cookie can switch back")
	}
	if !strings.Contains(home, `href="/ko/install"`) {
		t.Fatal("home start CTA/nav must use /ko/install")
	}
	if !strings.Contains(console, `href="/ko"`) {
		t.Fatal("console lang menu must link to /ko")
	}
}
