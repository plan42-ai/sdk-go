package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseDurationWithDays(t *testing.T) {
	t.Parallel()

	dur, err := parseDurationWithDays("1.5d")
	require.NoError(t, err)
	require.Equal(t, 36*time.Hour, dur)
}

func TestParseDurationWithDaysInvalid(t *testing.T) {
	t.Parallel()

	_, err := parseDurationWithDays("2x")
	require.Error(t, err)
}

func TestParseTimeOrDurationWithDuration(t *testing.T) {
	t.Parallel()

	now := time.Date(2024, 7, 10, 12, 0, 0, 0, time.UTC)
	timeValue, err := parseTimeOrDuration("-1.5h", now)
	require.NoError(t, err)
	require.Equal(t, now.Add(-90*time.Minute), timeValue)
}

func TestParseTimeOrDurationWithTime(t *testing.T) {
	t.Parallel()

	expected := time.Date(2024, 7, 10, 10, 0, 0, 0, time.UTC)
	timeValue, err := parseTimeOrDuration(expected.Format(time.RFC3339), time.Now())
	require.NoError(t, err)
	require.Equal(t, expected, timeValue)
}

func TestParseTimeOrDurationWithNanoTime(t *testing.T) {
	t.Parallel()

	expected := time.Date(2024, 7, 10, 10, 0, 0, 123456000, time.UTC)
	timeValue, err := parseTimeOrDuration(expected.Format(time.RFC3339Nano), time.Now())
	require.NoError(t, err)
	require.Equal(t, expected, timeValue)
}

func TestParseTimeOrDurationInvalid(t *testing.T) {
	t.Parallel()

	_, err := parseTimeOrDuration("invalid", time.Now())
	require.Error(t, err)
}
