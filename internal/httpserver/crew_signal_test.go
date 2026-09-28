package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCrewSignalPublicGet(t *testing.T) {
	s := &Server{}
	r := s.Router()
	req := httptest.NewRequest(http.MethodGet, "/billing/api/crew-signal", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["goodtek_url"] != "https://goodtek.xyz/" {
		t.Fatalf("goodtek_url: %v", out["goodtek_url"])
	}
	if out["vibecrew_url"] != "https://vibecrew.jp/" {
		t.Fatalf("vibecrew_url: %v", out["vibecrew_url"])
	}
	if _, ok := out["week_count"]; !ok {
		t.Fatal("missing week_count")
	}
}

func TestSoftCapHintMentionsCrewSignal(t *testing.T) {
	h := softCapHint()
	if !strings.Contains(h, "cloud") {
		t.Fatalf("softCapHint=%q", h)
	}
}
