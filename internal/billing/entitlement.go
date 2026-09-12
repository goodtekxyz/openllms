package billing

import "time"

// Entitlement is the gateway's view of whether a project may use paid surfaces.
type Entitlement struct {
	Plan                 Plan
	Status               string
	TrialEndsAt          *time.Time
	CurrentPeriodStart   *time.Time
	CurrentPeriodEnd     *time.Time
	CancelAtPeriodEnd    bool
	BillingProvider      string
	ProviderCustomerID   string
	ProviderSubscription string
	TrialUsed            bool
}

func (e Entitlement) EffectivePlan(now time.Time) Plan {
	if !e.Entitled(now) {
		return PlanNone
	}
	p := e.Plan.Canonical()
	switch p {
	case PlanFree, PlanStarter, PlanPro:
		return p
	default:
		return PlanNone
	}
}

func (e Entitlement) Entitled(now time.Time) bool {
	p := e.Plan.Canonical()
	// Free is perpetual (no period clock). Legacy "trial" rows canonicalize to free.
	if p == PlanFree {
		return true
	}
	switch e.Status {
	case "active", "past_due", "trialing":
		if e.CurrentPeriodEnd == nil {
			return e.Status == "active" || e.Status == "trialing"
		}
		end := *e.CurrentPeriodEnd
		if e.BillingProvider == "unifi" && e.Status == "past_due" {
			end = end.Add(UnifiGrace)
		}
		return !now.After(end)
	default:
		return false
	}
}

func (e Entitlement) Limits(now time.Time) Limits {
	return e.EffectivePlan(now).Limits()
}
