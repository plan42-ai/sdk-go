package p42_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/plan42-ai/sdk-go/p42"
)

func TestLogStream(t *testing.T) {
	t.Parallel()

	var calls int
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				calls++
				switch calls {
				case 1:
					if hdr := r.Header.Get("Last-Event-ID"); hdr != "" {
						t.Errorf("unexpected last-event-id on first call: %s", hdr)
					}
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(
						w,
						"id: 1\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:00Z\",\"Message\":\"one\"}\nretry: 10\n\n",
					)
					fmt.Fprintf(
						w,
						"id: 2\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:01Z\",\"Message\":\"two\"}\n\n",
					)
				case 2:
					if hdr := r.Header.Get("Last-Event-ID"); hdr != "2" {
						t.Errorf("expected last-event-id 2, got %s", hdr)
					}
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected call")
				}
			},
		),
	)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	ls := p42.NewLogStream(client, "ten", "task", 0, 10)
	defer ls.Close()

	var logs []p42.TurnLog
	for log := range ls.Logs() {
		logs = append(logs, log)
	}

	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}
	if logs[0].Message != "one" || logs[1].Message != "two" {
		t.Fatalf("unexpected logs: %#v", logs)
	}

	if err := ls.ShutdownTimeout(2 * time.Second); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
}

func TestLogStreamCloseDuringRead(t *testing.T) {
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				<-r.Context().Done()
			},
		),
	)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	ls := p42.NewLogStream(client, "ten", "task", 0, 1)

	time.Sleep(50 * time.Millisecond)

	start := time.Now()
	if err := ls.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("close took too long: %v", elapsed)
	}
}

func TestLogStreamWithLastID_UsesHeader(t *testing.T) {
	var gotLastEventID string
	var called int
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				called++
				if called > 1 {
					// Do not allow reconnects in this test
					return
				}
				gotLastEventID = r.Header.Get("Last-Event-ID")
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(
					w,
					"id: 5\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:00Z\",\"Message\":\"resumed\"}\n\n",
				)
				// Close connection after sending one event
			},
		),
	)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	ls := p42.NewLogStream(client, "ten", "task", 0, 1, p42.WithLastID(42))

	var logs []p42.TurnLog
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for log := range ls.Logs() {
			logs = append(logs, log)
		}
	}()
	time.Sleep(50 * time.Millisecond)
	ls.Close()
	wg.Wait()

	if gotLastEventID != "42" {
		t.Errorf("expected Last-Event-ID header '42', got '%s'", gotLastEventID)
	}
	if len(logs) != 1 || logs[0].Message != "resumed" {
		t.Errorf("unexpected logs: %#v", logs)
	}
}

func TestLogStreamWithLastID_UpdatesLastID(t *testing.T) {
	var gotLastEventID string
	var called int
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				called++
				if called > 1 {
					// Prevent reconnects
					return
				}
				gotLastEventID = r.Header.Get("Last-Event-ID")
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(
					w,
					"id: 100\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:00Z\",\"Message\":\"foo\"}\n\n",
				)
				// Connection will close after handler returns
			},
		),
	)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	ls := p42.NewLogStream(client, "ten", "task", 0, 1, p42.WithLastID(99))

	var logs []p42.TurnLog
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for log := range ls.Logs() {
			logs = append(logs, log)
		}
	}()
	time.Sleep(50 * time.Millisecond)
	ls.Close()
	wg.Wait()

	if gotLastEventID != "99" {
		t.Errorf("expected Last-Event-ID header '99', got '%s'", gotLastEventID)
	}
	if len(logs) != 1 || logs[0].Message != "foo" {
		t.Errorf("unexpected logs: %#v", logs)
	}
	// Check that lastID is updated after receiving event with id: 100
	if lsLastID := ls.LastEventID(); lsLastID != 100 {
		t.Errorf("expected lastID to be updated to 100, got %d", lsLastID)
	}
}

func TestLogStreamWorkstreamID(t *testing.T) {
	t.Parallel()

	var gotWorkstreamID string
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				gotWorkstreamID = r.URL.Query().Get("workstreamID")
				w.WriteHeader(http.StatusNoContent)
			},
		),
	)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	wsID := "ws-123"
	ls := p42.NewLogStream(client, "ten", "task", 0, 1, p42.WithWorkstreamID(&wsID))
	defer ls.Close()

	if err := ls.ShutdownTimeout(time.Second); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}

	if gotWorkstreamID != "ws-123" {
		t.Fatalf("expected workstreamID query 'ws-123', got '%s'", gotWorkstreamID)
	}
}

func TestLogStreamReadBoundedBatch(t *testing.T) {
	t.Parallel()

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "id: 1\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:00Z\",\"Message\":\"one\"}\n\n")
			fmt.Fprintf(w, "id: 2\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:01Z\",\"Message\":\"two\"}\n\n")
			fmt.Fprintf(w, "id: 3\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:02Z\",\"Message\":\"three\"}\n\n")
		case 2:
			if got := r.Header.Get("Last-Event-ID"); got != "3" {
				t.Fatalf("expected resume header 3, got %s", got)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected extra call")
		}
	}))
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	ls := p42.NewLogStream(client, "ten", "task", 0, 10)
	defer ls.Close()

	result, err := ls.Read(context.Background(), p42.LogStreamReadOptions{MaxEvents: 2, Timeout: time.Second})
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if len(result.Logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(result.Logs))
	}
	if !result.MaxEventsHit {
		t.Fatalf("expected MaxEventsHit to be true")
	}
	if result.LastEventID < 2 {
		t.Fatalf("expected LastEventID to have advanced past delivered logs, got %d", result.LastEventID)
	}

	result, err = ls.Read(context.Background(), p42.LogStreamReadOptions{Timeout: time.Second})
	if err != nil {
		t.Fatalf("resume read error: %v", err)
	}
	if len(result.Logs) > 1 {
		t.Fatalf("expected at most one resumed log, got %#v", result.Logs)
	}
	if len(result.Logs) == 1 && result.Logs[0].Message != "three" {
		t.Fatalf("unexpected resumed logs: %#v", result.Logs)
	}
	if !result.EndOfAvailable {
		t.Fatalf("expected end of available logs after resume")
	}
}

func TestLogStreamReadTimeoutStop(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-r.Context().Done()
	}))
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	ls := p42.NewLogStream(client, "ten", "task", 0, 1)
	defer ls.Close()

	result, err := ls.Read(context.Background(), p42.LogStreamReadOptions{Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if !result.TimedOut {
		t.Fatalf("expected timeout result")
	}
	if len(result.Logs) != 0 {
		t.Fatalf("expected no logs, got %d", len(result.Logs))
	}
}

func TestLogStreamLastEventIDExposure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "id: 55\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:00Z\",\"Message\":\"checkpoint\"}\n\n")
	}))
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	ls := p42.NewLogStream(client, "ten", "task", 0, 1)
	defer ls.Close()

	result, err := ls.Read(context.Background(), p42.LogStreamReadOptions{MaxEvents: 1, Timeout: time.Second})
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if result.LastEventID != 55 {
		t.Fatalf("expected result last event ID 55, got %d", result.LastEventID)
	}
	if got := ls.LastEventID(); got != 55 {
		t.Fatalf("expected stream last event ID 55, got %d", got)
	}
}

func TestLogStreamEndOfAvailableLogDetection(t *testing.T) {
	t.Parallel()

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "id: 1\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:00Z\",\"Message\":\"only\"}\n\n")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	ls := p42.NewLogStream(client, "ten", "task", 0, 10)
	defer ls.Close()

	first, err := ls.Read(context.Background(), p42.LogStreamReadOptions{MaxEvents: 1, Timeout: time.Second})
	if err != nil {
		t.Fatalf("first read error: %v", err)
	}
	if first.EndOfAvailable {
		t.Fatalf("did not expect end-of-available after bounded first read")
	}

	second, err := ls.Read(context.Background(), p42.LogStreamReadOptions{Timeout: time.Second})
	if err != nil {
		t.Fatalf("second read error: %v", err)
	}
	if !second.EndOfAvailable {
		t.Fatalf("expected end-of-available on second read")
	}
	if !ls.EndOfAvailable() {
		t.Fatalf("expected stream to report end-of-available")
	}
}
