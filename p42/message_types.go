package p42

import "time"

// MainAgentID is the reserved agent id for the main turn agent.
const MainAgentID = "00000000-0000-0000-0000-000000000000"

// TurnMessage represents an agent or user message exchanged during a turn.
type TurnMessage struct {
	MessageID    string    `json:"MessageID"`
	FromTenantID *string   `json:"FromTenantID,omitempty"`
	FromAgentID  *string   `json:"FromAgentID,omitempty"`
	To           []string  `json:"To"`
	Message      string    `json:"Message"`
	CreatedAt    time.Time `json:"CreatedAt"`
}

// SubAgentCompletionMessage is emitted when a sub-agent completes work.
type SubAgentCompletionMessage struct {
	AgentID           string `json:"AgentID"`
	StillRunning      bool   `json:"StillRunning"`
	CompletionMessage string `json:"CompletionMessage"`
}

// TurnMessageType is the SSE event type emitted by StreamMessages.
type TurnMessageType string

const (
	TurnMessageTypeAgentMessage       TurnMessageType = "AgentMessage"
	TurnMessageTypeUserMessage        TurnMessageType = "UserMessage"
	TurnMessageTypeSubAgentCompletion TurnMessageType = "SubAgentCompletion"
)

// TurnMessageEvent models the polymorphic StreamMessages SSE payload.
// Exactly one of Message or SubAgentCompletion is set for supported event types.
type TurnMessageEvent struct {
	Type               TurnMessageType
	Message            *TurnMessage
	SubAgentCompletion *SubAgentCompletionMessage
}
