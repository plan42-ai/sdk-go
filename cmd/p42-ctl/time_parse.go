package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const day = 24 * time.Hour

func parseDurationWithDays(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("duration is required")
	}

	sign := 1.0
	if strings.HasPrefix(trimmed, "+") {
		trimmed = trimmed[1:]
	} else if strings.HasPrefix(trimmed, "-") {
		sign = -1
		trimmed = trimmed[1:]
	}

	if len(trimmed) < 2 {
		return 0, fmt.Errorf("duration %q is missing units", value)
	}

	unit := trimmed[len(trimmed)-1]
	number := trimmed[:len(trimmed)-1]
	magnitude, err := strconv.ParseFloat(number, 64)
	if err != nil {
		return 0, err
	}

	var base time.Duration
	switch unit {
	case 'd':
		base = day
	case 'h':
		base = time.Hour
	case 'm':
		base = time.Minute
	case 's':
		base = time.Second
	default:
		return 0, fmt.Errorf("invalid duration suffix %q", string(unit))
	}

	dur := time.Duration(sign * magnitude * float64(base))
	return dur, nil
}

func parseTimeOrDuration(value string, now time.Time) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("time value is required")
	}

	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		timestamp, err := time.Parse(layout, trimmed)
		if err == nil {
			return timestamp, nil
		}
	}

	dur, durErr := parseDurationWithDays(trimmed)
	if durErr != nil {
		return time.Time{}, fmt.Errorf("failed to parse %q as duration or RFC3339 time", value)
	}

	return now.Add(dur), nil
}
