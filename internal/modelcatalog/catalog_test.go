package modelcatalog_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/goodtekxyz/openllms/internal/modelcatalog"
	"github.com/goodtekxyz/openllms/internal/secrets/memory"
	"github.com/goodtekxyz/openllms/internal/store"
	"github.com/google/uuid"
)

func TestOpenAIListNeverUsesAccountRefs(t *testing.T) {
	sec := memory.New()
	path := "/llms/p/accounts/deepseek/a"
	_ = sec.Put(context.Background(), path, "credential", `{"api_key":"sk"}`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" && r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "deepseek-chat", "owned_by": "deepseek"},
				{"id": "codex:should-skip", "owned_by": "x"},
			},
		})
	}))
	defer srv.Close()

	d := modelcatalog.New(sec, srv.Client(), time.Minute)
	acc := store.Account{
		ID: uuid.New(), Vendor: "deepseek", Name: "work", AuthType: "api_key",
		InfisicalPath: path, BaseURL: srv.URL + "/v1",
	}
	rt := &store.Route{Slug: "demo", Strategy: "sequential"}
	cat := d.ForRoute(context.Background(), rt, []store.Account{acc})
	list := modelcatalog.OpenAIList(cat)
	data := list["data"].([]map[string]any)
	if len(data) != 1 || data[0]["id"] != "deepseek-chat" {
		t.Fatalf("data=%v", data)
	}
	if cat.SuggestedModel != "deepseek-chat" {
		t.Fatalf("suggested=%q", cat.SuggestedModel)
	}
}

func TestCodexOAuthUsesKnownList(t *testing.T) {
	sec := memory.New()
	path := "/llms/p/accounts/codex/a"
	_ = sec.Put(context.Background(), path, "credential", `{"access_token":"tok","chatgpt_account_id":"acc"}`)

	d := modelcatalog.New(sec, http.DefaultClient, time.Minute)
	acc := store.Account{
		ID: uuid.New(), Vendor: "codex", Name: "codex-goodtek", AuthType: "oauth",
		InfisicalPath: path,
	}
	rt := &store.Route{Slug: "codex-quota-first", Strategy: "quota_aware"}
	cat := d.ForRoute(context.Background(), rt, []store.Account{acc})
	if len(cat.Accounts) != 1 || cat.Accounts[0].Status != "ok" {
		t.Fatalf("account=%+v", cat.Accounts)
	}
	if cat.Accounts[0].ID != "codex:codex-goodtek" {
		t.Fatalf("account id should remain ref: %s", cat.Accounts[0].ID)
	}
	list := modelcatalog.OpenAIList(cat)
	data := list["data"].([]map[string]any)
	if len(data) == 0 {
		t.Fatal("expected models")
	}
	for _, row := range data {
		id, _ := row["id"].(string)
		if id == "codex:codex-goodtek" || id == "" {
			t.Fatalf("bad model id %q", id)
		}
	}
	if cat.SuggestedModel != "gpt-5.5" {
		t.Fatalf("suggested=%q", cat.SuggestedModel)
	}
}

func TestDiscoveryErrorNoAccountAsModel(t *testing.T) {
	sec := memory.New()
	// no secret put → error, no fallback for deepseek
	d := modelcatalog.New(sec, http.DefaultClient, time.Minute)
	acc := store.Account{
		ID: uuid.New(), Vendor: "deepseek", Name: "x", AuthType: "api_key",
		InfisicalPath: "/missing", BaseURL: "http://127.0.0.1:1",
	}
	rt := &store.Route{Slug: "r", Strategy: "sequential"}
	cat := d.ForRoute(context.Background(), rt, []store.Account{acc})
	if cat.Accounts[0].Status != "error" {
		t.Fatalf("status=%s", cat.Accounts[0].Status)
	}
	list := modelcatalog.OpenAIList(cat)
	data := list["data"].([]map[string]any)
	if len(data) != 0 {
		t.Fatalf("expected empty openai list, got %v", data)
	}
}
