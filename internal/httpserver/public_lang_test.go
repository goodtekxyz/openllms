package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLangFromAccept(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		{"", "ko"},
		{"en-US,en;q=0.9", "en"},
		{"ko-KR,ko;q=0.9,en;q=0.8", "ko"},
		{"ja,en-US;q=0.9,en;q=0.8", "ja"},
		{"zh-CN,zh;q=0.9,en;q=0.8", "zh"},
		{"fr-FR,fr;q=0.9", "ko"},
		{"en;q=0.5,ja;q=0.9", "ja"},
	}
	for _, tc := range cases {
		if got := langFromAccept(tc.header); got != tc.want {
			t.Fatalf("Accept-Language %q: got %q want %q", tc.header, got, tc.want)
		}
	}
}

func TestPublicRootNegotiatesLanguage(t *testing.T) {
	s := &Server{}
	r := s.Router()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/en" {
		t.Fatalf("en negotiate: %d %s", rec.Code, rec.Header().Get("Location"))
	}

	req = httptest.NewRequest(http.MethodGet, "/install", nil)
	req.Header.Set("Accept-Language", "ja")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/ja/install" {
		t.Fatalf("ja install negotiate: %d %s", rec.Code, rec.Header().Get("Location"))
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: publicLangCookie, Value: "zh"})
	req.Header.Set("Accept-Language", "en") // cookie wins
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/zh" {
		t.Fatalf("cookie wins: %d %s", rec.Code, rec.Header().Get("Location"))
	}

	req = httptest.NewRequest(http.MethodGet, "/en", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("/en status %d", rec.Code)
	}
	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == publicLangCookie && c.Value == "en" {
			found = true
		}
	}
	if !found {
		t.Fatal("/en should set llms_lang=en cookie")
	}

	// Explicit Korean switch must override an existing en cookie (lang menu uses /ko).
	req = httptest.NewRequest(http.MethodGet, "/ko", nil)
	req.AddCookie(&http.Cookie{Name: publicLangCookie, Value: "en"})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/" {
		t.Fatalf("/ko with en cookie: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	koCookie := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == publicLangCookie && c.Value == "ko" {
			koCookie = true
		}
	}
	if !koCookie {
		t.Fatal("/ko should set llms_lang=ko cookie")
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: publicLangCookie, Value: "ko"})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "lang=\"ko\"") {
		t.Fatalf("ko cookie on /: %d", rec.Code)
	}
}
