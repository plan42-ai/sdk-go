package eh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ModelType represents the model to use for executing a task.
type ModelType string

const (
	ModelTypeCodexMini     ModelType = "Codex Mini"
	ModelTypeO3            ModelType = "O3"
	ModelTypeO3Pro         ModelType = "O3 Pro"
	ModelTypeGpt5          ModelType = "GPT-5"
	ModelTypeGpt5Codex     ModelType = "GPT-5 Codex"
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
	EnvironmentID      *string              `json:"EnvironmentId"`
	Prompt             string               `json:"Prompt"`
	Parallel           bool                 `json:"Parallel"`
	Model              *ModelType           `json:"Model"`
	AssignedToTenantID *string              `json:"AssignedToTenantId,omitempty"`
	AssignedToAI       bool                 `json:"AssignedToAI"`
	TaskNumber         *int                 `json:"TaskNumber,omitempty"`
	RepoInfo           map[string]*RepoInfo `json:"RepoInfo"`
	State              TaskState            `json:"State"`
	CreatedAt          time.Time            `json:"CreatedAt"`
	UpdatedAt          time.Time            `json:"UpdatedAt"`
	Deleted            bool                 `json:"Deleted"`
	Version            int                  `json:"Version"`
}

// ObjectType returns the object type for ConflictError handling.
func (Task) ObjectType() ObjectType { return ObjectTypeTask }

// GetTaskRequest is the request payload for GetTask.
type GetTaskRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID       string `json:"-"`
	TaskID         string `json:"-"`
	IncludeDeleted *bool  `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *GetTaskRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "TaskID":
		return r.TaskID, true
	case "IncludeDeleted":
		return evalNullable(r.IncludeDeleted)
	default:
		return nil, false
	}
}

// GetTask retrieves a task by ID.
// nolint:dupl
func (c *Client) GetTask(ctx context.Context, req *GetTaskRequest) (*Task, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.TaskID == "" {
		return nil, fmt.Errorf("task id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "tasks", url.PathEscape(req.TaskID))
	q := u.Query()
	if req.IncludeDeleted != nil {
		q.Set("includeDeleted", strconv.FormatBool(*req.IncludeDeleted))
	}
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	processFeatureFlags(httpReq, req.FeatureFlags)

	if err := c.authenticate(req.DelegatedAuthInfo, httpReq); err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var out Task
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateTaskRequest is the request payload for CreateTask.
type CreateTaskRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID      string               `json:"-"`
	TaskID        string               `json:"-"`
	Title         string               `json:"Title"`
	EnvironmentID *string              `json:"EnvironmentId,omitempty"`
	Prompt        string               `json:"Prompt"`
	Model         *ModelType           `json:"Model,omitempty"`
	RepoInfo      map[string]*RepoInfo `json:"RepoInfo"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *CreateTaskRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "TaskID":
		return r.TaskID, true
	case "Title":
		return r.Title, true
	case "EnvironmentID":
		return r.EnvironmentID, true
	case "Prompt":
		return r.Prompt, true
	case "Model":
		return evalNullable(r.Model)
	case "RepoInfo":
		return r.RepoInfo, true
	default:
		return nil, false
	}
}

// CreateTask creates a new task.
// nolint:dupl
func (c *Client) CreateTask(ctx context.Context, req *CreateTaskRequest) (*Task, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.TaskID == "" {
		return nil, fmt.Errorf("task id is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "tasks", url.PathEscape(req.TaskID))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	processFeatureFlags(httpReq, req.FeatureFlags)

	if err := c.authenticate(req.DelegatedAuthInfo, httpReq); err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, decodeError(resp)
	}

	var out Task
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateWorkstreamTaskRequest is the request payload for CreateWorkstreamTask.
type CreateWorkstreamTaskRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID           string               `json:"-"`
	WorkstreamID       string               `json:"-"`
	TaskID             string               `json:"-"`
	Title              string               `json:"Title"`
	EnvironmentID      *string              `json:"EnvironmentId,omitempty"`
	Prompt             *string              `json:"Prompt,omitempty"`
	Parallel           *bool                `json:"Parallel,omitempty"`
	Model              *ModelType           `json:"Model,omitempty"`
	AssignedToTenantID *string              `json:"AssignedToTenantId,omitempty"`
	AssignedToAI       bool                 `json:"AssignedToAI"`
	RepoInfo           map[string]*RepoInfo `json:"RepoInfo,omitempty"`
	State              *TaskState           `json:"State,omitempty"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *CreateWorkstreamTaskRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "WorkstreamID":
		return r.WorkstreamID, true
	case "TaskID":
		return r.TaskID, true
	case "Title":
		return r.Title, true
	case "EnvironmentID":
		return evalNullable(r.EnvironmentID)
	case "Prompt":
		return evalNullable(r.Prompt)
	case "Parallel":
		return evalNullable(r.Parallel)
	case "Model":
		return evalNullable(r.Model)
	case "AssignedToTenantID":
		return evalNullable(r.AssignedToTenantID)
	case "AssignedToAI":
		return r.AssignedToAI, true
	case "RepoInfo":
		return r.RepoInfo, true
	case "State":
		return evalNullable(r.State)
	default:
		return nil, false
	}
}

// CreateWorkstreamTask creates a new task inside a workstream.
func (c *Client) CreateWorkstreamTask(ctx context.Context, req *CreateWorkstreamTaskRequest) (*Task, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.WorkstreamID == "" {
		return nil, fmt.Errorf("workstream id is required")
	}
	if req.TaskID == "" {
		return nil, fmt.Errorf("task id is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"workstreams",
		url.PathEscape(req.WorkstreamID),
		"tasks",
		url.PathEscape(req.TaskID),
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	processFeatureFlags(httpReq, req.FeatureFlags)

	err = c.authenticate(req.DelegatedAuthInfo, httpReq)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, decodeError(resp)
	}

	var out Task
	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

// UpdateTaskRequest is the request payload for UpdateTask.
type UpdateTaskRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID string                `json:"-"`
	TaskID   string                `json:"-"`
	Version  int                   `json:"-"`
	Title    *string               `json:"Title,omitempty"`
	Prompt   *string               `json:"Prompt,omitempty"`
	Model    *ModelType            `json:"Model,omitempty"`
	RepoInfo *map[string]*RepoInfo `json:"RepoInfo,omitempty"`
	Deleted  *bool                 `json:"Deleted,omitempty"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *UpdateTaskRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "TaskID":
		return r.TaskID, true
	case "Version":
		return r.Version, true
	case "Title":
		return evalNullable(r.Title)
	case "Prompt":
		return evalNullable(r.Prompt)
	case "Model":
		return evalNullable(r.Model)
	case "RepoInfo":
		return evalNullable(r.RepoInfo)
	case "Deleted":
		return evalNullable(r.Deleted)
	default:
		return nil, false
	}
}

// UpdateTask updates an existing task.
// nolint: dupl
func (c *Client) UpdateTask(ctx context.Context, req *UpdateTaskRequest) (*Task, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.TaskID == "" {
		return nil, fmt.Errorf("task id is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "tasks", url.PathEscape(req.TaskID))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("If-Match", strconv.Itoa(req.Version))
	processFeatureFlags(httpReq, req.FeatureFlags)

	if err := c.authenticate(req.DelegatedAuthInfo, httpReq); err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var out Task
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteTaskRequest is the request payload for DeleteTask.
type DeleteTaskRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID string `json:"-"`
	TaskID   string `json:"-"`
	Version  int    `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *DeleteTaskRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "TaskID":
		return r.TaskID, true
	case "Version":
		return r.Version, true
	default:
		return nil, false
	}
}

// DeleteTask soft-deletes a task.
// nolint: dupl
func (c *Client) DeleteTask(ctx context.Context, req *DeleteTaskRequest) error {
	if req == nil {
		return fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return fmt.Errorf("tenant id is required")
	}
	if req.TaskID == "" {
		return fmt.Errorf("task id is required")
	}
	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "tasks", url.PathEscape(req.TaskID))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, u.String(), nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("If-Match", strconv.Itoa(req.Version))
	processFeatureFlags(httpReq, req.FeatureFlags)

	if err := c.authenticate(req.DelegatedAuthInfo, httpReq); err != nil {
		return err
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return decodeError(resp)
	}
	return nil
}

// ListTasksRequest is the request for ListTasks.
type ListTasksRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID       string
	MaxResults     *int
	Token          *string
	IncludeDeleted *bool
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *ListTasksRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "MaxResults":
		return evalNullable(r.MaxResults)
	case "Token":
		return evalNullable(r.Token)
	case "IncludeDeleted":
		return evalNullable(r.IncludeDeleted)
	default:
		return nil, false
	}
}

// ListTasksResponse is the response from ListTasks.
type ListTasksResponse struct {
	Tasks     []Task  `json:"Tasks"`
	NextToken *string `json:"NextToken"`
}

// ListTasks retrieves the tasks for a tenant.
// nolint: dupl
func (c *Client) ListTasks(ctx context.Context, req *ListTasksRequest) (*ListTasksResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "tasks")
	q := u.Query()

	if req.MaxResults != nil {
		q.Set("maxResults", strconv.Itoa(*req.MaxResults))
	}
	if req.Token != nil {
		q.Set("token", *req.Token)
	}
	if req.IncludeDeleted != nil {
		q.Set("includeDeleted", strconv.FormatBool(*req.IncludeDeleted))
	}
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	processFeatureFlags(httpReq, req.FeatureFlags)

	if err := c.authenticate(req.DelegatedAuthInfo, httpReq); err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var out ListTasksResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MoveTaskRequest is the request payload for MoveTask.
type MoveTaskRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID                     string `json:"-"`
	TaskID                       string `json:"-"`
	DestinationWorkstreamID      string `json:"DestinationWorkstreamID"`
	TaskVersion                  int    `json:"TaskVersion"`
	SourceWorkstreamVersion      int    `json:"SourceWorkstreamVersion"`
	DestinationWorkstreamVersion int    `json:"DestinationWorkstreamVersion"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *MoveTaskRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "TaskID":
		return r.TaskID, true
	case "DestinationWorkstreamID":
		return r.DestinationWorkstreamID, true
	case "TaskVersion":
		return r.TaskVersion, true
	case "SourceWorkstreamVersion":
		return r.SourceWorkstreamVersion, true
	case "DestinationWorkstreamVersion":
		return r.DestinationWorkstreamVersion, true
	default:
		return nil, false
	}
}

// MoveTaskResponse is the response payload for MoveTask.
type MoveTaskResponse struct {
	Task                  Task       `json:"Task"`
	SourceWorkstream      Workstream `json:"SourceWorkstream"`
	DestinationWorkstream Workstream `json:"DestinationWorkstream"`
}

// MoveTask moves a task from one workstream to another.
func (c *Client) MoveTask(ctx context.Context, req *MoveTaskRequest) (*MoveTaskResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.TaskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	if req.DestinationWorkstreamID == "" {
		return nil, fmt.Errorf("destination workstream id is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "tasks", url.PathEscape(req.TaskID), "move")

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	processFeatureFlags(httpReq, req.FeatureFlags)

	if err := c.authenticate(req.DelegatedAuthInfo, httpReq); err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var out MoveTaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
