package eh

import "time"

// ModelType represents the model to use for executing a task.
type ModelType string

const (
	ModelTypeCodexMini     ModelType = "Codex Mini"
	ModelTypeO3            ModelType = "O3"
	ModelTypeO3Pro         ModelType = "O3 Pro"
	ModelTypeClaude4Opus   ModelType = "Claude 4 Opus"
	ModelTypeClaude4Sonnet ModelType = "Claude 4 Sonnet"
)

// TaskState represents the state of a task.
type TaskState string

const (
	TaskStatePending            TaskState = "Pending"
	TaskStateExecuting          TaskState = "Executing"
	TaskStateAwaitingCodeReview TaskState = "Awaiting Code Review"
	TaskStateCompleted          TaskState = "Completed"
)

// RepoInfo contains information about a repository used in a task's environment.
type RepoInfo struct {
	PRLink        *string `json:"PRLink,omitempty"`
	PRID          *string `json:"PRID,omitempty"`
	PRNumber      *int    `json:"PRNumber,omitempty"`
	FeatureBranch string  `json:"FeatureBranch"`
	TargetBranch  string  `json:"TargetBranch"`
}

// Task represents a task returned by the API.
type Task struct {
	TenantID           string               `json:"TenantId"`
	WorkstreamID       *string              `json:"WorkstreamId,omitempty"`
	TaskID             string               `json:"TaskId"`
	Title              string               `json:"Title"`
	EnvironmentID      string               `json:"EnvironmentId"`
	Prompt             string               `json:"Prompt"`
	AfterTaskID        *string              `json:"AfterTaskId,omitempty"`
	Parallel           bool                 `json:"Parallel"`
	Model              ModelType            `json:"Model"`
	AssignedToTenantID *string              `json:"AssignedToTenantId,omitempty"`
	AssignedToAI       bool                 `json:"AssignedToAI"`
	RepoInfo           map[string]*RepoInfo `json:"RepoInfo"`
	State              TaskState            `json:"State"`
	CreatedAt          time.Time            `json:"CreatedAt"`
	UpdatedAt          time.Time            `json:"UpdatedAt"`
	Deleted            bool                 `json:"Deleted"`
	Version            int                  `json:"Version"`
}

// ObjectType returns the object type for ConflictError handling.
func (Task) ObjectType() ObjectType { return ObjectTypeTask }
