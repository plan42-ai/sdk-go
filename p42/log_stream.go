package p42

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/plan42-ai/concurrency"
	"github.com/plan42-ai/sdk-go/internal/util"
)

// LogStream streams logs for a turn using Server-Sent Events.
type LogStream struct {
	cg             *concurrency.ContextGroup
	client         *Client
	tenantID       string
	taskID         string
	turnIndex      int
	includeDeleted bool
	workstreamID   *string
	featureFlags   map[string]bool
	delegatedAuth  DelegatedAuthInfo
	logs           chan TurnLog
	mu             sync.RWMutex
	lastID         int
	retry          time.Duration
	backoff        *util.Backoff
	endOfAvailable bool
}

type LogStreamOption func(s *LogStream)

// LogBatchOptions controls how ReadBatch bounds a model-facing log read.
type LogBatchOptions struct {
	// MaxEvents stops the batch after collecting this many log events.
	// Values <= 0 mean no event-count limit.
	MaxEvents int
	// Timeout stops the batch after the duration elapses.
	// Values <= 0 mean no timeout limit.
	Timeout time.Duration
}

// LogBatchResult reports the outcome of a bounded log read.
type LogBatchResult struct {
	Logs             []TurnLog
	LastEventID      int
	EndOfAvailable   bool
	StoppedByLimit   bool
	StoppedByTimeout bool
}

func WithIncludeDeleted(value bool) LogStreamOption {
	return func(s *LogStream) {
		s.includeDeleted = value
	}
}

func WithWorkstreamID(workstreamID *string) LogStreamOption {
	return func(s *LogStream) {
		s.workstreamID = workstreamID
	}
}

func WithFeatureFlags(flags map[string]bool) LogStreamOption {
	return func(s *LogStream) {
		s.featureFlags = flags
	}
}

func WithLastID(lastID int) LogStreamOption {
	return func(s *LogStream) {
		s.lastID = lastID
	}
}

func WithDelegatedAuth(delegatedAuth DelegatedAuthInfo) LogStreamOption {
	return func(s *LogStream) {
		s.delegatedAuth = delegatedAuth
	}
}

// NewLogStream creates and starts a LogStream.
func NewLogStream(
	client *Client,
	tenantID, taskID string,
	turnIndex int,
	buffer int,
	options ...LogStreamOption,
) *LogStream {
	ls := &LogStream{
		cg:        concurrency.NewContextGroup(),
		client:    client,
		tenantID:  tenantID,
		taskID:    taskID,
		turnIndex: turnIndex,
		logs:      make(chan TurnLog, buffer),
		backoff:   util.NewBackoff(100*time.Millisecond, 2*time.Second),
	}

	for _, opt := range options {
		opt(ls)
	}

	ls.cg.Add(1)
	go ls.run()
	return ls
}

// Logs returns a channel that emits TurnLog entries as they are received.
func (l *LogStream) Logs() <-chan TurnLog { return l.logs }

// LastEventID returns the most recently observed SSE event ID.
func (l *LogStream) LastEventID() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lastID
}

// EndOfAvailable reports whether the stream has reached the end of currently available logs.
func (l *LogStream) EndOfAvailable() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.endOfAvailable
}

// ReadBatch collects logs until a configured limit is reached, a timeout expires,
// or the stream reaches the end of currently available logs.
func (l *LogStream) ReadBatch(ctx context.Context, opts LogBatchOptions) (*LogBatchResult, error) {
	result := &LogBatchResult{}

	if opts.MaxEvents <= 0 && opts.Timeout <= 0 {
		for {
			if l.EndOfAvailable() {
				result.LastEventID = l.LastEventID()
				result.EndOfAvailable = true
				return result, nil
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case log, ok := <-l.logs:
				if !ok {
					result.LastEventID = l.LastEventID()
					result.EndOfAvailable = l.EndOfAvailable()
					return result, nil
				}
				result.Logs = append(result.Logs, log)
			}
		}
	}

	var timeoutCh <-chan time.Time
	if opts.Timeout > 0 {
		timer := time.NewTimer(opts.Timeout)
		defer timer.Stop()
		timeoutCh = timer.C
	}

	for {
		if opts.MaxEvents > 0 && len(result.Logs) >= opts.MaxEvents {
			result.StoppedByLimit = true
			result.LastEventID = l.LastEventID()
			result.EndOfAvailable = l.EndOfAvailable()
			return result, nil
		}
		if l.EndOfAvailable() {
			result.LastEventID = l.LastEventID()
			result.EndOfAvailable = true
			return result, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeoutCh:
			result.StoppedByTimeout = true
			result.LastEventID = l.LastEventID()
			result.EndOfAvailable = l.EndOfAvailable()
			return result, nil
		case log, ok := <-l.logs:
			if !ok {
				result.LastEventID = l.LastEventID()
				result.EndOfAvailable = l.EndOfAvailable()
				return result, nil
			}
			result.Logs = append(result.Logs, log)
		}
	}
}

// Close cancels the stream and waits for shutdown.
func (l *LogStream) Close() error { return l.cg.Close() }

// ShutdownContext waits for the stream to finish with a context.
func (l *LogStream) ShutdownContext(ctx context.Context) error { return l.cg.WaitContext(ctx) }

// ShutdownTimeout waits for the stream to finish with a timeout.
func (l *LogStream) ShutdownTimeout(d time.Duration) error { return l.cg.WaitTimeout(d) }

func (l *LogStream) run() {
	defer l.cg.Done()
	defer l.cg.Cancel()
	defer close(l.logs)

	for {
		if err := l.backoff.WaitAtLeast(l.cg.Context(), l.retry); err != nil {
			return
		}

		err := l.connectAndStream(l.cg.Context())
		if err == nil {
			l.backoff.Recover()
			continue
		}
		if errors.Is(err, io.EOF) {
			// no more logs
			l.setEndOfAvailable(true)
			return
		}
		if l.cg.Context().Err() != nil {
			return
		}
		slog.ErrorContext(l.cg.Context(), "LogStream: stream error", "error", err)
		l.backoff.Backoff()
	}
}

func (l *LogStream) connectAndStream(ctx context.Context) error {
	req := &StreamTurnLogsRequest{
		FeatureFlags: FeatureFlags{
			FeatureFlags: l.featureFlags,
		},
		DelegatedAuthInfo: l.delegatedAuth,
		TenantID:          l.tenantID,
		TaskID:            l.taskID,
		TurnIndex:         l.turnIndex,
		IncludeDeleted:    util.Pointer(l.includeDeleted),
		WorkstreamID:      l.workstreamID,
	}
	if l.lastID != 0 {
		req.LastEventID = &l.lastID
	}

	req.FeatureFlags = FeatureFlags{FeatureFlags: l.featureFlags}

	body, err := l.client.StreamTurnLogs(ctx, req)
	if err != nil {
		return err
	}
	if body == nil {
		return io.EOF
	}
	l.setEndOfAvailable(false)
	defer body.Close()

	return l.consume(ctx, body)
}

// sseEvent represents a Server-Sent Event being parsed.
type sseEvent struct {
	eventType string
	dataBuf   strings.Builder
	id        *int
	retry     *int
}

// reset clears the event state for reuse.
func (e *sseEvent) reset() {
	e.eventType = ""
	e.dataBuf.Reset()
	e.id = nil
	e.retry = nil
}

func (l *LogStream) consume(ctx context.Context, r io.Reader) error {
	br := bufio.NewReader(r)
	event := &sseEvent{}

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

		// Empty line indicates end of an event
		if line == "" {
			if err := l.processCompleteEvent(ctx, event); err != nil {
				return err
			}
			event.reset()
			continue
		}

		l.parseSSELine(event, line)
	}
}

// parseSSELine processes a single line of an SSE stream.
func (l *LogStream) parseSSELine(event *sseEvent, line string) {
	// Skip comments
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
		if v, err := strconv.Atoi(value); err == nil {
			event.id = util.Pointer(v)
		}
	case "retry":
		if v, err := strconv.Atoi(value); err == nil {
			event.retry = util.Pointer(v)
		}
	}
}

// processCompleteEvent handles a completed SSE event.
func (l *LogStream) processCompleteEvent(ctx context.Context, event *sseEvent) error {
	if event.eventType != "log" || event.dataBuf.Len() == 0 {
		return nil
	}

	var logEntry TurnLog
	if err := json.Unmarshal([]byte(event.dataBuf.String()), &logEntry); err != nil {
		slog.ErrorContext(ctx, "LogStream: failed to decode log", "error", err)
		return nil
	}

	select {
	case l.logs <- logEntry:
	case <-ctx.Done():
		return ctx.Err()
	}

	if event.id != nil {
		l.setLastID(*event.id)
	}
	if event.retry != nil {
		l.retry = time.Duration(*event.retry) * time.Millisecond
	}

	return nil
}

func (l *LogStream) setLastID(lastID int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lastID = lastID
}

func (l *LogStream) setEndOfAvailable(value bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.endOfAvailable = value
}
