package extract

import (
	"testing"
	"time"
)

func TestClampedDate(t *testing.T) {
	tests := []struct {
		name    string
		year    int
		month   time.Month
		day     int
		wantDay int
	}{
		{"jan 31 stays 31", 2026, time.January, 31, 31},
		{"feb 31 clamps to 28", 2026, time.February, 31, 28},
		{"feb 29 in leap year", 2024, time.February, 29, 29},
		{"feb 29 in non-leap clamps to 28", 2026, time.February, 29, 28},
		{"apr 31 clamps to 30", 2026, time.April, 31, 30},
		{"jun 30 stays 30", 2026, time.June, 30, 30},
		{"normal day", 2026, time.March, 15, 15},
		{"day 1", 2026, time.January, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampedDate(tt.year, tt.month, tt.day)
			if got.Day() != tt.wantDay {
				t.Errorf("clampedDate(%d, %v, %d).Day() = %d, want %d",
					tt.year, tt.month, tt.day, got.Day(), tt.wantDay)
			}
			if got.Month() != tt.month {
				t.Errorf("clampedDate(%d, %v, %d).Month() = %v, want %v",
					tt.year, tt.month, tt.day, got.Month(), tt.month)
			}
		})
	}
}

func TestBuildPlanAnalysis(t *testing.T) {
	end := "2026-02-28"
	planHistory := []PlanConfig{
		{
			Plan:       "Max",
			Start:      "2026-01-01",
			End:        &end,
			CostUSD:    93.00,
			BillingDay: 1,
		},
	}
	dailyCosts := []map[string]any{
		{"date": "2026-01-15", "total": 50.0},
		{"date": "2026-02-10", "total": 30.0},
		{"date": "2026-03-01", "total": 99.0}, // outside period
	}
	sessions := []SessionOutput{
		{Date: "2026-01-15", Messages: 100},
		{Date: "2026-02-10", Messages: 50},
		{Date: "2026-03-01", Messages: 999}, // outside period
	}

	result := BuildPlanAnalysis(planHistory, dailyCosts, sessions)

	// Monthly billing cycles: Jan 1–31 and Feb 1–28
	if len(result.Periods) != 2 {
		t.Fatalf("expected 2 periods, got %d", len(result.Periods))
	}

	// January period
	p0 := result.Periods[0]
	if p0.Start != "2026-01-01" || p0.End != "2026-01-31" {
		t.Errorf("period 0: expected 2026-01-01 to 2026-01-31, got %s to %s", p0.Start, p0.End)
	}
	if p0.APICost != 50.0 {
		t.Errorf("period 0: expected API cost 50.0, got %f", p0.APICost)
	}
	if p0.Sessions != 1 {
		t.Errorf("period 0: expected 1 session, got %d", p0.Sessions)
	}
	if p0.Messages != 100 {
		t.Errorf("period 0: expected 100 messages, got %d", p0.Messages)
	}

	// February period
	p1 := result.Periods[1]
	if p1.Start != "2026-02-01" || p1.End != "2026-02-28" {
		t.Errorf("period 1: expected 2026-02-01 to 2026-02-28, got %s to %s", p1.Start, p1.End)
	}
	if p1.APICost != 30.0 {
		t.Errorf("period 1: expected API cost 30.0, got %f", p1.APICost)
	}
	if p1.Sessions != 1 {
		t.Errorf("period 1: expected 1 session, got %d", p1.Sessions)
	}

	// Total across periods
	if result.TotalAPICost != 80.0 {
		t.Errorf("expected total API cost 80.0, got %f", result.TotalAPICost)
	}
}

func TestBuildPlanAnalysis_EmptyHistory(t *testing.T) {
	result := BuildPlanAnalysis(nil, nil, nil)
	if len(result.Periods) != 0 {
		t.Errorf("expected 0 periods, got %d", len(result.Periods))
	}
	if result.CurrentBilling != nil {
		t.Error("expected nil current billing with no plan history")
	}
}
