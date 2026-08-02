package extract

import (
	"testing"
	"time"
)

func TestParseWeeklyReset(t *testing.T) {
	tests := []struct {
		in     string
		wd     time.Weekday
		hh, mm int
		ok     bool
	}{
		{"Thu 10:00", time.Thursday, 10, 0, true},
		{"thursday 09:59", time.Thursday, 9, 59, true},
		{"MON 0:30", time.Monday, 0, 30, true},
		{"", 0, 0, 0, false},
		{"Thu", 0, 0, 0, false},
		{"Xyz 10:00", 0, 0, 0, false},
		{"Thu 24:00", 0, 0, 0, false},
		{"Thu 10:60", 0, 0, 0, false},
	}
	for _, tt := range tests {
		wd, hh, mm, ok := parseWeeklyReset(tt.in)
		if ok != tt.ok {
			t.Errorf("parseWeeklyReset(%q) ok = %v, want %v", tt.in, ok, tt.ok)
			continue
		}
		if ok && (wd != tt.wd || hh != tt.hh || mm != tt.mm) {
			t.Errorf("parseWeeklyReset(%q) = %v %d:%d, want %v %d:%d",
				tt.in, wd, hh, mm, tt.wd, tt.hh, tt.mm)
		}
	}
}

func TestLastWeeklyReset(t *testing.T) {
	// Sunday 2026-08-02 16:00 → most recent Thu 10:00 is 2026-07-30.
	now := time.Date(2026, 8, 2, 16, 0, 0, 0, time.UTC)
	got := lastWeeklyReset(now, time.Thursday, 10, 0)
	want := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("lastWeeklyReset = %v, want %v", got, want)
	}

	// Thursday 09:00, before the reset time → previous Thursday.
	now = time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	got = lastWeeklyReset(now, time.Thursday, 10, 0)
	want = time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("lastWeeklyReset before reset time = %v, want %v", got, want)
	}

	// Thursday 10:00 exactly → window starts now.
	now = time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	got = lastWeeklyReset(now, time.Thursday, 10, 0)
	if !got.Equal(now) {
		t.Errorf("lastWeeklyReset at reset instant = %v, want %v", got, now)
	}
}

func TestIsFableModel(t *testing.T) {
	if !isFableModel("claude-fable-5") || !isFableModel("claude-mythos-5") {
		t.Error("fable/mythos models should be detected")
	}
	if isFableModel("claude-opus-5") || isFableModel("claude-sonnet-5") {
		t.Error("opus/sonnet must not count toward the Fable weekly bucket")
	}
}
