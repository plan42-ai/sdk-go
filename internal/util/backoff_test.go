package util_test

import (
	"context"
	"testing"
	"time"

	"github.com/debugging-sucks/event-horizon-sdk-go/internal/util"
)

func TestBackoffBasic(t *testing.T) {
	b := util.NewBackoff(10*time.Millisecond, 40*time.Millisecond)

	// Wait should return immediately when current is 0
	start := time.Now()
	err := b.Wait(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if time.Since(start) > 5*time.Millisecond {
		t.Fatalf("wait took too long with zero backoff")
	}

	// Increase backoff and ensure it doesn't exceed max
	b.Backoff() // 10ms
	if b.Current() != 10*time.Millisecond {
		t.Fatalf("expected 10ms, got %v", b.Current())
	}

	start = time.Now()
	err = b.Wait(context.Background())
	if err != nil {
		t.Fatalf("wait error: %v", err)
	}
	if time.Since(start) > 20*time.Millisecond+5*time.Millisecond {
		t.Fatalf("wait exceeded expected maximum")
	}

	b.Backoff() // 20ms
	b.Backoff() // 40ms
	b.Backoff() // still 40ms
	if b.Current() != 40*time.Millisecond {
		t.Fatalf("expected max 40ms, got %v", b.Current())
	}

	b.Recover() // 20ms
	if b.Current() != 20*time.Millisecond {
		t.Fatalf("recover expected 20ms, got %v", b.Current())
	}
	b.Recover() // 10ms
	b.Recover() // 0
	if b.Current() != 0 {
		t.Fatalf("expected 0 after recover, got %v", b.Current())
	}
}

func TestBackoffContextCancel(t *testing.T) {
	b := util.NewBackoff(5*time.Millisecond, 10*time.Millisecond)
	b.Backoff() // set current to min
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := b.Wait(ctx); err == nil {
		t.Fatalf("expected context error")
	}
}

func TestBackoffWaitAtLeast(t *testing.T) {
	b := util.NewBackoff(5*time.Millisecond, 20*time.Millisecond)

	start := time.Now()
	if err := b.WaitAtLeast(context.Background(), 10*time.Millisecond); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 10*time.Millisecond {
		t.Fatalf("waited %v, expected at least 10ms", elapsed)
	}

	b.Backoff() // set to min 5ms
	start = time.Now()
	if err := b.WaitAtLeast(context.Background(), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if time.Since(start) > 5*time.Millisecond+5*time.Millisecond {
		t.Fatalf("wait exceeded expected maximum")
	}
}
