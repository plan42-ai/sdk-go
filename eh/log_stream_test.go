package eh_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
)

func TestLogStream(t *testing.T) {
	t.Parallel()

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			if hdr := r.Header.Get("Last-Event-ID"); hdr != "" {
				t.Errorf("unexpected last-event-id on first call: %s", hdr)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "id: 1\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:00Z\",\"Message\":\"one\"}\nretry: 10\n\n")
			fmt.Fprintf(w, "id: 2\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:01Z\",\"Message\":\"two\"}\n\n")
		case 2:
			if hdr := r.Header.Get("Last-Event-ID"); hdr != "2" {
				t.Errorf("expected last-event-id 2, got %s", hdr)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected call")
		}
	}))
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	ls := eh.NewLogStream(client, "ten", "task", 0, 10, nil, nil)
	defer ls.Close()

	var logs []eh.TurnLog
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-r.Context().Done()
	}))
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	ls := eh.NewLogStream(client, "ten", "task", 0, 1, nil, nil)

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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLastEventID = r.Header.Get("Last-Event-ID")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "id: 5\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:00Z\",\"Message\":\"resumed\"}\n\n")
	}))
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	ls := eh.NewLogStreamWithLastID(client, "ten", "task", 0, 1, nil, nil, 42)
	defer ls.Close()

	var logs []eh.TurnLog
	for log := range ls.Logs() {
		logs = append(logs, log)
	}

	if gotLastEventID != "42" {
		t.Errorf("expected Last-Event-ID header '42', got '%s'", gotLastEventID)
	}
	if len(logs) != 1 || logs[0].Message != "resumed" {
		t.Errorf("unexpected logs: %#v", logs)
	}
}

func TestLogStreamWithLastID_UpdatesLastID(t *testing.T) {
	var gotLastEventID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLastEventID = r.Header.Get("Last-Event-ID")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "id: 100\nevent: log\ndata: {\"Timestamp\":\"2025-01-01T00:00:00Z\",\"Message\":\"foo\"}\n\n")
	}))
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	ls := eh.NewLogStreamWithLastID(client, "ten", "task", 0, 1, nil, nil, 99)
	defer ls.Close()

	var logs []eh.TurnLog
	for log := range ls.Logs() {
		logs = append(logs, log)
	}

	if gotLastEventID != "99" {
		t.Errorf("expected Last-Event-ID header '99', got '%s'", gotLastEventID)
	}
	if len(logs) != 1 || logs[0].Message != "foo" {
		t.Errorf("unexpected logs: %#v", logs)
	}
	// Check that lastID is updated after receiving event with id: 100
	if lsLastID := getLogStreamLastID(ls); lsLastID != 100 {
		t.Errorf("expected lastID to be updated to 100, got %d", lsLastID)
	}
}

// getLogStreamLastID is a helper to access the unexported lastID field for testing.
func getLogStreamLastID(ls interface{}) int {
	type lastIDGetter interface{ LastID() int }
	if g, ok := ls.(lastIDGetter); ok {
		return g.LastID()
	}
	// fallback to reflection if needed
	return -1
}
