package p42_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/plan42-ai/sdk-go/p42"
	"github.com/stretchr/testify/require"
)

func TestMessageStream(t *testing.T) {
	t.Parallel()

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			require.Equal(t, "", r.Header.Get("Last-Event-ID"))
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "id: a1\nevent: AgentMessage\ndata: {\"MessageID\":\"m1\",\"FromAgentID\":\"agent-2\",\"To\":[\"agent-1\"],\"Message\":\"hello\",\"CreatedAt\":\"2025-01-01T00:00:00Z\"}\n\n")
			fmt.Fprintf(w, "id: a2\nevent: SubAgentCompletion\ndata: {\"AgentID\":\"agent-2\",\"StillRunning\":false,\"CompletionMessage\":\"done\"}\n\n")
		case 2:
			require.Equal(t, "a2", r.Header.Get("Last-Event-ID"))
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected call %d", calls)
		}
	}))
	defer srv.Close()

	ms := p42.NewMessageStream(p42.NewClient(srv.URL), "ten", "task", 0, 10, &p42.MessageStreamConfig{AgentID: "agent-1"})
	defer ms.Close()

	var events []p42.TurnMessageEvent
	for entry := range ms.Events() {
		events = append(events, entry.Event)
	}

	require.Len(t, events, 2)
	require.Equal(t, p42.TurnMessageTypeAgentMessage, events[0].Type)
	require.NotNil(t, events[0].Message)
	require.Equal(t, "hello", events[0].Message.Message)
	require.Nil(t, events[0].SubAgentCompletion)
	require.Equal(t, p42.TurnMessageTypeSubAgentCompletion, events[1].Type)
	require.NotNil(t, events[1].SubAgentCompletion)
	require.Equal(t, "done", events[1].SubAgentCompletion.CompletionMessage)
	require.Nil(t, events[1].Message)
	require.NoError(t, ms.ShutdownTimeout(2*time.Second))
}

func TestMessageStreamCloseDuringRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-r.Context().Done()
	}))
	defer srv.Close()

	ms := p42.NewMessageStream(p42.NewClient(srv.URL), "ten", "task", 0, 1, &p42.MessageStreamConfig{AgentID: "agent-1"})
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	require.NoError(t, ms.Close())
	require.Less(t, time.Since(start), 200*time.Millisecond)
}

func TestMessageStreamWithLastEventID(t *testing.T) {
	var gotLastEventID string
	var called int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		if called > 1 {
			return
		}
		gotLastEventID = r.Header.Get("Last-Event-ID")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "id: next\nevent: UserMessage\ndata: {\"MessageID\":\"m1\",\"FromTenantID\":\"tenant-1\",\"To\":[\"%s\"],\"Message\":\"resume\",\"CreatedAt\":\"2025-01-01T00:00:00Z\"}\n\n", p42.MainAgentID)
	}))
	defer srv.Close()

	ms := p42.NewMessageStream(p42.NewClient(srv.URL), "ten", "task", 0, 1, &p42.MessageStreamConfig{AgentID: p42.MainAgentID, LastEventID: "prior"})
	var events []p42.TurnMessageEvent
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for entry := range ms.Events() {
			events = append(events, entry.Event)
		}
	}()
	time.Sleep(50 * time.Millisecond)
	ms.Close()
	wg.Wait()
	require.Equal(t, "prior", gotLastEventID)
	require.Len(t, events, 1)
	require.Equal(t, "resume", events[0].Message.Message)
	require.Equal(t, "next", ms.LastEventID())
}

func TestMessageStreamReadBatchReachedEnd(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := requestCount.Add(1)
		if call == 1 {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "id: 1\nevent: AgentMessage\ndata: {\"MessageID\":\"m1\",\"FromAgentID\":\"agent-2\",\"To\":[\"agent-1\"],\"Message\":\"one\",\"CreatedAt\":\"2025-01-01T00:00:00Z\"}\n\n")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	ms := p42.NewMessageStream(p42.NewClient(srv.URL), "ten", "task", 0, 10, &p42.MessageStreamConfig{AgentID: "agent-1"})
	defer ms.Close()

	firstBatch, err := ms.ReadBatch(context.Background(), 1, time.Second, "")
	require.NoError(t, err)
	require.Len(t, firstBatch.Events, 1)
	require.False(t, firstBatch.ReachedEnd)

	secondBatch, err := ms.ReadBatch(context.Background(), 1, time.Second, firstBatch.LastEventID)
	require.NoError(t, err)
	require.Empty(t, secondBatch.Events)
	require.True(t, secondBatch.ReachedEnd)
	require.Equal(t, "1", secondBatch.LastEventID)
}
