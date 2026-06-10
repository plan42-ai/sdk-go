package p42_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/plan42-ai/sdk-go/p42"
	"github.com/stretchr/testify/require"
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
	for entry := range ls.Events() {
		logs = append(logs, entry.Event)
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
		for entry := range ls.Events() {
			logs = append(logs, entry.Event)
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
		for entry := range ls.Events() {
			logs = append(logs, entry.Event)
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
	if lsLastID := ls.LastEventID(); lsLastID != "100" {
		t.Errorf("expected lastID to be updated to 100, got %s", lsLastID)
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

func TestLogStreamReadBatchBounded(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, "id: 1\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:00Z\",\"Message\":\"one\"}\n\n")
				_, _ = io.WriteString(w, "id: 2\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:01Z\",\"Message\":\"two\"}\n\n")
				_, _ = io.WriteString(w, "id: 3\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:02Z\",\"Message\":\"three\"}\n\n")
			},
		),
	)
	defer srv.Close()

	ls := p42.NewLogStream(p42.NewClient(srv.URL), "ten", "task", 0, 10)
	defer ls.Close()

	batch, err := ls.ReadBatch(context.Background(), 2, time.Second, "0")
	require.NoError(t, err)
	require.Len(t, batch.Events, 2)
	require.Equal(t, "one", batch.Events[0].Message)
	require.Equal(t, "two", batch.Events[1].Message)
	require.Equal(t, "2", batch.LastEventID)
	require.False(t, batch.ReachedEnd)
}

func TestLogStreamReadBatchTimeout(t *testing.T) {
	t.Parallel()

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

	ls := p42.NewLogStream(p42.NewClient(srv.URL), "ten", "task", 0, 1)
	defer ls.Close()

	batch, err := ls.ReadBatch(context.Background(), 1, 50*time.Millisecond, "0")
	require.NoError(t, err)
	require.Empty(t, batch.Events)
	require.Equal(t, "0", batch.LastEventID)
	require.False(t, batch.ReachedEnd)
}

func TestLogStreamReadBatchResumeUsesLastEventID(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32
	lastEventIDs := make(chan string, 4)
	allowSecondResponse := make(chan struct{})
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				call := requestCount.Add(1)
				lastEventIDs <- r.Header.Get("Last-Event-ID")
				w.Header().Set("Content-Type", "text/event-stream")
				switch call {
				case 1:
					_, _ = io.WriteString(w, "id: 1\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:00Z\",\"Message\":\"one\"}\n\n")
					_, _ = io.WriteString(w, "id: 2\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:01Z\",\"Message\":\"two\"}\n\n")
				case 2:
					<-allowSecondResponse
					_, _ = io.WriteString(w, "id: 3\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:02Z\",\"Message\":\"three\"}\n\n")
				default:
					w.WriteHeader(http.StatusNoContent)
				}
			},
		),
	)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	first := p42.NewLogStream(client, "ten", "task", 0, 10)
	firstBatch, err := first.ReadBatch(context.Background(), 2, time.Second, "0")
	require.NoError(t, err)
	require.Len(t, firstBatch.Events, 2)
	require.Equal(t, "2", firstBatch.LastEventID)
	require.NoError(t, first.Close())

	lastID, err := strconv.Atoi(firstBatch.LastEventID)
	require.NoError(t, err)
	second := p42.NewLogStream(client, "ten", "task", 0, 10, p42.WithLastID(lastID))
	defer second.Close()
	close(allowSecondResponse)
	secondBatch, err := second.ReadBatch(context.Background(), 1, 50*time.Millisecond, "2")
	require.NoError(t, err)
	if len(secondBatch.Events) == 1 {
		require.Equal(t, "three", secondBatch.Events[0].Message)
	}
	require.Equal(t, secondBatch.LastEventID, second.LastEventID())
	require.GreaterOrEqual(t, secondBatch.LastEventID, firstBatch.LastEventID)
	require.NoError(t, second.Close())
	require.Equal(t, "", <-lastEventIDs)
	require.Equal(t, firstBatch.LastEventID, <-lastEventIDs)
}

func TestLogStreamReadBatchReachedEnd(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				call := requestCount.Add(1)
				if call == 1 {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, "id: 1\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:00Z\",\"Message\":\"one\"}\n\n")
					return
				}
				w.WriteHeader(http.StatusNoContent)
			},
		),
	)
	defer srv.Close()

	ls := p42.NewLogStream(p42.NewClient(srv.URL), "ten", "task", 0, 10)
	defer ls.Close()

	firstBatch, err := ls.ReadBatch(context.Background(), 1, time.Second, "0")
	require.NoError(t, err)
	require.Len(t, firstBatch.Events, 1)
	require.False(t, firstBatch.ReachedEnd)

	secondBatch, err := ls.ReadBatch(context.Background(), 1, time.Second, "1")
	require.NoError(t, err)
	require.Empty(t, secondBatch.Events)
	require.True(t, secondBatch.ReachedEnd)
	require.Equal(t, "1", secondBatch.LastEventID)
}
