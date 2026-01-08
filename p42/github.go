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

// GithubOrg represents a github organization.
type GithubOrg struct {
	OrgID          string    `json:"OrgID"`
	OrgName        string    `json:"OrgName"`
	ExternalOrgID  int       `json:"ExternalOrgID"`
	InstallationID int       `json:"InstallationID"`
	CreatedAt      time.Time `json:"CreatedAt"`
	UpdatedAt      time.Time `json:"UpdatedAt"`
	Version        int       `json:"Version"`
	Deleted        bool      `json:"Deleted"`
}

// ObjectType returns the object type for ConflictError handling.
func (GithubOrg) ObjectType() ObjectType { return ObjectTypeGithubOrg }

// GithubConnection represents a GitHub connection for a tenant.
type GithubConnection struct {
	TenantID        string     `json:"TenantID"`
	ConnectionID    string     `json:"ConnectionID"`
	Private         bool       `json:"Private"`
	RunnerID        *string    `json:"RunnerID,omitempty"`
	GithubUserLogin *string    `json:"GithubUserLogin,omitempty"`
	GithubUserID    *int       `json:"GithubUserID,omitempty"`
	OAuthToken      *string    `json:"OAuthToken,omitempty"`
	RefreshToken    *string    `json:"RefreshToken,omitempty"`
	TokenExpiry     *time.Time `json:"TokenExpiry,omitempty"`
	State           *string    `json:"State,omitempty"`
	StateExpiry     *time.Time `json:"StateExpiry,omitempty"`
	CreatedAt       time.Time  `json:"CreatedAt"`
	UpdatedAt       time.Time  `json:"UpdatedAt"`
	Name            *string    `json:"Name,omitempty"`
	Version         int        `json:"Version"`
}

// ObjectType returns the object type for ConflictError handling.
func (GithubConnection) ObjectType() ObjectType { return ObjectTypeGithubConnection }

// CreateGithubConnectionRequest is the request payload for CreateGithubConnection.
type CreateGithubConnectionRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID     string `json:"-"`
	ConnectionID string `json:"-"`

	Private         bool    `json:"Private"`
	RunnerID        *string `json:"RunnerID,omitempty"`
	GithubUserLogin *string `json:"GithubUserLogin,omitempty"`
	GithubUserID    *int    `json:"GithubUserID,omitempty"`
	Name            *string `json:"Name,omitempty"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *CreateGithubConnectionRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "ConnectionID":
		return r.ConnectionID, true
	case "Private":
		return r.Private, true
	case "RunnerID":
		return EvalNullable(r.RunnerID)
	case "GithubUserLogin":
		return EvalNullable(r.GithubUserLogin)
	case "GithubUserID":
		return EvalNullable(r.GithubUserID)
	case "Name":
		return EvalNullable(r.Name)
	default:
		return nil, false
	}
}

// CreateGithubConnection creates a GitHub connection for a tenant.
// nolint: dupl
func (c *Client) CreateGithubConnection(
	ctx context.Context,
	req *CreateGithubConnectionRequest,
) (*GithubConnection, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.ConnectionID == "" {
		return nil, fmt.Errorf("connection id is required")
	}
	if req.Private && req.RunnerID == nil {
		return nil, fmt.Errorf("runner id is required when private is true")
	}
	if req.Private {
		if req.GithubUserLogin != nil {
			return nil, fmt.Errorf("github user login must be nil when private is true")
		}
		if req.GithubUserID != nil {
			return nil, fmt.Errorf("github user id must be nil when private is true")
		}
	}
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"github-connections",
		url.PathEscape(req.ConnectionID),
	)
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

	var out GithubConnection
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListGithubConnectionsRequest represents the request parameters for ListGithubConnections.
type ListGithubConnectionsRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID   string
	MaxResults *int
	Token      *string
	Private    *bool
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *ListGithubConnectionsRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "MaxResults":
		return EvalNullable(r.MaxResults)
	case "Token":
		return EvalNullable(r.Token)
	case "Private":
		return EvalNullable(r.Private)
	default:
		return nil, false
	}
}

// ListGithubConnections retrieves the GitHub connections for a tenant.
// nolint: dupl
func (c *Client) ListGithubConnections(
	ctx context.Context,
	req *ListGithubConnectionsRequest,
) (*List[*GithubConnection], error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "github-connections")
	q := u.Query()
	if req.MaxResults != nil {
		q.Set("maxResults", strconv.Itoa(*req.MaxResults))
	}
	if req.Token != nil {
		q.Set("token", *req.Token)
	}
	if req.Private != nil {
		q.Set("private", strconv.FormatBool(*req.Private))
	}
	u.RawQuery = q.Encode()

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

	var out List[*GithubConnection]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteGithubConnectionRequest is the request payload for DeleteGithubConnection.
type DeleteGithubConnectionRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID     string `json:"-"`
	ConnectionID string `json:"-"`
	Version      int    `json:"-"`
}

// GetVersion returns the expected version for optimistic concurrency control.
func (r *DeleteGithubConnectionRequest) GetVersion() int {
	return r.Version
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *DeleteGithubConnectionRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "ConnectionID":
		return r.ConnectionID, true
	case "Version":
		return r.Version, true
	default:
		return nil, false
	}
}

// DeleteGithubConnection deletes a GitHub connection for a tenant.
func (c *Client) DeleteGithubConnection(ctx context.Context, req *DeleteGithubConnectionRequest) error {
	if req == nil {
		return fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return fmt.Errorf("tenant id is required")
	}
	if req.ConnectionID == "" {
		return fmt.Errorf("connection id is required")
	}

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"github-connections",
		url.PathEscape(req.ConnectionID),
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

// GetGithubConnectionRequest is the request payload for GetGithubConnection.
type GetGithubConnectionRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID     string `json:"-"`
	ConnectionID string `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *GetGithubConnectionRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "ConnectionID":
		return r.ConnectionID, true
	default:
		return nil, false
	}
}

// GetGithubConnection retrieves a GitHub connection for a tenant.
// nolint: dupl
func (c *Client) GetGithubConnection(ctx context.Context, req *GetGithubConnectionRequest) (*GithubConnection, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.ConnectionID == "" {
		return nil, fmt.Errorf("connection id is required")
	}

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"github-connections",
		url.PathEscape(req.ConnectionID),
	)

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

	var out GithubConnection
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateGithubConnectionRequest contains fields for updating a GitHub connection.
type UpdateGithubConnectionRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID     string `json:"-"`
	ConnectionID string `json:"-"`
	Version      int    `json:"-"`

	Private         *bool      `json:"Private,omitempty"`
	RunnerID        *string    `json:"RunnerID,omitempty"`
	GithubUserLogin *string    `json:"GithubUserLogin,omitempty"`
	GithubUserID    *int       `json:"GithubUserID,omitempty"`
	OAuthToken      *string    `json:"OAuthToken,omitempty"`
	RefreshToken    *string    `json:"RefreshToken,omitempty"`
	State           *string    `json:"State,omitempty"`
	StateExpiry     *time.Time `json:"StateExpiry,omitempty"`
	Name            *string    `json:"Name,omitempty"`
}

func (r *UpdateGithubConnectionRequest) GetVersion() int {
	return r.Version
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *UpdateGithubConnectionRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "ConnectionID":
		return r.ConnectionID, true
	case "Version":
		return r.Version, true
	case "Private":
		return EvalNullable(r.Private)
	case "RunnerID":
		return EvalNullable(r.RunnerID)
	case "GithubUserLogin":
		return EvalNullable(r.GithubUserLogin)
	case "GithubUserID":
		return EvalNullable(r.GithubUserID)
	case "OAuthToken":
		return EvalNullable(r.OAuthToken)
	case "RefreshToken":
		return EvalNullable(r.RefreshToken)
	case "State":
		return EvalNullable(r.State)
	case "StateExpiry":
		return EvalNullable(r.StateExpiry)
	default:
		return nil, false
	}
}

// UpdateGithubConnection updates an existing GitHub connection.
// nolint: dupl
func (c *Client) UpdateGithubConnection(ctx context.Context, req *UpdateGithubConnectionRequest) (
	*GithubConnection,
	error,
) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.ConnectionID == "" {
		return nil, fmt.Errorf("connection id is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"github-connections",
		url.PathEscape(req.ConnectionID),
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

	var out GithubConnection
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FindGithubUserRequest is the request for FindGithubUser.
// Exactly one of GithubID or GithubLogin must be provided.
type FindGithubUserRequest struct {
	GithubID       *int    `json:"-"`
	GithubLogin    *string `json:"-"`
	MaxResults     *int    `json:"-"`
	Token          *string `json:"-"`
	IncludeDeleted *bool   `json:"-"`
}

// GetField retrieves the value of a field by name. This is primarily used by
// test helpers to perform golden-file style comparisons.
// nolint:goconst
func (r *FindGithubUserRequest) GetField(name string) (any, bool) {
	switch name {
	case "GithubID":
		return EvalNullable(r.GithubID)
	case "GithubLogin":
		return EvalNullable(r.GithubLogin)
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

// FindGithubUserResponse is the response from FindGithubUser.
type FindGithubUserResponse struct {
	Users     []TenantGithubCreds `json:"Users"`
	NextToken *string             `json:"NextToken"`
}

// FindGithubUser looks up users by their GitHub login or ID.
//
// Exactly one of githubID or githubLogin must be provided as per the API
// specification. If both are provided or neither is provided an error is
// returned before any HTTP request is executed.
func (c *Client) FindGithubUser(ctx context.Context, req *FindGithubUserRequest) (*FindGithubUserResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}

	// Validate exclusivity of search parameters.
	idProvided := req.GithubID != nil
	loginProvided := req.GithubLogin != nil
	if idProvided == loginProvided { // both true or both false
		return nil, fmt.Errorf("exactly one of githubID or githubLogin must be provided")
	}

	u := c.BaseURL.JoinPath("v1", "users")
	q := u.Query()

	if idProvided {
		q.Set("githubID", strconv.Itoa(*req.GithubID))
	}
	if loginProvided {
		q.Set("githubLogin", *req.GithubLogin)
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

	if err := c.authenticate(DelegatedAuthInfo{}, httpReq); err != nil {
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

	var out FindGithubUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// TenantGithubCreds represents the GitHub credentials and related information for a tenant.
type TenantGithubCreds struct {
	TenantID        string     `json:"TenantID"`
	SkipOnboarding  bool       `json:"SkipOnboarding"`
	OAuthToken      *string    `json:"OAuthToken,omitempty"`
	RefreshToken    *string    `json:"RefreshToken,omitempty"`
	TokenExpiry     *time.Time `json:"TokenExpiry,omitempty"`
	State           *string    `json:"State,omitempty"`
	StateExpiry     *time.Time `json:"StateExpiry,omitempty"`
	GithubUserLogin *string    `json:"GithubUserLogin,omitempty"`
	GithubUserID    *int       `json:"GithubUserID,omitempty"`
	TenantVersion   int        `json:"TenantVersion"`
}

func (TenantGithubCreds) ObjectType() ObjectType { return ObjectTypeTenantGithubCreds }

// UpdateTenantGithubCredsRequest is the request payload for UpdateTenantGithubCreds.
// All fields are optional and, when set, will be updated. Fields left as nil are not modified.
// The Version field is supplied via the If-Match header to implement optimistic concurrency.
type UpdateTenantGithubCredsRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID string `json:"-"`
	Version  int    `json:"-"`

	SkipOnboarding  *bool      `json:"SkipOnboarding,omitempty"`
	OAuthToken      *string    `json:"OAuthToken,omitempty"`
	RefreshToken    *string    `json:"RefreshToken,omitempty"`
	TokenExpiry     *time.Time `json:"TokenExpiry,omitempty"`
	State           *string    `json:"State,omitempty"`
	StateExpiry     *time.Time `json:"StateExpiry,omitempty"`
	GithubUserLogin *string    `json:"GithubUserLogin,omitempty"`
	GithubUserID    *int       `json:"GithubUserID,omitempty"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *UpdateTenantGithubCredsRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "Version":
		return r.Version, true
	case "SkipOnboarding":
		return EvalNullable(r.SkipOnboarding)
	case "OAuthToken":
		return EvalNullable(r.OAuthToken)
	case "RefreshToken":
		return EvalNullable(r.RefreshToken)
	case "TokenExpiry":
		return EvalNullable(r.TokenExpiry)
	case "State":
		return EvalNullable(r.State)
	case "StateExpiry":
		return EvalNullable(r.StateExpiry)
	case "GithubUserLogin":
		return EvalNullable(r.GithubUserLogin)
	case "GithubUserID":
		return EvalNullable(r.GithubUserID)
	default:
		return nil, false
	}
}

// UpdateTenantGithubCreds updates the GitHub credentials for a tenant.
// nolint: dupl
func (c *Client) UpdateTenantGithubCreds(ctx context.Context, req *UpdateTenantGithubCredsRequest) (
	*TenantGithubCreds,
	error,
) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "githubcreds")

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

	var out TenantGithubCreds
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTenantGithubCredsRequest is the request payload for GetTenantGithubCreds.
type GetTenantGithubCredsRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID string `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *GetTenantGithubCredsRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	default:
		return nil, false
	}
}

// GetTenantGithubCreds retrieves the GitHub credentials for a tenant.
// nolint:dupl
func (c *Client) GetTenantGithubCreds(ctx context.Context, req *GetTenantGithubCredsRequest) (
	*TenantGithubCreds,
	error,
) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "githubcreds")

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

	var out TenantGithubCreds
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AddGithubOrgRequest is the request payload for AddGithubOrg.
type AddGithubOrgRequest struct {
	OrgID          string `json:"-"`
	OrgName        string `json:"OrgName"`
	ExternalOrgID  int    `json:"ExternalOrgID"`
	InstallationID int    `json:"InstallationID"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *AddGithubOrgRequest) GetField(name string) (any, bool) {
	switch name {
	case "OrgID":
		return r.OrgID, true
	case "OrgName":
		return r.OrgName, true
	case "ExternalOrgID":
		return r.ExternalOrgID, true
	case "InstallationID":
		return r.InstallationID, true
	default:
		return nil, false
	}
}

// AddGithubOrg adds a github org to the service.
func (c *Client) AddGithubOrg(ctx context.Context, req *AddGithubOrgRequest) (*GithubOrg, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.OrgID == "" {
		return nil, fmt.Errorf("org id is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "github", "orgs", url.PathEscape(req.OrgID))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")

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

	var out GithubOrg
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetGithubOrgRequest is the request for GetGithubOrg.
type GetGithubOrgRequest struct {
	OrgID          string `json:"-"`
	IncludeDeleted *bool  `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *GetGithubOrgRequest) GetField(name string) (any, bool) {
	switch name {
	case "OrgID":
		return r.OrgID, true
	case "IncludeDeleted":
		return EvalNullable(r.IncludeDeleted)
	default:
		return nil, false
	}
}

// GetGithubOrg retrieves a github org by ID.
func (c *Client) GetGithubOrg(ctx context.Context, req *GetGithubOrgRequest) (*GithubOrg, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.OrgID == "" {
		return nil, fmt.Errorf("org id is required")
	}

	u := c.BaseURL.JoinPath("v1", "github", "orgs", url.PathEscape(req.OrgID))
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

	if err := c.authenticate(DelegatedAuthInfo{}, httpReq); err != nil {
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

	var out GithubOrg
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListGithubOrgsRequest is the request for ListGithubOrgs.
type ListGithubOrgsRequest struct {
	MaxResults     *int
	Token          *string
	Name           *string
	IncludeDeleted *bool
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *ListGithubOrgsRequest) GetField(name string) (any, bool) {
	switch name {
	case "MaxResults":
		return EvalNullable(r.MaxResults)
	case "Token":
		return EvalNullable(r.Token)
	case "Name":
		return EvalNullable(r.Name)
	case "IncludeDeleted":
		return EvalNullable(r.IncludeDeleted)
	default:
		return nil, false
	}
}

// ListGithubOrgsResponse is the response from ListGithubOrgs.
type ListGithubOrgsResponse struct {
	Orgs      []GithubOrg `json:"Orgs"`
	NextToken *string     `json:"NextToken"`
}

// ListGithubOrgs lists all github orgs in the service.
func (c *Client) ListGithubOrgs(ctx context.Context, req *ListGithubOrgsRequest) (*ListGithubOrgsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	u := c.BaseURL.JoinPath("v1", "github", "orgs")
	q := u.Query()
	if req.MaxResults != nil {
		q.Set("maxResults", strconv.Itoa(*req.MaxResults))
	}
	if req.Token != nil {
		q.Set("token", *req.Token)
	}
	if req.Name != nil {
		q.Set("name", *req.Name)
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

	if err := c.authenticate(DelegatedAuthInfo{}, httpReq); err != nil {
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

	var out ListGithubOrgsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListOrgsForGithubConnectionRequest is the request for ListOrgsForGithubConnection.
type ListOrgsForGithubConnectionRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID     string
	ConnectionID string
	MaxResults   *int
	Token        *string
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *ListOrgsForGithubConnectionRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "ConnectionID":
		return r.ConnectionID, true
	case "MaxResults":
		return EvalNullable(r.MaxResults)
	case "Token":
		return EvalNullable(r.Token)
	default:
		return nil, false
	}
}

// ListOrgsForGithubConnectionResponse is the response from ListOrgsForGithubConnection.
type ListOrgsForGithubConnectionResponse struct {
	Items     []string `json:"Items"`
	NextToken *string  `json:"NextToken"`
}

// ListOrgsForGithubConnection lists orgs for a specific GitHub connection.
// nolint: dupl
func (c *Client) ListOrgsForGithubConnection(ctx context.Context, req *ListOrgsForGithubConnectionRequest) (
	*ListOrgsForGithubConnectionResponse,
	error,
) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.ConnectionID == "" {
		return nil, fmt.Errorf("connection id is required")
	}

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"github-connections",
		url.PathEscape(req.ConnectionID),
		"orgs",
	)
	q := u.Query()
	if req.MaxResults != nil {
		q.Set("maxResults", strconv.Itoa(*req.MaxResults))
	}
	if req.Token != nil {
		q.Set("token", *req.Token)
	}
	u.RawQuery = q.Encode()

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

	var out ListOrgsForGithubConnectionResponse
	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateGithubOrgRequest is the request payload for UpdateGithubOrg.
type UpdateGithubOrgRequest struct {
	OrgID          string  `json:"-"`
	Version        int     `json:"-"`
	OrgName        *string `json:"OrgName,omitempty"`
	InstallationID *int    `json:"InstallationID,omitempty"`
	Deleted        *bool   `json:"Deleted,omitempty"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *UpdateGithubOrgRequest) GetField(name string) (any, bool) {
	switch name {
	case "OrgID":
		return r.OrgID, true
	case "Version":
		return r.Version, true
	case "OrgName":
		return EvalNullable(r.OrgName)
	case "InstallationID":
		return EvalNullable(r.InstallationID)
	case "Deleted":
		return EvalNullable(r.Deleted)
	default:
		return nil, false
	}
}

// UpdateGithubOrg updates a github org in the service.
func (c *Client) UpdateGithubOrg(ctx context.Context, req *UpdateGithubOrgRequest) (*GithubOrg, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.OrgID == "" {
		return nil, fmt.Errorf("org id is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "github", "orgs", url.PathEscape(req.OrgID))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("If-Match", strconv.Itoa(req.Version))

	if err := c.authenticate(DelegatedAuthInfo{}, httpReq); err != nil {
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

	var out GithubOrg
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteGithubOrgRequest is the request payload for DeleteGithubOrg.
type DeleteGithubOrgRequest struct {
	OrgID   string `json:"-"`
	Version int    `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *DeleteGithubOrgRequest) GetField(name string) (any, bool) {
	switch name {
	case "OrgID":
		return r.OrgID, true
	case "Version":
		return r.Version, true
	default:
		return nil, false
	}
}

// DeleteGithubOrg soft deletes a github org from the service.
// nolint: dupl
func (c *Client) DeleteGithubOrg(ctx context.Context, req *DeleteGithubOrgRequest) error {
	if req == nil {
		return fmt.Errorf("req is nil")
	}
	if req.OrgID == "" {
		return fmt.Errorf("org id is required")
	}
	u := c.BaseURL.JoinPath("v1", "github", "orgs", url.PathEscape(req.OrgID))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, u.String(), nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("If-Match", strconv.Itoa(req.Version))

	if err := c.authenticate(DelegatedAuthInfo{}, httpReq); err != nil {
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
