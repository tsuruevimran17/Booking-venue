package validation

import "testing"

func TestValidateTime(t *testing.T) {
	if _, err := ValidateTime("09:30"); err != nil {
		t.Fatalf("expected valid time, got error: %v", err)
	}
	if _, err := ValidateTime("25:00"); err == nil {
		t.Fatal("expected error for invalid hour")
	}
}

func TestCompareTimes(t *testing.T) {
	ok, err := CompareTimes("09:00", "18:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected start time to be before end time")
	}
}
