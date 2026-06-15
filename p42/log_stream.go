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

// LogStreamConfig configures a LogStream.
type LogStreamConfig struct {
	IncludeDeleted bool
	WorkstreamID   *string
	AgentID        *string
	FeatureFlags   map[string]bool
	DelegatedAuth  DelegatedAuthInfo
	LastID         int
}

// NewLogStream creates and starts a LogStream.
func NewLogStream(
	client *Client,
	tenantID, taskID string,
	turnIndex int,
	buffer int,
	config *LogStreamConfig,
) *LogStream {
	cfg := &LogStreamConfig{}
	if config != nil {
		cfg = config
	}

	stream := NewSSEStream[TurnLog](
		buffer,
		strconv.Itoa(cfg.LastID),
		"LogStream",
		func(ctx context.Context, lastEventID string) (io.ReadCloser, error) {
			req := &StreamTurnLogsRequest{
				FeatureFlags:      FeatureFlags{FeatureFlags: cfg.FeatureFlags},
				DelegatedAuthInfo: cfg.DelegatedAuth,
				TenantID:          tenantID,
				TaskID:            taskID,
				TurnIndex:         turnIndex,
				IncludeDeleted:    util.Pointer(cfg.IncludeDeleted),
				WorkstreamID:      cfg.WorkstreamID,
				AgentID:           cfg.AgentID,
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
