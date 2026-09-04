package billing

import (
	"testing"
	"time"
)

func TestEntitledTrial(t *testing.T) {
	end := time.Now().UTC().Add(24 * time.Hour)
	e := Entitlement{Plan: PlanTrial, Status: "trialing", TrialEndsAt: &end}
	if !e.Entitled(time.Now().UTC()) {
		t.Fatal("expected entitled during trial")
	}
	if e.EffectivePlan(time.Now().UTC()) != PlanTrial {
		t.Fatal("expected trial plan")
	}
}

func TestEntitledExpired(t *testing.T) {
	end := time.Now().UTC().Add(-time.Hour)
	e := Entitlement{Plan: PlanStarter, Status: "active", CurrentPeriodEnd: &end}
	if e.Entitled(time.Now().UTC()) {
		t.Fatal("expected not entitled after period")
	}
}

func TestPlanLimits(t *testing.T) {
	if PlanStarter.Limits().Accounts != 5 || PlanPro.PriceUSD() != 9 {
		t.Fatalf("unexpected limits/price: %+v %v", PlanStarter.Limits(), PlanPro.PriceUSD())
	}
}
