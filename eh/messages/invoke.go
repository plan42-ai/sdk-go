package messages

import (
	"encoding/json"
	"time"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
)

type EncryptedInvokeAgentRequest struct {
	CipherText   []byte
	EncryptedCek []byte
	IV           []byte
	Alias        string
	TenantID     string
}

type InvokeAgentRequest struct {
	Task        *eh.Task
	Turn        *eh.Turn
	Environment *eh.Environment
	GithubToken string
	AgentToken  string
	FeedBack    map[string][]PRFeedback
}

func (r *InvokeAgentRequest) Type() MessageType {
	return InvokeAgentRequestMessage
}

func (r *InvokeAgentRequest) GetModel() eh.ModelType {
	if r.Task.Model == nil {
		return eh.ModelTypeGpt5Codex
	}
	return *r.Task.Model
}

func (r InvokeAgentRequest) MarshalJSON() ([]byte, error) {
	var tmp struct {
		Type        MessageType
		Task        *eh.Task
		Turn        *eh.Turn
		Environment *eh.Environment
		GithubToken string
		AgentToken  string
		FeedBack    map[string][]PRFeedback
	}

	tmp.Type = InvokeAgentRequestMessage
	tmp.Task = r.Task
	tmp.Turn = r.Turn
	tmp.Environment = r.Environment
	tmp.GithubToken = r.GithubToken
	tmp.AgentToken = r.AgentToken
	tmp.FeedBack = r.FeedBack

	return json.Marshal(tmp)
}

type PRFeedback struct {
	// ID is the github ID number for the top most feedback item.
	ID string

	// IsResolved indicates whether the feedback thread was marked as resolved.
	IsResolved bool

	// Comments is the list of comments in the feedback thread.
	Comments []Comment
}

type Comment struct {
	// The github login of the user making a comment.
	User string
	// The comment text.
	Body string
	// The date the comment was made.
	Date time.Time

	// DiffHunk shows the diff context associated with the PR feedback.
	DiffHunk string
	// Path is the path to the source file associated with the feedback.
	Path string
	// StartLine is the "new" line-number where the DiffHunk starts.
	StartLine int
	// OrigStartLine is the "old" line-number where the DiffHunk starts.
	OrigStartLine int
	// CommitHash is the commit hash associated with the pr feedback.
	CommitHash string

	// IsMinimized indicates whether the feedback thread has been minimized.
	IsMinimized bool
	// MinimizedReason indicates the reason the feedback thread has been minimized.
	MinimizedReason string
}

type InvokeAgentResponse struct {
	ErrorMessage *string `json:"ErrorMessage,omitempty"`
}

func (r *InvokeAgentResponse) Type() MessageType {
	return InvokeAgentResponseMessage
}

func (r InvokeAgentResponse) MarshalJSON() ([]byte, error) {
	var tmp struct {
		Type         MessageType
		ErrorMessage *string
	}

	tmp.Type = InvokeAgentResponseMessage
	tmp.ErrorMessage = r.ErrorMessage

	return json.Marshal(tmp)
}
