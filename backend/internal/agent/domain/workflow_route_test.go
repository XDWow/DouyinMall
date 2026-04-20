package domain

import "testing"

func TestDefaultReadOnlyForIntent(t *testing.T) {
	if !DefaultReadOnlyForIntent(IntentOrderService) {
		t.Fatal("order_service should be read-only")
	}
	if DefaultReadOnlyForIntent(IntentAddToCart) {
		t.Fatal("add_to_cart should allow writes")
	}
	if !DefaultReadOnlyForIntent(IntentUnknown) {
		t.Fatal("unknown intent should be read-only")
	}
}
