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

// EnvVar defines an environment variable for an Environment.
type EnvVar struct {
	Name     string `json:"Name"`
	Value    string `json:"Value"`
	IsSecret bool   `json:"IsSecret"`
}

// Environment represents an execution environment.
type Environment struct {
	TenantID      string    `json:"TenantId"`
	EnvironmentID string    `json:"EnvironmentId"`
	Name          string    `json:"Name"`
	Description   string    `json:"Description"`
	Context       string    `json:"Context"`
	Repos         []string  `json:"Repos"`
	SetupScript   string    `json:"SetupScript"`
	DockerImage   string    `json:"DockerImage"`
	AllowedHosts  []string  `json:"AllowedHosts"`
	EnvVars       []EnvVar  `json:"EnvVars"`
	CreatedAt     time.Time `json:"CreatedAt"`
	UpdatedAt     time.Time `json:"UpdatedAt"`
	Deleted       bool      `json:"Deleted"`
	Version       int       `json:"Version"`
}

// ObjectType returns the object type for ConflictError handling.
func (Environment) ObjectType() ObjectType { return ObjectTypeEnvironment }

// CreateEnvironmentRequest is the request payload for CreateEnvironment.
type CreateEnvironmentRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID      string   `json:"-"`
	EnvironmentID string   `json:"-"`
	Name          string   `json:"Name"`
	Description   string   `json:"Description"`
	Context       string   `json:"Context"`
	Repos         []string `json:"Repos"`
	SetupScript   string   `json:"SetupScript"`
	DockerImage   string   `json:"DockerImage"`
	AllowedHosts  []string `json:"AllowedHosts"`
	EnvVars       []EnvVar `json:"EnvVars"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *CreateEnvironmentRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "EnvironmentID":
		return r.EnvironmentID, true
	case "Name": //nolint: goconst
		return r.Name, true
	case "Description":
		return r.Description, true
	case "Context":
		return r.Context, true
	case "Repos":
		return r.Repos, true
	case "SetupScript":
		return r.SetupScript, true
	case "DockerImage":
		return r.DockerImage, true
	case "AllowedHosts":
		return r.AllowedHosts, true
	case "EnvVars":
		return r.EnvVars, true
	default:
		return nil, false
	}
}

// CreateEnvironment creates a new environment for a tenant.
func (c *Client) CreateEnvironment(ctx context.Context, req *CreateEnvironmentRequest) (*Environment, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "environments", url.PathEscape(req.EnvironmentID))
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

	var out Environment
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetEnvironmentRequest is the request payload for GetEnvironment.
type GetEnvironmentRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID       string `json:"-"`
	EnvironmentID  string `json:"-"`
	IncludeDeleted *bool  `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *GetEnvironmentRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "EnvironmentID":
		return r.EnvironmentID, true
	case "IncludeDeleted":
		return evalNullable(r.IncludeDeleted)
	default:
		return nil, false
	}
}

// GetEnvironment retrieves an environment by ID.
// nolint:dupl
func (c *Client) GetEnvironment(ctx context.Context, req *GetEnvironmentRequest) (*Environment, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.EnvironmentID == "" {
		return nil, fmt.Errorf("environment id is required")
	}
	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "environments", url.PathEscape(req.EnvironmentID))
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

	var out Environment
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateEnvironmentRequest is the request payload for UpdateEnvironment.
type UpdateEnvironmentRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID      string    `json:"-"`
	EnvironmentID string    `json:"-"`
	Version       int       `json:"-"`
	Name          *string   `json:"Name,omitempty"`
	Description   *string   `json:"Description,omitempty"`
	Context       *string   `json:"Context,omitempty"`
	Repos         *[]string `json:"Repos,omitempty"`
	SetupScript   *string   `json:"SetupScript,omitempty"`
	DockerImage   *string   `json:"DockerImage,omitempty"`
	AllowedHosts  *[]string `json:"AllowedHosts,omitempty"`
	EnvVars       *[]EnvVar `json:"EnvVars,omitempty"`
	Deleted       *bool     `json:"Deleted,omitempty"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *UpdateEnvironmentRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "EnvironmentID":
		return r.EnvironmentID, true
	case "Version": //nolint: goconst
		return r.Version, true
	case "Name":
		return evalNullable(r.Name)
	case "Description":
		return evalNullable(r.Description)
	case "Context":
		return evalNullable(r.Context)
	case "Repos":
		return evalNullable(r.Repos)
	case "SetupScript":
		return evalNullable(r.SetupScript)
	case "DockerImage":
		return evalNullable(r.DockerImage)
	case "AllowedHosts":
		return evalNullable(r.AllowedHosts)
	case "EnvVars":
		return evalNullable(r.EnvVars)
	case "Deleted":
		return evalNullable(r.Deleted)
	default:
		return nil, false
	}
}

// UpdateEnvironment updates an existing environment.
// nolint: dupl
func (c *Client) UpdateEnvironment(ctx context.Context, req *UpdateEnvironmentRequest) (*Environment, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.EnvironmentID == "" {
		return nil, fmt.Errorf("environment id is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "environments", url.PathEscape(req.EnvironmentID))
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

	var out Environment
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListEnvironmentsRequest is the request for ListEnvironments.
type ListEnvironmentsRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID       string
	MaxResults     *int
	Token          *string
	IncludeDeleted *bool
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *ListEnvironmentsRequest) GetField(name string) (any, bool) {
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

// ListEnvironments retrieves the environments for a tenant.
// nolint: dupl
func (c *Client) ListEnvironments(ctx context.Context, req *ListEnvironmentsRequest) (*List[Environment], error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "environments")
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

	var out List[Environment]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteEnvironmentRequest is the request payload for DeleteEnvironment.
type DeleteEnvironmentRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID      string `json:"-"`
	EnvironmentID string `json:"-"`
	Version       int    `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *DeleteEnvironmentRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "EnvironmentID":
		return r.EnvironmentID, true
	case "Version":
		return r.Version, true
	default:
		return nil, false
	}
}

// DeleteEnvironment deletes an environment.
// nolint: dupl
func (c *Client) DeleteEnvironment(ctx context.Context, req *DeleteEnvironmentRequest) error {
	if req == nil {
		return fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return fmt.Errorf("tenant id is required")
	}
	if req.EnvironmentID == "" {
		return fmt.Errorf("environment id is required")
	}
	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "environments", url.PathEscape(req.EnvironmentID))
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
