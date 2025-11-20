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

// RunnerInstance represents a runner instance in Event Horizon.
type RunnerInstance struct {
	TenantID        string    `json:"TenantId"`
	RunnerID        string    `json:"RunnerId"`
	InstanceID      string    `json:"InstanceId"`
	PublicKey       string    `json:"PublicKey"`
	RegisteredAt    time.Time `json:"RegisteredAt"`
	LastHeartBeatAt time.Time `json:"LastHeartBeatAt"`
	IsHealthy       bool      `json:"IsHealthy"`
}

// ObjectType returns the object type for ConflictError handling.
func (RunnerInstance) ObjectType() ObjectType { return ObjectTypeRunnerInstance }

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
		return evalNullable(r.Description)
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
		return evalNullable(r.Name)
	case "Description":
		return evalNullable(r.Description)
	case "IsCloud":
		return evalNullable(r.IsCloud)
	case "RunsTasks":
		return evalNullable(r.RunsTasks)
	case "ProxiesGithub":
		return evalNullable(r.ProxiesGithub)
	case "Deleted":
		return evalNullable(r.Deleted)
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
		return evalNullable(r.MaxResults)
	case "Token":
		return evalNullable(r.Token)
	case "IncludeDeleted":
		return evalNullable(r.IncludeDeleted)
	case "RunsTasks":
		return evalNullable(r.RunsTasks)
	case "ProxiesGithub":
		return evalNullable(r.ProxiesGithub)
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
		return evalNullable(r.IncludeDeleted)
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
		return evalNullable(r.TTLDays)
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

// RegisterRunnerInstanceRequest is the request payload for RegisterRunnerInstance.
type RegisterRunnerInstanceRequest struct {
	FeatureFlags

	TenantID   string `json:"-"`
	RunnerID   string `json:"-"`
	InstanceID string `json:"-"`
	PublicKey  string `json:"PublicKey"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *RegisterRunnerInstanceRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "RunnerID":
		return r.RunnerID, true
	case "InstanceID":
		return r.InstanceID, true
	case "PublicKey":
		return r.PublicKey, true
	default:
		return nil, false
	}
}

// RegisterRunnerInstance registers a new runner instance.
func (c *Client) RegisterRunnerInstance(ctx context.Context, req *RegisterRunnerInstanceRequest) (*RunnerInstance, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.RunnerID == "" {
		return nil, fmt.Errorf("runner id is required")
	}
	if req.InstanceID == "" {
		return nil, fmt.Errorf("instance id is required")
	}
	if req.PublicKey == "" {
		return nil, fmt.Errorf("public key is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "runners", url.PathEscape(req.RunnerID), "instances", url.PathEscape(req.InstanceID))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	processFeatureFlags(httpReq, req.FeatureFlags)

	if err := c.authenticate(DelegatedAuthInfo{}, httpReq); err != nil {
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

	var instance RunnerInstance
	if err := json.NewDecoder(resp.Body).Decode(&instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

// WriteResponseRequest is the request payload for WriteResponse.
type WriteResponseRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID   string `json:"-"`
	RunnerID   string `json:"-"`
	InstanceID string `json:"-"`
	MessageID  string `json:"-"`

	CallerID string `json:"CallerID"`
	Payload  string `json:"Payload"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *WriteResponseRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "RunnerID":
		return r.RunnerID, true
	case "InstanceID":
		return r.InstanceID, true
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
// nolint: dupl
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
	if req.InstanceID == "" {
		return fmt.Errorf("instance id is required")
	}
	if req.MessageID == "" {
		return fmt.Errorf("message id is required")
	}
	if req.CallerID == "" {
		return fmt.Errorf("caller id is required")
	}
	if req.Payload == "" {
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
		"instances",
		url.PathEscape(req.InstanceID),
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
