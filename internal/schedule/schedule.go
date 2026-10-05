package schedule

import (
	"fmt"
	"time"
)

const (
	minimumDurationHours = 1
	maximumDurationHours = 768
)

// SelectDeadline selects the next target wall-clock deadline in location.
func SelectDeadline(now time.Time, location *time.Location, targetWeekday time.Weekday, hour, minute int, sameDayThreshold time.Duration) (time.Time, error) {
	if location == nil {
		return time.Time{}, fmt.Errorf("location must not be nil")
	}
	if targetWeekday < time.Sunday || targetWeekday > time.Saturday {
		return time.Time{}, fmt.Errorf("target weekday is invalid")
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return time.Time{}, fmt.Errorf("target time is invalid")
	}
	if sameDayThreshold < time.Hour {
		return time.Time{}, fmt.Errorf("same-day threshold must be at least 1 hour")
	}

	localNow := now.In(location)
	daysUntilTarget := (int(targetWeekday) - int(localNow.Weekday()) + 7) % 7
	deadlineWall := time.Date(
		localNow.Year(),
		localNow.Month(),
		localNow.Day()+daysUntilTarget,
		hour,
		minute,
		0,
		0,
		time.UTC,
	)
	deadline, err := resolveWallTime(deadlineWall, location)
	if err != nil {
		return time.Time{}, err
	}
	if daysUntilTarget == 0 && !now.Before(deadline.Add(-sameDayThreshold)) {
		deadlineWall = deadlineWall.AddDate(0, 0, 7)
		return resolveWallTime(deadlineWall, location)
	}
	return deadline, nil
}

// DurationHours returns the whole-hour Discord poll duration for a deadline.
func DurationHours(deadline, sendTime time.Time) (int, error) {
	remaining := deadline.Sub(sendTime)
	hours := int(remaining / time.Hour)
	if hours < minimumDurationHours || hours > maximumDurationHours {
		return 0, fmt.Errorf("remaining duration must be between %d and %d whole hours", minimumDurationHours, maximumDurationHours)
	}
	return hours, nil
}

func resolveWallTime(wall time.Time, location *time.Location) (time.Time, error) {
	offsets := make(map[int]struct{})
	for probe := wall.Add(-48 * time.Hour); !probe.After(wall.Add(48 * time.Hour)); probe = probe.Add(15 * time.Minute) {
		_, offset := probe.In(location).Zone()
		offsets[offset] = struct{}{}
	}

	var candidates []time.Time
	for offset := range offsets {
		candidate := wall.Add(-time.Duration(offset) * time.Second)
		local := candidate.In(location)
		if sameWallTime(local, wall) {
			candidates = append(candidates, candidate)
		}
	}

	switch len(candidates) {
	case 0:
		return time.Time{}, fmt.Errorf("target wall-clock time does not exist in %s", location)
	case 1:
		return candidates[0].In(location), nil
	default:
		return time.Time{}, fmt.Errorf("target wall-clock time is ambiguous in %s", location)
	}
}

func sameWallTime(local, wall time.Time) bool {
	return local.Year() == wall.Year() &&
		local.Month() == wall.Month() &&
		local.Day() == wall.Day() &&
		local.Hour() == wall.Hour() &&
		local.Minute() == wall.Minute() &&
		local.Second() == wall.Second() &&
		local.Nanosecond() == wall.Nanosecond()
}
