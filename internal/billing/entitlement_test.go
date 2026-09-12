package billing

import (
	"testing"
	"time"
)

func TestEntitledFreeForever(t *testing.T) {
	e := Entitlement{Plan: PlanFree, Status: "active"}
	if !e.Entitled(time.Now().UTC()) {
		t.Fatal("expected free always entitled")
	}
	if e.EffectivePlan(time.Now().UTC()) != PlanFree {
		t.Fatal("expected free plan")
	}
}

func TestEntitledLegacyTrialAsFree(t *testing.T) {
	past := time.Now().UTC().Add(-24 * time.Hour)
	e := Entitlement{Plan: PlanTrial, Status: "trialing", TrialEndsAt: &past, CurrentPeriodEnd: &past}
	// Canonical trial → free; free ignores period end.
	if !e.Entitled(time.Now().UTC()) {
		t.Fatal("expected legacy trial row entitled as free")
	}
	if e.EffectivePlan(time.Now().UTC()) != PlanFree {
		t.Fatal("expected free effective plan")
	}
}

func TestEntitledExpiredPaid(t *testing.T) {
	end := time.Now().UTC().Add(-time.Hour)
	e := Entitlement{Plan: PlanStarter, Status: "active", CurrentPeriodEnd: &end}
	if e.Entitled(time.Now().UTC()) {
		t.Fatal("expected not entitled after period")
	}
}

func TestPlanLimits(t *testing.T) {
	free := PlanFree.Limits()
	if free.Accounts != 1 || free.Routes != 1 || free.Keys != 1 || free.SoftCap != 5_000_000 {
		t.Fatalf("unexpected free limits: %+v", free)
	}
	starter := PlanStarter.Limits()
	if starter.Accounts != 3 || starter.Routes != 3 || starter.Keys != 3 || starter.SoftCap != 0 || starter.RPM != 0 {
		t.Fatalf("unexpected starter limits: %+v", starter)
	}
	if free.RPM != 30 {
		t.Fatalf("expected free RPM 30, got %d", free.RPM)
	}
	pro := PlanPro.Limits()
	if pro.Accounts != 0 || pro.Routes != 0 || pro.Keys != 0 || pro.SoftCap != 0 || PlanPro.PriceUSD() != 9 {
		t.Fatalf("unexpected pro limits/price: %+v %v", pro, PlanPro.PriceUSD())
	}
	if PlanTrial.Canonical() != PlanFree {
		t.Fatal("trial should canonicalize to free")
	}
}
