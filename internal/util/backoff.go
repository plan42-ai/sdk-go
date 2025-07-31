package util

import (
	"context"
	rand "math/rand/v2"
	"time"
)

// Backoff provides exponential backoff with jitter.
type Backoff struct {
	min     time.Duration
	max     time.Duration
	current time.Duration
	timer   *time.Timer
}

// NewBackoff creates a new Backoff with the given minimum and maximum durations.
func NewBackoff(minBackoff, maxBackoff time.Duration) *Backoff {
	if minBackoff <= 0 {
		panic("minBackoff must be positive")
	}
	if maxBackoff < minBackoff {
		panic("maxBackoff must be >= minBackoff")
	}
	return &Backoff{min: minBackoff, max: maxBackoff}
}

func (b *Backoff) setupTimer(d time.Duration) *time.Timer {
	if b.timer == nil {
		b.timer = time.NewTimer(d)
	} else {
		if !b.timer.Stop() {
			select {
			case <-b.timer.C:
			default:
			}
		}
		b.timer.Reset(d)
	}
	return b.timer
}

// Wait sleeps for a random duration less than or equal to the current backoff.
// If the backoff is zero, Wait returns immediately.
func (b *Backoff) Wait(ctx context.Context) error {
	if b.current == 0 {
		return nil
	}
	d := rand.N[time.Duration](b.current)
	t := b.setupTimer(d)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// WaitAtLeast waits for at least minWait or the current jittered backoff,
// whichever is greater.
func (b *Backoff) WaitAtLeast(ctx context.Context, minWait time.Duration) error {
	wait := time.Duration(0)
	if b.current > 0 {
		wait = rand.N[time.Duration](b.current)
	}
	if wait < minWait {
		wait = minWait
	}
	if wait == 0 {
		return nil
	}
	t := b.setupTimer(wait)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Backoff increases the current backoff, doubling it up to the configured maximum.
func (b *Backoff) Backoff() {
	if b.current == 0 {
		b.current = b.min
		return
	}
	b.current *= 2
	if b.current > b.max {
		b.current = b.max
	}
}

// Recover decreases the current backoff by half, resetting to zero once below the minimum.
func (b *Backoff) Recover() {
	if b.current == 0 {
		return
	}
	if b.current/2 < b.min {
		b.current = 0
	} else {
		b.current /= 2
	}
}

// Current returns the current backoff duration.
func (b *Backoff) Current() time.Duration {
	return b.current
}
