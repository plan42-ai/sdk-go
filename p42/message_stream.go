package p42

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
)

// MessageStream streams turn messages using Server-Sent Events.
type MessageStream struct {
	*SSEStream[TurnMessageEvent]
}

// MessageStreamConfig configures a MessageStream.
type MessageStreamConfig struct {
	FeatureFlags  map[string]bool
	DelegatedAuth DelegatedAuthInfo
	LastEventID   string
	AgentID       string
}

// NewMessageStream creates and starts a MessageStream.
func NewMessageStream(
	client *Client,
	tenantID, taskID string,
	turnIndex int,
	buffer int,
	config *MessageStreamConfig,
) *MessageStream {
	cfg := &MessageStreamConfig{}
	if config != nil {
		cfg = config
	}

	stream := NewSSEStream[TurnMessageEvent](
		buffer,
		cfg.LastEventID,
		"MessageStream",
		func(ctx context.Context, lastEventID string) (io.ReadCloser, error) {
			req := &StreamMessagesRequest{
				FeatureFlags:      FeatureFlags{FeatureFlags: cfg.FeatureFlags},
				DelegatedAuthInfo: cfg.DelegatedAuth,
				TenantID:          tenantID,
				TaskID:            taskID,
				TurnIndex:         turnIndex,
				AgentID:           cfg.AgentID,
			}
			if lastEventID != "" {
				req.LastEventID = &lastEventID
			}
			return client.StreamMessages(ctx, req)
		},
		func(ctx context.Context, event *SSEEvent) (*TurnMessageEvent, error) {
			parsed, err := decodeTurnMessageEvent(event)
			if err != nil {
				slog.ErrorContext(ctx, "MessageStream: failed to decode message", "event_type", event.EventType, "error", err)
				return nil, nil
			}
			return parsed, nil
		},
	)

	return &MessageStream{SSEStream: stream}
}

func decodeTurnMessageEvent(event *SSEEvent) (*TurnMessageEvent, error) {
	if event.EventType == "" || event.Data == "" {
		return nil, nil
	}

	out := &TurnMessageEvent{Type: TurnMessageType(event.EventType)}
	switch out.Type {
	case TurnMessageTypeAgentMessage, TurnMessageTypeUserMessage:
		var msg TurnMessage
		if err := json.Unmarshal([]byte(event.Data), &msg); err != nil {
			return nil, err
		}
		out.Message = &msg
	case TurnMessageTypeSubAgentCompletion:
		var msg SubAgentCompletionMessage
		if err := json.Unmarshal([]byte(event.Data), &msg); err != nil {
			return nil, err
		}
		out.SubAgentCompletion = &msg
	default:
		return nil, nil
	}

	return out, nil
}
