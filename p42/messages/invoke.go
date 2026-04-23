package messages

import (
	"encoding/json"
	"time"

	"github.com/plan42-ai/sdk-go/p42"
)

type EncryptedInvokeAgentRequest struct {
	CipherText   []byte
	EncryptedCek []byte
	IV           []byte
	Alias        string
	TenantID     string
}

type InvokeAgentRequest struct {
	Task                      *p42.Task
	Turn                      *p42.Turn
	Environment               *p42.Environment
	Tenant                    *TenantInfo
	GithubToken               *string
	GithubURL                 *string
	PrivateGithubConnectionID *string
	AgentToken                string
	FeedBack                  *map[string][]PRFeedback
}

func (r *InvokeAgentRequest) Type() MessageType {
	return InvokeAgentRequestMessage
}

func (r *InvokeAgentRequest) GetModel() p42.ModelType {
	if r.Task.Model == nil {
		return p42.ModelTypeGpt51Codex
	}
	return *r.Task.Model
}

func (r InvokeAgentRequest) MarshalJSON() ([]byte, error) {
	var tmp struct {
		Type                      MessageType
		Task                      *p42.Task
		Turn                      *p42.Turn
		Environment               *p42.Environment
		Tenant                    *TenantInfo
		GithubToken               *string
		GithubURL                 *string
		PrivateGithubConnectionID *string
		AgentToken                string
		FeedBack                  *map[string][]PRFeedback
	}

	tmp.Type = InvokeAgentRequestMessage
	tmp.Task = r.Task
	tmp.Turn = r.Turn
	tmp.Environment = r.Environment
	tmp.Tenant = r.Tenant
	tmp.GithubToken = r.GithubToken
	tmp.GithubURL = r.GithubURL
	tmp.PrivateGithubConnectionID = r.PrivateGithubConnectionID
	tmp.AgentToken = r.AgentToken
	tmp.FeedBack = r.FeedBack

	return json.Marshal(tmp)
}

// TenantInfo exposes tenant-level configuration to the agent.
type TenantInfo struct {
	TenantID                  string  `json:"TenantID"`
	DefaultGithubConnectionID *string `json:"DefaultGithubConnectionID,omitempty"`
	DefaultRunnerID           *string `json:"DefaultRunnerID,omitempty"`
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
