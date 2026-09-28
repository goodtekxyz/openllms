package billing

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// FormatUnifiRequestID encodes project + plan for webhook round-trip.
func FormatUnifiRequestID(projectID uuid.UUID, plan Plan) string {
	return fmt.Sprintf("llms:%s:%s", projectID.String(), string(plan))
}

// ParseUnifiRequestID decodes requestId from checkout or webhook.
func ParseUnifiRequestID(raw string) (uuid.UUID, Plan, bool) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "llms:") {
		return uuid.Nil, PlanNone, false
	}
	parts := strings.SplitN(raw, ":", 3)
	if len(parts) < 3 {
		return uuid.Nil, PlanNone, false
	}
	pid, err := uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, PlanNone, false
	}
	plan := NormalizePlan(parts[2])
	if !plan.ValidPaid() {
		return uuid.Nil, PlanNone, false
	}
	return pid, plan, true
}
