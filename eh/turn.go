package eh

import "time"

// Turn represents a single execution turn of a task.
type Turn struct {
	TenantID           string    `json:"TenantId"`
	TaskID             string    `json:"TaskId"`
	TurnIndex          int       `json:"TurnIndex"`
	Prompt             string    `json:"Prompt"`
	PreviousResponseID *string   `json:"PreviousResponseID,omitempty"`
	BaselineCommitHash *string   `json:"BaselineCommitHash,omitempty"`
	LastCommitHash     *string   `json:"LastCommitHash,omitempty"`
	Status             string    `json:"Status"`
	OutputMessage      *string   `json:"OutputMessage,omitempty"`
	ErrorMessage       *string   `json:"ErrorMessage,omitempty"`
	CreatedAt          time.Time `json:"CreatedAt"`
	UpdatedAt          time.Time `json:"UpdatedAt"`
	Version            int       `json:"Version"`
}

// ObjectType returns the object type for ConflictError handling.
func (Turn) ObjectType() ObjectType { return ObjectTypeTurn }
