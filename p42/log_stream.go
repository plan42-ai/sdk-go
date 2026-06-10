package p42

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strconv"

	"github.com/plan42-ai/sdk-go/internal/util"
)

// LogStream streams logs for a turn using Server-Sent Events.
type LogStream struct {
	*SSEStream[TurnLog]
}

type LogStreamOption func(*logStreamConfig)

type logStreamConfig struct {
	includeDeleted bool
	workstreamID   *string
	featureFlags   map[string]bool
	delegatedAuth  DelegatedAuthInfo
	lastID         int
}

func WithIncludeDeleted(value bool) LogStreamOption {
	return func(cfg *logStreamConfig) {
		cfg.includeDeleted = value
	}
}

func WithWorkstreamID(workstreamID *string) LogStreamOption {
	return func(cfg *logStreamConfig) {
		cfg.workstreamID = workstreamID
	}
}

func WithFeatureFlags(flags map[string]bool) LogStreamOption {
	return func(cfg *logStreamConfig) {
		cfg.featureFlags = flags
	}
}

func WithLastID(lastID int) LogStreamOption {
	return func(cfg *logStreamConfig) {
		cfg.lastID = lastID
	}
}

func WithDelegatedAuth(delegatedAuth DelegatedAuthInfo) LogStreamOption {
	return func(cfg *logStreamConfig) {
		cfg.delegatedAuth = delegatedAuth
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
	cfg := logStreamConfig{}
	for _, opt := range options {
		opt(&cfg)
	}

	stream := NewSSEStream[TurnLog](
		buffer,
		strconv.Itoa(cfg.lastID),
		"LogStream",
		func(ctx context.Context, lastEventID string) (io.ReadCloser, error) {
			req := &StreamTurnLogsRequest{
				FeatureFlags:      FeatureFlags{FeatureFlags: cfg.featureFlags},
				DelegatedAuthInfo: cfg.delegatedAuth,
				TenantID:          tenantID,
				TaskID:            taskID,
				TurnIndex:         turnIndex,
				IncludeDeleted:    util.Pointer(cfg.includeDeleted),
				WorkstreamID:      cfg.workstreamID,
			}
			if lastEventID != "" && lastEventID != "0" {
				parsedLastID, err := strconv.Atoi(lastEventID)
				if err != nil {
					return nil, err
				}
				req.LastEventID = &parsedLastID
			}
			return client.StreamTurnLogs(ctx, req)
		},
		func(ctx context.Context, event *SSEEvent) (*TurnLog, error) {
			if event.EventType != "log" || event.Data == "" {
				return nil, nil
			}

			var logEntry TurnLog
			if err := json.Unmarshal([]byte(event.Data), &logEntry); err != nil {
				slog.ErrorContext(ctx, "LogStream: failed to decode log", "error", err)
				return nil, nil
			}
			return &logEntry, nil
		},
	)

	return &LogStream{SSEStream: stream}
}
