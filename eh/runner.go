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

	"github.com/debugging-sucks/ecies"
)

// Runner represents a runner in Event Horizon.
type Runner struct {
	TenantID      string    `json:"TenantId"`
	RunnerID      string    `json:"RunnerId"`
	Name          string    `json:"Name"`
	Description   *string   `json:"Description,omitempty"`
	IsCloud       bool      `json:"IsCloud"`
	RunsTasks     bool      `json:"RunsTasks"`
	ProxiesGithub bool      `json:"ProxiesGithub"`
	CreatedAt     time.Time `json:"CreatedAt"`
	UpdatedAt     time.Time `json:"UpdatedAt"`
	Deleted       bool      `json:"Deleted"`
	Version       int       `json:"Version"`
}

// ObjectType returns the object type for ConflictError handling.
func (Runner) ObjectType() ObjectType { return ObjectTypeRunner }

// CreateRunnerRequest is the request payload for CreateRunner.
type CreateRunnerRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID string `json:"-"`
	RunnerID string `json:"-"`

	Name          string  `json:"Name"`
	Description   *string `json:"Description,omitempty"`
	IsCloud       bool    `json:"IsCloud"`
	RunsTasks     bool    `json:"RunsTasks"`
	ProxiesGithub bool    `json:"ProxiesGithub"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *CreateRunnerRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "RunnerID":
		return r.RunnerID, true
	case "Name":
		return r.Name, true
	case "Description":
		return EvalNullable(r.Description)
	case "IsCloud":
		return r.IsCloud, true
	case "RunsTasks":
		return r.RunsTasks, true
	case "ProxiesGithub":
		return r.ProxiesGithub, true
	default:
		return nil, false
	}
}

// UpdateRunnerRequest is the request payload for UpdateRunner.
type UpdateRunnerRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID string `json:"-"`
	RunnerID string `json:"-"`
	Version  int    `json:"-"`

	Name          *string `json:"Name,omitempty"`
	Description   *string `json:"Description,omitempty"`
	IsCloud       *bool   `json:"IsCloud,omitempty"`
	RunsTasks     *bool   `json:"RunsTasks,omitempty"`
	ProxiesGithub *bool   `json:"ProxiesGithub,omitempty"`
	Deleted       *bool   `json:"Deleted,omitempty"`
}

func (r *UpdateRunnerRequest) GetVersion() int {
	return r.Version
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *UpdateRunnerRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "RunnerID":
		return r.RunnerID, true
	case "Version":
		return r.Version, true
	case "Name":
		return EvalNullable(r.Name)
	case "Description":
		return EvalNullable(r.Description)
	case "IsCloud":
		return EvalNullable(r.IsCloud)
	case "RunsTasks":
		return EvalNullable(r.RunsTasks)
	case "ProxiesGithub":
		return EvalNullable(r.ProxiesGithub)
	case "Deleted":
		return EvalNullable(r.Deleted)
	default:
		return nil, false
	}
}

// CreateRunner creates a new runner for a tenant.
// nolint: dupl
func (c *Client) CreateRunner(ctx context.Context, req *CreateRunnerRequest) (*Runner, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.RunnerID == "" {
		return nil, fmt.Errorf("runner id is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "runners", url.PathEscape(req.RunnerID))
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

	var runner Runner
	if err := json.NewDecoder(resp.Body).Decode(&runner); err != nil {
		return nil, err
	}
	return &runner, nil
}

// ListRunnersRequest is the request payload for ListRunners.
type ListRunnersRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID       string
	MaxResults     *int
	Token          *string
	IncludeDeleted *bool
	RunsTasks      *bool
	ProxiesGithub  *bool
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *ListRunnersRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "MaxResults":
		return EvalNullable(r.MaxResults)
	case "Token":
		return EvalNullable(r.Token)
	case "IncludeDeleted":
		return EvalNullable(r.IncludeDeleted)
	case "RunsTasks":
		return EvalNullable(r.RunsTasks)
	case "ProxiesGithub":
		return EvalNullable(r.ProxiesGithub)
	default:
		return nil, false
	}
}

// ListRunners lists the runners for a tenant.
// nolint: dupl
func (c *Client) ListRunners(ctx context.Context, req *ListRunnersRequest) (*List[*Runner], error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "runners")
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
	if req.RunsTasks != nil {
		q.Set("runsTasks", strconv.FormatBool(*req.RunsTasks))
	}
	if req.ProxiesGithub != nil {
		q.Set("proxiesGithub", strconv.FormatBool(*req.ProxiesGithub))
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

	var out List[*Runner]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteRunnerRequest is the request payload for DeleteRunner.
type DeleteRunnerRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID string `json:"-"`
	RunnerID string `json:"-"`
	Version  int    `json:"-"`
}

func (r *DeleteRunnerRequest) GetVersion() int {
	return r.Version
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *DeleteRunnerRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "RunnerID":
		return r.RunnerID, true
	case "Version":
		return r.Version, true
	default:
		return nil, false
	}
}

// DeleteRunner deletes a runner.
// nolint: dupl
func (c *Client) DeleteRunner(ctx context.Context, req *DeleteRunnerRequest) error {
	if req == nil {
		return fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return fmt.Errorf("tenant id is required")
	}
	if req.RunnerID == "" {
		return fmt.Errorf("runner id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "runners", url.PathEscape(req.RunnerID))
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

// GetRunnerRequest is the request payload for GetRunner.
type GetRunnerRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID       string `json:"-"`
	RunnerID       string `json:"-"`
	IncludeDeleted *bool  `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *GetRunnerRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "RunnerID":
		return r.RunnerID, true
	case "IncludeDeleted":
		return EvalNullable(r.IncludeDeleted)
	default:
		return nil, false
	}
}

// GetRunner retrieves a runner by ID.
// nolint:dupl
func (c *Client) GetRunner(ctx context.Context, req *GetRunnerRequest) (*Runner, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.RunnerID == "" {
		return nil, fmt.Errorf("runner id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "runners", url.PathEscape(req.RunnerID))
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

	var runner Runner
	if err := json.NewDecoder(resp.Body).Decode(&runner); err != nil {
		return nil, err
	}
	return &runner, nil
}

// UpdateRunner updates an existing runner for a tenant.
// nolint: dupl
func (c *Client) UpdateRunner(ctx context.Context, req *UpdateRunnerRequest) (*Runner, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.RunnerID == "" {
		return nil, fmt.Errorf("runner id is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "runners", url.PathEscape(req.RunnerID))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("If-Match", strconv.Itoa(req.Version))
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

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var runner Runner
	err = json.NewDecoder(resp.Body).Decode(&runner)
	if err != nil {
		return nil, err
	}
	return &runner, nil
}

// GenerateRunnerTokenRequest is the request payload for GenerateRunnerToken.
type GenerateRunnerTokenRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID string `json:"-"`
	RunnerID string `json:"-"`
	TokenID  string `json:"-"`
	TTLDays  *int   `json:"TTLDays,omitempty"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *GenerateRunnerTokenRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "RunnerID":
		return r.RunnerID, true
	case "TokenID":
		return r.TokenID, true
	case "TTLDays":
		return EvalNullable(r.TTLDays)
	default:
		return nil, false
	}
}

// GenerateRunnerTokenResponse is the response payload for GenerateRunnerToken.
type GenerateRunnerTokenResponse struct {
	RunnerTokenMetadata
	Token string `json:"Token"`
}

// GenerateRunnerToken generates a new token for a runner.
func (c *Client) GenerateRunnerToken(ctx context.Context, req *GenerateRunnerTokenRequest) (*GenerateRunnerTokenResponse, error) {
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.RunnerID == "" {
		return nil, fmt.Errorf("runner id is required")
	}
	if req.TokenID == "" {
		return nil, fmt.Errorf("token id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "runners", url.PathEscape(req.RunnerID), "tokens", url.PathEscape(req.TokenID))
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(body))
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

	var token GenerateRunnerTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}
	return &token, nil
}

// RevokeRunnerTokenRequest is the request payload for RevokeRunnerToken.
type RevokeRunnerTokenRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID string `json:"-"`
	RunnerID string `json:"-"`
	TokenID  string `json:"-"`
	Version  int    `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *RevokeRunnerTokenRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "RunnerID":
		return r.RunnerID, true
	case "TokenID":
		return r.TokenID, true
	case "Version":
		return r.Version, true
	default:
		return nil, false
	}
}

func (r *RevokeRunnerTokenRequest) GetVersion() int {
	return r.Version
}

// RevokeRunnerToken revokes a runner token.
// nolint: dupl
func (c *Client) RevokeRunnerToken(ctx context.Context, req *RevokeRunnerTokenRequest) error {
	if req == nil {
		return fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return fmt.Errorf("tenant id is required")
	}
	if req.RunnerID == "" {
		return fmt.Errorf("runner id is required")
	}
	if req.TokenID == "" {
		return fmt.Errorf("token id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "runners", url.PathEscape(req.RunnerID), "tokens", url.PathEscape(req.TokenID), "revoke")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), nil)
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

// RunnerQueue represents a queue registered for a runner.
type RunnerQueue struct {
	TenantID                           string    `json:"TenantID"`
	RunnerID                           string    `json:"RunnerID"`
	QueueID                            string    `json:"QueueID"`
	PublicKey                          string    `json:"PublicKey"`
	CreatedAt                          time.Time `json:"CreatedAt"`
	Version                            int       `json:"Version"`
	IsHealthy                          bool      `json:"IsHealthy"`
	Draining                           bool      `json:"Draining"`
	NConsecutiveFailedHealthChecks     int       `json:"NConsecutiveFailedHealthChecks"`
	NConsecutiveSuccessfulHealthChecks int       `json:"NConsecutiveSuccessfulHealthChecks"`
	LastHealthCheckAt                  time.Time `json:"LastHealthCheckAt"`
}

func (RunnerQueue) ObjectType() ObjectType { return ObjectTypeRunnerQueue }

// ListRunnerQueuesRequest contains parameters for ListRunnerQueues.
type ListRunnerQueuesRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID       *string
	RunnerID       *string
	IncludeHealthy *bool
	IncludeDrained *bool
	MaxResults     *int
	Token          *string
	MinQueueID     *string
	MaxQueueID     *string
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *ListRunnerQueuesRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return EvalNullable(r.TenantID)
	case "RunnerID":
		return EvalNullable(r.RunnerID)
	case "IncludeHealthy":
		return EvalNullable(r.IncludeHealthy)
	case "IncludeDrained":
		return EvalNullable(r.IncludeDrained)
	case "MaxResults":
		return EvalNullable(r.MaxResults)
	case "Token":
		return EvalNullable(r.Token)
	case "MinQueueID":
		return EvalNullable(r.MinQueueID)
	case "MaxQueueID":
		return EvalNullable(r.MaxQueueID)
	default:
		return nil, false
	}
}

// ListRunnerQueues retrieves runner queues, optionally filtered by tenant, runner, health status, drained status, and queue range.
// nolint: dupl
func (c *Client) ListRunnerQueues(ctx context.Context, req *ListRunnerQueuesRequest) (*List[*RunnerQueue], error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if (req.TenantID == nil) != (req.RunnerID == nil) {
		return nil, fmt.Errorf("tenant id and runner id must be provided together")
	}
	if (req.MinQueueID == nil) != (req.MaxQueueID == nil) {
		return nil, fmt.Errorf("min queue id and max queue id must be provided together")
	}

	u := c.BaseURL.JoinPath("v1", "runner-queues")
	q := u.Query()
	if req.TenantID != nil && req.RunnerID != nil {
		q.Set("tenantID", *req.TenantID)
		q.Set("runnerID", *req.RunnerID)
	}
	if req.IncludeHealthy != nil {
		q.Set("includeHealthy", strconv.FormatBool(*req.IncludeHealthy))
	}
	if req.IncludeDrained != nil {
		q.Set("includeDrained", strconv.FormatBool(*req.IncludeDrained))
	}
	if req.MaxResults != nil {
		q.Set("maxResults", strconv.Itoa(*req.MaxResults))
	}
	if req.Token != nil {
		q.Set("token", *req.Token)
	}
	if req.MinQueueID != nil {
		q.Set("minQueueID", *req.MinQueueID)
	}
	if req.MaxQueueID != nil {
		q.Set("maxQueueID", *req.MaxQueueID)
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

	var out List[*RunnerQueue]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRunnerQueueRequest contains parameters for GetRunnerQueue.
type GetRunnerQueueRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID string `json:"-"`
	RunnerID string `json:"-"`
	QueueID  string `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *GetRunnerQueueRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "RunnerID":
		return r.RunnerID, true
	case "QueueID":
		return r.QueueID, true
	default:
		return nil, false
	}
}

// GetRunnerQueue retrieves runner queue metadata.
// nolint:dupl
func (c *Client) GetRunnerQueue(ctx context.Context, req *GetRunnerQueueRequest) (*RunnerQueue, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.RunnerID == "" {
		return nil, fmt.Errorf("runner id is required")
	}
	if req.QueueID == "" {
		return nil, fmt.Errorf("queue id is required")
	}

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"runners",
		url.PathEscape(req.RunnerID),
		"queues",
		url.PathEscape(req.QueueID),
	)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Accept", "application/json")
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

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var queue RunnerQueue
	err = json.NewDecoder(resp.Body).Decode(&queue)
	if err != nil {
		return nil, err
	}
	return &queue, nil
}

// RegisterRunnerQueueRequest contains parameters for RegisterRunnerQueue.
type RegisterRunnerQueueRequest struct {
	FeatureFlags

	TenantID  string `json:"-"`
	RunnerID  string `json:"-"`
	QueueID   string `json:"-"`
	PublicKey string `json:"PublicKey"`
}

// UpdateRunnerQueueRequest contains parameters for UpdateRunnerQueue.
type UpdateRunnerQueueRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID string `json:"-"`
	RunnerID string `json:"-"`
	QueueID  string `json:"-"`
	Version  int    `json:"-"`

	IsHealthy                          *bool      `json:"IsHealthy,omitempty"`
	Draining                           *bool      `json:"Draining,omitempty"`
	NConsecutiveFailedHealthChecks     *int       `json:"NConsecutiveFailedHealthChecks,omitempty"`
	NConsecutiveSuccessfulHealthChecks *int       `json:"NConsecutiveSuccessfulHealthChecks,omitempty"`
	LastHealthCheckAt                  *time.Time `json:"LastHealthCheckAt,omitempty"`
}

func (r *UpdateRunnerQueueRequest) GetVersion() int {
	return r.Version
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *UpdateRunnerQueueRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "RunnerID":
		return r.RunnerID, true
	case "QueueID":
		return r.QueueID, true
	case "Version":
		return r.Version, true
	case "IsHealthy":
		return EvalNullable(r.IsHealthy)
	case "Draining":
		return EvalNullable(r.Draining)
	case "NConsecutiveFailedHealthChecks":
		return EvalNullable(r.NConsecutiveFailedHealthChecks)
	case "NConsecutiveSuccessfulHealthChecks":
		return EvalNullable(r.NConsecutiveSuccessfulHealthChecks)
	case "LastHealthCheckAt":
		return EvalNullable(r.LastHealthCheckAt)
	default:
		return nil, false
	}
}

// RegisterRunnerQueue registers a queue for a runner.
func (c *Client) RegisterRunnerQueue(ctx context.Context, req *RegisterRunnerQueueRequest) (*RunnerQueue, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.RunnerID == "" {
		return nil, fmt.Errorf("runner id is required")
	}
	if req.QueueID == "" {
		return nil, fmt.Errorf("queue id is required")
	}
	if req.PublicKey == "" {
		return nil, fmt.Errorf("public key is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"runners",
		url.PathEscape(req.RunnerID),
		"queues",
		url.PathEscape(req.QueueID),
	)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	processFeatureFlags(httpReq, req.FeatureFlags)

	err = c.authenticate(DelegatedAuthInfo{}, httpReq)
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

	var queue RunnerQueue
	err = json.NewDecoder(resp.Body).Decode(&queue)
	if err != nil {
		return nil, err
	}
	return &queue, nil
}

// UpdateRunnerQueue updates metadata for a runner queue.
// nolint: dupl
func (c *Client) UpdateRunnerQueue(ctx context.Context, req *UpdateRunnerQueueRequest) (*RunnerQueue, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.RunnerID == "" {
		return nil, fmt.Errorf("runner id is required")
	}
	if req.QueueID == "" {
		return nil, fmt.Errorf("queue id is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"runners",
		url.PathEscape(req.RunnerID),
		"queues",
		url.PathEscape(req.QueueID),
	)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("If-Match", strconv.Itoa(req.Version))
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

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var queue RunnerQueue
	err = json.NewDecoder(resp.Body).Decode(&queue)
	if err != nil {
		return nil, err
	}
	return &queue, nil
}

// DeleteRunnerQueueRequest contains parameters for DeleteRunnerQueue.
type DeleteRunnerQueueRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID string `json:"-"`
	RunnerID string `json:"-"`
	QueueID  string `json:"-"`
	Version  int    `json:"-"`
}

func (r *DeleteRunnerQueueRequest) GetVersion() int {
	return r.Version
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *DeleteRunnerQueueRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "RunnerID":
		return r.RunnerID, true
	case "QueueID":
		return r.QueueID, true
	case "Version":
		return r.Version, true
	default:
		return nil, false
	}
}

// DeleteRunnerQueue removes a queue registered for a runner.
func (c *Client) DeleteRunnerQueue(ctx context.Context, req *DeleteRunnerQueueRequest) error {
	if req == nil {
		return fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return fmt.Errorf("tenant id is required")
	}
	if req.RunnerID == "" {
		return fmt.Errorf("runner id is required")
	}
	if req.QueueID == "" {
		return fmt.Errorf("queue id is required")
	}

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"runners",
		url.PathEscape(req.RunnerID),
		"queues",
		url.PathEscape(req.QueueID),
	)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, u.String(), nil)
	if err != nil {
		return err
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("If-Match", strconv.Itoa(req.Version))
	processFeatureFlags(httpReq, req.FeatureFlags)

	err = c.authenticate(req.DelegatedAuthInfo, httpReq)
	if err != nil {
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

// WriteResponseRequest is the request payload for WriteResponse.
type WriteResponseRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID  string        `json:"-"`
	RunnerID  string        `json:"-"`
	QueueID   string        `json:"-"`
	MessageID string        `json:"-"`
	CallerID  string        `json:"CallerID"`
	Payload   WrappedSecret `json:"Payload"`
}

func (r *WriteResponseRequest) UnmarshalJSON(data []byte) error {
	var tmp struct {
		CallerID string
		Payload  json.RawMessage
	}

	var payload struct {
		EncryptionAlgorithm string
	}

	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}

	err = json.Unmarshal(tmp.Payload, &payload)
	if err != nil {
		return err
	}

	r.CallerID = tmp.CallerID

	switch payload.EncryptionAlgorithm {
	case ecies.EciesCofactorVariableIVX963SHA256AESGCM:
		r.Payload = &ecies.WrappedSecret{}
	default:
		return fmt.Errorf("unknown encryption algorithm: %s", payload.EncryptionAlgorithm)
	}

	return json.Unmarshal(tmp.Payload, r.Payload)
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *WriteResponseRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "RunnerID":
		return r.RunnerID, true
	case "QueueID":
		return r.QueueID, true
	case "MessageID":
		return r.MessageID, true
	case "CallerID":
		return r.CallerID, true
	case "Payload":
		return r.Payload, true
	default:
		return nil, false
	}
}

// WriteResponse sends a response for a runner message.
// nolint:dupl
func (c *Client) WriteResponse(ctx context.Context, req *WriteResponseRequest) error {
	if req == nil {
		return fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return fmt.Errorf("tenant id is required")
	}
	if req.RunnerID == "" {
		return fmt.Errorf("runner id is required")
	}
	if req.QueueID == "" {
		return fmt.Errorf("queue id is required")
	}
	if req.MessageID == "" {
		return fmt.Errorf("message id is required")
	}
	if req.CallerID == "" {
		return fmt.Errorf("caller id is required")
	}
	if req.Payload == nil {
		return fmt.Errorf("payload is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return err
	}

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"runners",
		url.PathEscape(req.RunnerID),
		"queues",
		url.PathEscape(req.QueueID),
		"messages",
		url.PathEscape(req.MessageID),
		"response",
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	processFeatureFlags(httpReq, req.FeatureFlags)

	err = c.authenticate(req.DelegatedAuthInfo, httpReq)
	if err != nil {
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
