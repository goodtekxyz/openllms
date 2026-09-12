//go:build cloud

package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goodtekxyz/openllms/internal/config"
	"github.com/google/uuid"
)

func TestCheckoutPausedSoftOpen(t *testing.T) {
	s := &Server{cfg: config.Config{PublicBaseURL: "https://llms.goodtek.xyz", BillingMock: true}}
	r := s.Router()
	pid := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/billing/api/checkout?project_id="+pid.String(), strings.NewReader(`{"plan":"starter","provider":"polar"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 checkout_paused, got %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != "checkout_paused" {
		t.Fatalf("error=%v", out["error"])
	}
	if out["soft_open"] != true {
		t.Fatalf("soft_open=%v", out["soft_open"])
	}
}
