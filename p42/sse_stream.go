package p42

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/plan42-ai/concurrency"
	"github.com/plan42-ai/sdk-go/internal/util"
)

// EventBatch describes the result of a bounded event read.
type EventBatch[T any] struct {
	Events      []T
	LastEventID string
	ReachedEnd  bool
}

// SSEStreamEvent is a single parsed SSE event emitted by SSEStream.
type SSEStreamEvent[T any] struct {
	EventID string
	Event   T
}

// SSEConnectFunc opens an SSE stream, optionally resuming from a prior event id.
type SSEConnectFunc func(context.Context, string) (io.ReadCloser, error)

// SSEParseFunc converts a completed SSE event into a typed event value.
type SSEParseFunc[T any] func(context.Context, *SSEEvent) (*T, error)

// SSEEvent represents a parsed raw Server-Sent Event.
type SSEEvent struct {
	EventType string
	Data      string
	ID        *string
	Retry     *int
}

// SSEStream streams parsed events from a Server-Sent Events endpoint.
type SSEStream[T any] struct {
	cg         *concurrency.ContextGroup
	events     chan SSEStreamEvent[T]
	lastID     atomic.Pointer[string]
	retry      time.Duration
	backoff    *util.Backoff
	connect    SSEConnectFunc
	parse      SSEParseFunc[T]
	streamName string
}

type sseEventBuffer struct {
	eventType string
	dataBuf   strings.Builder
	id        *string
	retry     *int
}

func (e *sseEventBuffer) reset() {
	e.eventType = ""
	e.dataBuf.Reset()
	e.id = nil
	e.retry = nil
}

// NewSSEStream creates and starts an SSEStream.
func NewSSEStream[T any](
	buffer int,
	lastEventID string,
	streamName string,
	connect SSEConnectFunc,
	parse SSEParseFunc[T],
) *SSEStream[T] {
	stream := &SSEStream[T]{
		cg:         concurrency.NewContextGroup(),
		events:     make(chan SSEStreamEvent[T], buffer),
		backoff:    util.NewBackoff(100*time.Millisecond, 2*time.Second),
		connect:    connect,
		parse:      parse,
		streamName: streamName,
	}
	stream.storeLastEventID(lastEventID)

	stream.cg.Add(1)
	go stream.run()
	return stream
}

// Events returns a channel that emits parsed events as they are received.
func (s *SSEStream[T]) Events() <-chan SSEStreamEvent[T] { return s.events }

// LastEventID returns the latest SSE event id observed by the stream.
func (s *SSEStream[T]) LastEventID() string {
	lastID := s.lastID.Load()
	if lastID == nil {
		return ""
	}
	return *lastID
}

// ReadBatch reads events until maxEvents have been collected, the timeout expires, or the
// currently available event stream is exhausted. The returned LastEventID can be supplied
// to a future stream to resume from the same position.
func (s *SSEStream[T]) ReadBatch(ctx context.Context, maxEvents int, timeout time.Duration, lastEventID string) (*EventBatch[T], error) {
	if maxEvents <= 0 {
		return nil, errors.New("max events must be greater than zero")
	}

	batchCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		batchCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	events := make([]T, 0, maxEvents)
	for len(events) < maxEvents {
		select {
		case <-batchCtx.Done():
			if errors.Is(batchCtx.Err(), context.DeadlineExceeded) {
				return &EventBatch[T]{
					Events:      events,
					LastEventID: lastEventID,
					ReachedEnd:  false,
				}, nil
			}
			return nil, batchCtx.Err()
		case entry, ok := <-s.events:
			if !ok {
				return &EventBatch[T]{
					Events:      events,
					LastEventID: lastEventID,
					ReachedEnd:  true,
				}, nil
			}
			events = append(events, entry.Event)
			lastEventID = entry.EventID
		}
	}

	return &EventBatch[T]{
		Events:      events,
		LastEventID: lastEventID,
		ReachedEnd:  false,
	}, nil
}

// Close cancels the stream and waits for shutdown.
func (s *SSEStream[T]) Close() error { return s.cg.Close() }

// ShutdownContext waits for the stream to finish with a context.
func (s *SSEStream[T]) ShutdownContext(ctx context.Context) error { return s.cg.WaitContext(ctx) }

// ShutdownTimeout waits for the stream to finish with a timeout.
func (s *SSEStream[T]) ShutdownTimeout(d time.Duration) error { return s.cg.WaitTimeout(d) }

func (s *SSEStream[T]) run() {
	defer s.cg.Done()
	defer s.cg.Cancel()
	defer close(s.events)

	for {
		if err := s.backoff.WaitAtLeast(s.cg.Context(), s.retry); err != nil {
			return
		}

		err := s.connectAndStream(s.cg.Context())
		if err == nil {
			s.backoff.Recover()
			continue
		}
		if errors.Is(err, io.EOF) {
			return
		}
		if s.cg.Context().Err() != nil {
			return
		}
		slog.ErrorContext(s.cg.Context(), s.streamName+": stream error", "error", err)
		s.backoff.Backoff()
	}
}

func (s *SSEStream[T]) connectAndStream(ctx context.Context) error {
	body, err := s.connect(ctx, s.LastEventID())
	if err != nil {
		return err
	}
	if body == nil {
		return io.EOF
	}
	defer body.Close()

	return s.consume(ctx, body)
}

func (s *SSEStream[T]) consume(ctx context.Context, r io.Reader) error {
	br := bufio.NewReader(r)
	event := &sseEventBuffer{}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := br.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			if err := s.processCompleteEvent(ctx, event); err != nil {
				return err
			}
			event.reset()
			continue
		}

		parseSSELine(event, line)
	}
}

func parseSSELine(event *sseEventBuffer, line string) {
	if strings.HasPrefix(line, ":") {
		return
	}

	idx := strings.Index(line, ":")
	if idx == -1 {
		return
	}

	field := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])

	switch field {
	case "event":
		event.eventType = value
	case "data":
		if event.dataBuf.Len() > 0 {
			event.dataBuf.WriteByte('\n')
		}
		event.dataBuf.WriteString(value)
	case "id":
		event.id = util.Pointer(value)
	case "retry":
		if v, err := strconv.Atoi(value); err == nil {
			event.retry = util.Pointer(v)
		}
	}
}

func (s *SSEStream[T]) processCompleteEvent(ctx context.Context, event *sseEventBuffer) error {
	parsed, err := s.parse(ctx, &SSEEvent{
		EventType: event.eventType,
		Data:      event.dataBuf.String(),
		ID:        event.id,
		Retry:     event.retry,
	})
	if err != nil {
		return err
	}
	if parsed == nil {
		return nil
	}

	entry := SSEStreamEvent[T]{Event: *parsed}
	if event.id != nil {
		entry.EventID = *event.id
	}

	select {
	case s.events <- entry:
	case <-ctx.Done():
		return ctx.Err()
	}

	if event.id != nil {
		s.storeLastEventID(*event.id)
	}
	if event.retry != nil {
		s.retry = time.Duration(*event.retry) * time.Millisecond
	}

	return nil
}

func (s *SSEStream[T]) storeLastEventID(lastEventID string) {
	lastID := new(string)
	*lastID = lastEventID
	s.lastID.Store(lastID)
}
