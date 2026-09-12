package billing

import (
	"testing"

	"github.com/google/uuid"
)

func TestParseUnifiRequestID(t *testing.T) {
	pid := uuid.MustParse("b2770eb9-9205-4f07-973d-457f3f1cab33")
	raw := FormatUnifiRequestID(pid, PlanStarter)
	gotPID, gotPlan, ok := ParseUnifiRequestID(raw)
	if !ok || gotPID != pid || gotPlan != PlanStarter {
		t.Fatalf("parse %q: ok=%v pid=%v plan=%v", raw, ok, gotPID, gotPlan)
	}
}
