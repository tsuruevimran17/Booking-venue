package service

import (
	"testing"
	"time"

	"reservation/internal/dto"
)

func TestCheckScheduleMatch(t *testing.T) {
	svc := &bookingService{}

	startAt := time.Date(2024, time.May, 10, 10, 0, 0, 0, time.UTC)
	endAt := time.Date(2024, time.May, 10, 12, 0, 0, 0, time.UTC)

	start := "09:00"
	end := "18:00"
	day := dto.DayScheduleDTO{
		Enabled:   true,
		StartTime: &start,
		EndTime:   &end,
	}

	if err := svc.checkScheduleMatch(day, startAt, endAt); err != nil {
		t.Fatalf("expected schedule match, got error: %v", err)
	}
}

func TestCheckScheduleMatch_DisabledDay(t *testing.T) {
	svc := &bookingService{}

	startAt := time.Date(2024, time.May, 10, 10, 0, 0, 0, time.UTC)
	endAt := time.Date(2024, time.May, 10, 12, 0, 0, 0, time.UTC)

	day := dto.DayScheduleDTO{
		Enabled: false,
	}

	if err := svc.checkScheduleMatch(day, startAt, endAt); err == nil {
		t.Fatal("expected error for disabled day")
	}
}
