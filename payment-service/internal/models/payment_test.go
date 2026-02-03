package models

import "testing"

func TestIsValidPaymentMethod(t *testing.T) {
	if !IsValidPaymentMethod(MethodCard) {
		t.Fatal("expected card method to be valid")
	}
	if IsValidPaymentMethod(PaymentMethod("crypto")) {
		t.Fatal("expected crypto method to be invalid")
	}
}
