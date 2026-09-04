package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goodtekxyz/openllms/internal/userauth"
)

func TestConsolePageServes(t *testing.T) {
	s := &Server{user: &userauth.Manager{}}
	req := httptest.NewRequest(http.MethodGet, "/console", nil)
	rec := httptest.NewRecorder()
	s.handleConsolePage(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "LLM 연결") {
		t.Fatalf("console page: %d %s", rec.Code, rec.Body.String())
	}
}

func TestConsoleOverviewUnauthorized(t *testing.T) {
	s := &Server{
		user:  &userauth.Manager{},
		admin: disabledAdmin{},
	}
	req := httptest.NewRequest(http.MethodGet, "/console/api/overview", nil)
	rec := httptest.NewRecorder()
	s.handleConsoleOverview(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}
