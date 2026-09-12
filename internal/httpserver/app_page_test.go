package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBillingPageRetired(t *testing.T) {
	s := &Server{}
	r := s.Router()
	for _, path := range []string{"/billing", "/en/billing", "/ja/billing", "/zh/billing"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s want 404 got %d", path, rec.Code)
		}
	}
}
