package billing

import "time"

type Plan string

const (
	PlanNone    Plan = "none"
	PlanTrial   Plan = "trial"
	PlanStarter Plan = "starter"
	PlanPro     Plan = "pro"
)

type Limits struct {
	Accounts int
	Routes   int
	Keys     int
	RPM      int
	SoftCap  int64 // tokens / month
}

var limits = map[Plan]Limits{
	PlanTrial:   {Accounts: 3, Routes: 2, Keys: 2, RPM: 40, SoftCap: 5_000_000},
	PlanStarter: {Accounts: 5, Routes: 3, Keys: 3, RPM: 60, SoftCap: 10_000_000},
	PlanPro:     {Accounts: 20, Routes: 15, Keys: 10, RPM: 120, SoftCap: 50_000_000},
}

func (p Plan) Limits() Limits {
	if l, ok := limits[p]; ok {
		return l
	}
	return Limits{} // none → zeros; gates require entitlement first
}

func (p Plan) PriceUSD() float64 {
	switch p {
	case PlanStarter:
		return 5
	case PlanPro:
		return 9
	default:
		return 0
	}
}

func (p Plan) ValidPaid() bool {
	return p == PlanStarter || p == PlanPro
}

const TrialDays = 7
const UnifiGrace = 3 * 24 * time.Hour

func NormalizePlan(s string) Plan {
	switch Plan(s) {
	case PlanTrial, PlanStarter, PlanPro, PlanNone:
		return Plan(s)
	default:
		return PlanNone
	}
}
