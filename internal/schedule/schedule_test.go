package schedule

import (
	"testing"
	"time"
)

func TestSelectDeadlineWeekdays(t *testing.T) {
	location := mustLoadLocation(t, "Europe/Berlin")
	now := time.Date(2026, time.October, 5, 12, 0, 0, 0, location)
	tests := []struct {
		name    string
		weekday time.Weekday
		want    time.Time
	}{
		{"Monday", time.Monday, time.Date(2026, time.October, 5, 18, 0, 0, 0, location)},
		{"Tuesday", time.Tuesday, time.Date(2026, time.October, 6, 18, 0, 0, 0, location)},
		{"Wednesday", time.Wednesday, time.Date(2026, time.October, 7, 18, 0, 0, 0, location)},
		{"Thursday", time.Thursday, time.Date(2026, time.October, 8, 18, 0, 0, 0, location)},
		{"Friday", time.Friday, time.Date(2026, time.October, 9, 18, 0, 0, 0, location)},
		{"Saturday", time.Saturday, time.Date(2026, time.October, 10, 18, 0, 0, 0, location)},
		{"Sunday", time.Sunday, time.Date(2026, time.October, 11, 18, 0, 0, 0, location)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := SelectDeadline(now, location, test.weekday, 18, 0, 3*time.Hour)
			if err != nil {
				t.Fatalf("SelectDeadline() error = %v", err)
			}
			if !got.Equal(test.want) {
				t.Errorf("SelectDeadline() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestSelectDeadlineSameDayCutoff(t *testing.T) {
	location := mustLoadLocation(t, "Europe/Berlin")
	tests := []struct {
		name string
		now  time.Time
		day  int
	}{
		{"before cutoff", time.Date(2026, time.October, 8, 14, 59, 59, 0, location), 8},
		{"at cutoff", time.Date(2026, time.October, 8, 15, 0, 0, 0, location), 15},
		{"after cutoff", time.Date(2026, time.October, 8, 15, 0, 1, 0, location), 15},
		{"at deadline", time.Date(2026, time.October, 8, 18, 0, 0, 0, location), 15},
		{"after deadline", time.Date(2026, time.October, 8, 23, 0, 0, 0, location), 15},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := SelectDeadline(test.now, location, time.Thursday, 18, 0, 3*time.Hour)
			if err != nil {
				t.Fatalf("SelectDeadline() error = %v", err)
			}
			want := time.Date(2026, time.October, test.day, 18, 0, 0, 0, location)
			if !got.Equal(want) {
				t.Errorf("SelectDeadline() = %s, want %s", got, want)
			}
		})
	}
}

func TestSelectDeadlineFractionalThreshold(t *testing.T) {
	location := mustLoadLocation(t, "Europe/Berlin")
	tests := []struct {
		name string
		now  time.Time
		day  int
	}{
		{"before cutoff", time.Date(2026, time.October, 8, 16, 29, 59, 0, location), 8},
		{"at cutoff", time.Date(2026, time.October, 8, 16, 30, 0, 0, location), 15},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := SelectDeadline(test.now, location, time.Thursday, 18, 0, 90*time.Minute)
			if err != nil {
				t.Fatalf("SelectDeadline() error = %v", err)
			}
			want := time.Date(2026, time.October, test.day, 18, 0, 0, 0, location)
			if !got.Equal(want) {
				t.Errorf("SelectDeadline() = %s, want %s", got, want)
			}
		})
	}
}

func TestSelectDeadlineCalendarBoundaries(t *testing.T) {
	location := mustLoadLocation(t, "Europe/Berlin")
	tests := []struct {
		name    string
		now     time.Time
		weekday time.Weekday
		hour    int
		minute  int
		want    time.Time
	}{
		{
			"midnight target",
			time.Date(2026, time.October, 5, 23, 59, 0, 0, location),
			time.Tuesday, 0, 0,
			time.Date(2026, time.October, 6, 0, 0, 0, 0, location),
		},
		{
			"month boundary",
			time.Date(2026, time.April, 29, 12, 0, 0, 0, location),
			time.Thursday, 18, 0,
			time.Date(2026, time.April, 30, 18, 0, 0, 0, location),
		},
		{
			"year boundary",
			time.Date(2026, time.December, 25, 12, 0, 0, 0, location),
			time.Thursday, 18, 0,
			time.Date(2026, time.December, 31, 18, 0, 0, 0, location),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := SelectDeadline(test.now, location, test.weekday, test.hour, test.minute, 3*time.Hour)
			if err != nil {
				t.Fatalf("SelectDeadline() error = %v", err)
			}
			if !got.Equal(test.want) {
				t.Errorf("SelectDeadline() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestSelectDeadlineTimezoneAndDST(t *testing.T) {
	location := mustLoadLocation(t, "Europe/Berlin")
	tests := []struct {
		name    string
		now     time.Time
		weekday time.Weekday
		hour    int
		want    time.Time
	}{
		{
			"UTC execution instant",
			time.Date(2026, time.October, 8, 12, 59, 59, 0, time.UTC),
			time.Thursday, 18,
			time.Date(2026, time.October, 8, 18, 0, 0, 0, location),
		},
		{
			"wall time preserved across spring DST change",
			time.Date(2026, time.March, 26, 17, 0, 0, 0, location),
			time.Thursday, 18,
			time.Date(2026, time.April, 2, 18, 0, 0, 0, location),
		},
		{
			"same-day cutoff uses elapsed time across spring DST change",
			time.Date(2026, time.March, 29, 0, 30, 0, 0, location),
			time.Sunday, 4,
			time.Date(2026, time.April, 5, 4, 0, 0, 0, location),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := SelectDeadline(test.now, location, test.weekday, test.hour, 0, 3*time.Hour)
			if err != nil {
				t.Fatalf("SelectDeadline() error = %v", err)
			}
			if !got.Equal(test.want) || got.Location() != location {
				t.Errorf("SelectDeadline() = %s (%s), want %s (%s)", got, got.Location(), test.want, location)
			}
		})
	}
}

func TestSelectDeadlineRejectsDSTGapAndFold(t *testing.T) {
	location := mustLoadLocation(t, "Europe/Berlin")
	tests := []struct {
		name string
		now  time.Time
	}{
		{
			"nonexistent spring time",
			time.Date(2026, time.March, 23, 12, 0, 0, 0, location),
		},
		{
			"ambiguous fall time",
			time.Date(2026, time.October, 19, 12, 0, 0, 0, location),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := SelectDeadline(test.now, location, time.Sunday, 2, 30, 3*time.Hour)
			if err == nil {
				t.Fatal("SelectDeadline() error = nil, want DST wall-time error")
			}
		})
	}
}

func TestSelectDeadlineRejectsInvalidArguments(t *testing.T) {
	location := mustLoadLocation(t, "Europe/Berlin")
	now := time.Date(2026, time.October, 5, 12, 0, 0, 0, location)
	tests := []struct {
		name      string
		location  *time.Location
		weekday   time.Weekday
		hour      int
		minute    int
		threshold time.Duration
	}{
		{"nil location", nil, time.Thursday, 18, 0, 3 * time.Hour},
		{"invalid weekday", location, time.Weekday(7), 18, 0, 3 * time.Hour},
		{"invalid hour", location, time.Thursday, 24, 0, 3 * time.Hour},
		{"invalid minute", location, time.Thursday, 18, 60, 3 * time.Hour},
		{"threshold below one hour", location, time.Thursday, 18, 0, time.Hour - 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := SelectDeadline(now, test.location, test.weekday, test.hour, test.minute, test.threshold)
			if err == nil {
				t.Fatal("SelectDeadline() error = nil, want validation error")
			}
		})
	}
}

func TestDurationHours(t *testing.T) {
	deadline := time.Date(2026, time.October, 8, 18, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		now  time.Time
		want int
		err  bool
	}{
		{"fractional remaining hour floors", deadline.Add(-168*time.Hour - 30*time.Minute), 168, false},
		{"exact hour", deadline.Add(-time.Hour), 1, false},
		{"subsecond below one hour", deadline.Add(-time.Hour + time.Nanosecond), 0, true},
		{"deadline passed", deadline.Add(time.Nanosecond), 0, true},
		{"maximum duration", deadline.Add(-768 * time.Hour), 768, false},
		{"over maximum duration", deadline.Add(-769 * time.Hour), 0, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := DurationHours(deadline, test.now)
			if test.err {
				if err == nil {
					t.Fatalf("DurationHours() = %d, error = nil; want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("DurationHours() error = %v", err)
			}
			if got != test.want {
				t.Errorf("DurationHours() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestDurationHoursExpiryWindow(t *testing.T) {
	deadline := time.Date(2026, time.October, 8, 18, 0, 0, 0, time.UTC)
	for _, remaining := range []time.Duration{
		time.Hour,
		time.Hour + time.Nanosecond,
		90 * time.Minute,
		24*time.Hour + time.Nanosecond,
		168 * time.Hour,
		168*time.Hour + 30*time.Minute,
	} {
		now := deadline.Add(-remaining)
		hours, err := DurationHours(deadline, now)
		if err != nil {
			t.Fatalf("DurationHours(%s) error = %v", remaining, err)
		}
		nominalExpiry := now.Add(time.Duration(hours) * time.Hour)
		if !nominalExpiry.After(deadline.Add(-time.Hour)) || nominalExpiry.After(deadline) {
			t.Errorf("remaining %s produced expiry %s outside (%s, %s]", remaining, nominalExpiry, deadline.Add(-time.Hour), deadline)
		}
	}
}

func mustLoadLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	location, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("time.LoadLocation(%q): %v", name, err)
	}
	return location
}
