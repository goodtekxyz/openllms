package router_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/goodtekxyz/openllms/internal/router"
	"github.com/goodtekxyz/openllms/internal/store"
	"github.com/google/uuid"
)

func accounts(ids ...string) []store.Account {
	out := make([]store.Account, 0, len(ids))
	for i, id := range ids {
		out = append(out, store.Account{ID: uuid.MustParse(id), Weight: 1, Position: i})
	}
	return out
}

func TestSequentialUsesFirst(t *testing.T) {
	a := accounts(
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
	)
	sel := router.NewSelector()
	rid := uuid.New()
	got := sel.Pick(rid, router.Sequential, a)
	if got.ID != a[0].ID {
		t.Fatalf("want first")
	}
}

func TestRoundRobinRotates(t *testing.T) {
	a := accounts(
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
	)
	sel := router.NewSelector()
	rid := uuid.New()
	seen := map[uuid.UUID]int{}
	for i := 0; i < 20; i++ {
		got := sel.Pick(rid, router.RoundRobin, a)
		seen[got.ID]++
	}
	if seen[a[0].ID] == 0 || seen[a[1].ID] == 0 {
		t.Fatalf("expected both accounts used: %v", seen)
	}
	if abs(seen[a[0].ID]-seen[a[1].ID]) > 1 {
		t.Fatalf("unbalanced RR: %v", seen)
	}
}

func TestWeightedPrefersHeavy(t *testing.T) {
	a := []store.Account{
		{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Weight: 9},
		{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Weight: 1},
	}
	sel := router.NewSelector()
	rid := uuid.New()
	counts := map[uuid.UUID]int{}
	for i := 0; i < 1000; i++ {
		got := sel.Pick(rid, router.Weighted, a)
		counts[got.ID]++
	}
	if counts[a[0].ID] < 700 {
		t.Fatalf("expected heavy preferred: %v", counts)
	}
}

func TestPresets(t *testing.T) {
	st, _ := router.ApplyPreset("failover")
	if st != router.Sequential {
		t.Fatal(st)
	}
	st, cfg := router.ApplyPreset("prefer-primary")
	if st != router.Weighted || cfg["primary_weight"].(int) != 80 {
		t.Fatalf("%v %v", st, cfg)
	}
}

func TestRetryable(t *testing.T) {
	if !router.IsRetryableStatus(429) || !router.IsRetryableStatus(503) {
		t.Fatal("retryable")
	}
	if router.IsRetryableStatus(400) {
		t.Fatal("400 not retryable")
	}
}

func TestPreferAccount(t *testing.T) {
	a := accounts(
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
	)
	got := router.PreferAccount(a, a[1].ID)
	if got[0].ID != a[1].ID {
		t.Fatalf("%v", got)
	}
}

func TestExcludeAndPreferFresh(t *testing.T) {
	a := accounts(
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
	)
	tried := map[uuid.UUID]struct{}{a[0].ID: {}}
	left := router.Exclude(a, tried)
	if len(left) != 1 || left[0].ID != a[1].ID {
		t.Fatalf("%v", left)
	}
	sel := router.NewSelector()
	sel.NoteSoftFail(a[0].ID)
	ordered := sel.PreferFresh(a)
	if ordered[0].ID != a[1].ID {
		t.Fatalf("want fresh first, got %v", ordered)
	}
}

func TestCooldownRetryAfter(t *testing.T) {
	h := make(http.Header)
	h.Set("Retry-After", "120")
	d := router.CooldownFromHeaders(429, h, 0)
	if d != 120*time.Second {
		t.Fatalf("got %v", d)
	}
	d2 := router.CooldownForStatus(429, 0)
	if d2 != time.Second {
		t.Fatalf("backoff0=%v", d2)
	}
	d3 := router.CooldownForStatus(429, 3)
	if d3 != 8*time.Second {
		t.Fatalf("backoff3=%v", d3)
	}
}

func TestQuotaAwarePrefersHigher(t *testing.T) {
	low, high := 10.0, 80.0
	a := []store.Account{
		{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), QuotaRemainingPct: &low},
		{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), QuotaRemainingPct: &high},
	}
	sel := router.NewSelector()
	got := sel.Pick(uuid.New(), router.QuotaAware, a)
	if got.ID != a[1].ID {
		t.Fatalf("want high quota account")
	}
}

func TestQuotaFirstPreset(t *testing.T) {
	st, cfg := router.ApplyPreset("quota-first")
	if st != router.QuotaAware || cfg["preset"] != "quota-first" {
		t.Fatalf("%v %v", st, cfg)
	}
}

func TestPresetFromRoute(t *testing.T) {
	if got := router.PresetFromRoute("sequential", []byte(`{"preset":"failover"}`)); got != "failover" {
		t.Fatalf("from config: %s", got)
	}
	if got := router.PresetFromRoute("round_robin", nil); got != "balance" {
		t.Fatalf("from strategy: %s", got)
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
