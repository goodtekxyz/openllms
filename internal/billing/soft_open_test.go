package billing

import "testing"

func TestSoftOpenFlags(t *testing.T) {
	if PublicPaidCheckout {
		t.Fatal("PublicPaidCheckout must be false during Free soft-open")
	}
	if !SoftOpen {
		t.Fatal("SoftOpen must be true when paid checkout is off")
	}
}
