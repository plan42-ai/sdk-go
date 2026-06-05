package p42

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

// Turn represents a single execution turn of a task.
type Turn struct {
	TenantID           string                `json:"TenantId"`
	WorkstreamID       *string               `json:"WorkstreamId,omitempty"`
	TaskID             string                `json:"TaskId"`
	TurnIndex          int                   `json:"TurnIndex"`
	Prompt             string                `json:"Prompt"`
	ReasoningLevel     *ReasoningLevel       `json:"ReasoningLevel,omitempty"`
	PreviousResponseID *string               `json:"PreviousResponseID,omitempty"`
	CommitInfo         map[string]CommitInfo `json:"CommitInfo"`
	Status             string                `json:"Status"`
	OutputMessage      *string               `json:"OutputMessage,omitempty"`
	ErrorMessage       *string               `json:"ErrorMessage,omitempty"`
	CreatedAt          time.Time             `json:"CreatedAt"`
	UpdatedAt          time.Time             `json:"UpdatedAt"`
	CompletedAt        *time.Time            `json:"CompletedAt,omitempty"`
	Version            int                   `json:"Version"`
}

// ObjectType returns the object type for ConflictError handling.
func (Turn) ObjectType() ObjectType { return ObjectTypeTurn }

type CommitInfo struct {
	BaselineCommitHash *string `json:"BaselineCommitHash"`
	LastCommitHash     *string `json:"LastCommitHash"`
}

// CreateTurnRequest is the request payload for CreateTurn.
type CreateTurnRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID          string          `json:"-"`
	TaskID            string          `json:"-"`
	TurnIndex         int             `json:"-"`
	TaskVersion       int             `json:"-"`
	Prompt            string          `json:"Prompt"`
	ReasoningLevel    *ReasoningLevel `json:"ReasoningLevel,omitempty"`
	WorkstreamID      *string         `json:"WorkstreamID,omitempty"`
	AdditionalFileIDs []string        `json:"AdditionalFileIDs,omitempty"`
}

// nolint: goconst
func (c *CreateTurnRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return c.TenantID, true
	case "TaskID":
		return c.TaskID, true
	case "TurnIndex":
		return c.TurnIndex, true
	case "TaskVersion":
		return c.TaskVersion, true
	case "Prompt":
		return c.Prompt, true
	case "ReasoningLevel":
		return EvalNullable(c.ReasoningLevel)
	case "WorkstreamID":
		return EvalNullable(c.WorkstreamID)
	case "AdditionalFileIDs":
		return c.AdditionalFileIDs, true
	default:
		return nil, false
	}
}

// CreateTurn creates a new turn for a task.
func (c *Client) CreateTurn(ctx context.Context, req *CreateTurnRequest) (*Turn, error) {
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

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"tasks",
		url.PathEscape(req.TaskID),
		"turns",
		strconv.Itoa(req.TurnIndex),
	)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	if req.TaskVersion != 0 {
		httpReq.Header.Set("If-Match", strconv.Itoa(req.TaskVersion))
	}
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

	var out Turn
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTurnRequest is the request payload for GetTurn.
type GetTurnRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID       string  `json:"-"`
	TaskID         string  `json:"-"`
	TurnIndex      int     `json:"-"`
	WorkstreamID   *string `json:"-"`
	IncludeDeleted *bool   `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *GetTurnRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "TaskID":
		return r.TaskID, true
	case "TurnIndex":
		return r.TurnIndex, true
	case "WorkstreamID":
		return EvalNullable(r.WorkstreamID)
	case "IncludeDeleted":
		return EvalNullable(r.IncludeDeleted)
	default:
		return nil, false
	}
}

// GetTurn retrieves a specific turn for a task.
// nolint:dupl
func (c *Client) GetTurn(ctx context.Context, req *GetTurnRequest) (*Turn, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.TaskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	if req.TurnIndex < 0 {
		return nil, fmt.Errorf("turn index is required")
	}
	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"tasks",
		url.PathEscape(req.TaskID),
		"turns",
		strconv.Itoa(req.TurnIndex),
	)
	q := u.Query()
	if req.WorkstreamID != nil {
		q.Set("workstreamID", *req.WorkstreamID)
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

	var out Turn
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetLastTurnRequest is the request payload for GetLastTurn.
type GetLastTurnRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID       string  `json:"-"`
	TaskID         string  `json:"-"`
	WorkstreamID   *string `json:"-"`
	IncludeDeleted *bool   `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *GetLastTurnRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "TaskID":
		return r.TaskID, true
	case "WorkstreamID":
		return EvalNullable(r.WorkstreamID)
	case "IncludeDeleted":
		return EvalNullable(r.IncludeDeleted)
	default:
		return nil, false
	}
}

// GetLastTurn retrieves the last turn for a task.
func (c *Client) GetLastTurn(ctx context.Context, req *GetLastTurnRequest) (*Turn, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.TaskID == "" {
		return nil, fmt.Errorf("task id is required")
	}

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"tasks",
		url.PathEscape(req.TaskID),
		"turns",
		"last",
	)
	q := u.Query()
	if req.WorkstreamID != nil {
		q.Set("workstreamID", *req.WorkstreamID)
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

	var out Turn
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateTurnRequest is the request payload for UpdateTurn.
type UpdateTurnRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID           string                 `json:"-"`
	TaskID             string                 `json:"-"`
	TurnIndex          int                    `json:"-"`
	Version            int                    `json:"-"`
	PreviousResponseID *string                `json:"PreviousResponseID,omitempty"`
	CommitInfo         *map[string]CommitInfo `json:"CommitInfo,omitempty"`
	WorkstreamID       *string                `json:"WorkstreamID,omitempty"`
	Status             *string                `json:"Status,omitempty"`
	OutputMessage      *string                `json:"OutputMessage,omitempty"`
	ErrorMessage       *string                `json:"ErrorMessage,omitempty"`
	CompletedAt        *time.Time             `json:"CompletedAt,omitempty"`
	UploadedFileIDs    []string               `json:"UploadedFileIDs,omitempty"`
}

// IsEmptyUpdate reports whether the request contains any turn mutations.
func (r *UpdateTurnRequest) IsEmptyUpdate() bool {
	if r == nil {
		return true
	}
	return r.PreviousResponseID == nil &&
		r.CommitInfo == nil &&
		r.Status == nil &&
		r.OutputMessage == nil &&
		r.ErrorMessage == nil &&
		r.CompletedAt == nil &&
		len(r.UploadedFileIDs) == 0
}

// GetVersion returns the optimistic concurrency control version for the request.
func (r *UpdateTurnRequest) GetVersion() int {
	if r == nil {
		return 0
	}
	return r.Version
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *UpdateTurnRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "TaskID":
		return r.TaskID, true
	case "TurnIndex":
		return r.TurnIndex, true
	case "Version":
		return r.Version, true
	case "PreviousResponseID":
		return EvalNullable(r.PreviousResponseID)
	case "CommitInfo":
		return EvalNullable(r.CommitInfo)
	case "WorkstreamID":
		return EvalNullable(r.WorkstreamID)
	case "Status":
		return EvalNullable(r.Status)
	case "OutputMessage":
		return EvalNullable(r.OutputMessage)
	case "ErrorMessage":
		return EvalNullable(r.ErrorMessage)
	case "CompletedAt":
		return EvalNullable(r.CompletedAt)
	case "UploadedFileIDs":
		return r.UploadedFileIDs, true
	default:
		return nil, false
	}
}

// UpdateTurn updates an existing turn for a task.
func (c *Client) UpdateTurn(ctx context.Context, req *UpdateTurnRequest) (*Turn, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.TaskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	if req.TurnIndex < 0 {
		return nil, fmt.Errorf("turn index is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"tasks",
		url.PathEscape(req.TaskID),
		"turns",
		strconv.Itoa(req.TurnIndex),
	)
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

	var out Turn
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListTurnsRequest is the request payload for ListTurns.
type ListTurnsRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID       string
	TaskID         string
	WorkstreamID   *string
	MaxResults     *int
	Token          *string
	IncludeDeleted *bool
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *ListTurnsRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "TaskID":
		return r.TaskID, true
	case "WorkstreamID":
		return EvalNullable(r.WorkstreamID)
	case "MaxResults":
		return EvalNullable(r.MaxResults)
	case "Token":
		return EvalNullable(r.Token)
	case "IncludeDeleted":
		return EvalNullable(r.IncludeDeleted)
	default:
		return nil, false
	}
}

// ListTurns retrieves the turns for a task.
// nolint: dupl
func (c *Client) ListTurns(ctx context.Context, req *ListTurnsRequest) (*List[Turn], error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.TaskID == "" {
		return nil, fmt.Errorf("task id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "tasks", url.PathEscape(req.TaskID), "turns")
	q := u.Query()
	if req.WorkstreamID != nil {
		q.Set("workstreamID", *req.WorkstreamID)
	}
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

	var out List[Turn]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListActiveTurnsRequest contains parameters for ListActiveTurns.
type ListActiveTurnsRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	MaxResults   *int
	Token        *string
	Partition    int
	MinUpdatedAt *time.Time
	MaxUpdatedAt time.Time
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *ListActiveTurnsRequest) GetField(name string) (any, bool) {
	switch name {
	case "MaxResults":
		return EvalNullable(r.MaxResults)
	case "Token":
		return EvalNullable(r.Token)
	case "Partition":
		return r.Partition, true
	case "MinUpdatedAt":
		return EvalNullable(r.MinUpdatedAt)
	case "MaxUpdatedAt":
		return r.MaxUpdatedAt, true
	default:
		return nil, false
	}
}

// ListActiveTurns retrieves active turns across all tenants and tasks.
// nolint: dupl
func (c *Client) ListActiveTurns(ctx context.Context, req *ListActiveTurnsRequest) (*List[Turn], error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.Partition < 0 {
		return nil, fmt.Errorf("partition must be non-negative")
	}
	if req.MaxUpdatedAt.IsZero() {
		return nil, fmt.Errorf("max updated at is required")
	}

	u := c.BaseURL.JoinPath("v1", "turns")
	q := u.Query()
	if req.MaxResults != nil {
		q.Set("maxResults", strconv.Itoa(*req.MaxResults))
	}
	if req.Token != nil {
		q.Set("token", *req.Token)
	}
	q.Set("partition", strconv.Itoa(req.Partition))
	if req.MinUpdatedAt != nil {
		q.Set("minUpdatedAt", req.MinUpdatedAt.UTC().Format(time.RFC3339Nano))
	}
	q.Set("maxUpdatedAt", req.MaxUpdatedAt.UTC().Format(time.RFC3339Nano))
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

	var out List[Turn]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
