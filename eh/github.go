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
		return evalNullable(r.GithubID)
	case "GithubLogin":
		return evalNullable(r.GithubLogin)
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
		return evalNullable(r.SkipOnboarding)
	case "OAuthToken":
		return evalNullable(r.OAuthToken)
	case "RefreshToken":
		return evalNullable(r.RefreshToken)
	case "TokenExpiry":
		return evalNullable(r.TokenExpiry)
	case "State":
		return evalNullable(r.State)
	case "StateExpiry":
		return evalNullable(r.StateExpiry)
	case "GithubUserLogin":
		return evalNullable(r.GithubUserLogin)
	case "GithubUserID":
		return evalNullable(r.GithubUserID)
	default:
		return nil, false
	}
}

// UpdateTenantGithubCreds updates the GitHub credentials for a tenant.
// nolint: dupl
func (c *Client) UpdateTenantGithubCreds(ctx context.Context, req *UpdateTenantGithubCredsRequest) (*TenantGithubCreds, error) {
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
func (r *GetTenantGithubCredsRequest) GetField(name string) (any, bool) { // nolint: goconst
	switch name {
	case "TenantID":
		return r.TenantID, true
	default:
		return nil, false
	}
}

// GetTenantGithubCreds retrieves the GitHub credentials for a tenant.
func (c *Client) GetTenantGithubCreds(ctx context.Context, req *GetTenantGithubCredsRequest) (*TenantGithubCreds, error) { // nolint:dupl
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
		return evalNullable(r.IncludeDeleted)
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
		return evalNullable(r.MaxResults)
	case "Token":
		return evalNullable(r.Token)
	case "Name":
		return evalNullable(r.Name)
	case "IncludeDeleted":
		return evalNullable(r.IncludeDeleted)
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
		return evalNullable(r.OrgName)
	case "InstallationID":
		return evalNullable(r.InstallationID)
	case "Deleted":
		return evalNullable(r.Deleted)
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
