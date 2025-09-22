package eh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// TenantGithubCreds represents the GitHub credentials and related information for a tenant.
type TenantGithubCreds struct {
	TenantID        string  `json:"TenantID"`
	SkipOnboarding  bool    `json:"SkipOnboarding"`
	OAuthToken      *string `json:"OAuthToken,omitempty"`
	RefreshToken    *string `json:"RefreshToken,omitempty"`
	TokenExpiry     *string `json:"TokenExpiry,omitempty"`
	State           *string `json:"State,omitempty"`
	StateExpiry     *string `json:"StateExpiry,omitempty"`
	GithubUserLogin *string `json:"GithubUserLogin,omitempty"`
	GithubUserID    *int    `json:"GithubUserID,omitempty"`
	TenantVersion   int     `json:"TenantVersion"`
}

// UpdateTenantGithubCredsRequest is the request payload for UpdateTenantGithubCreds.
// All fields are optional and, when set, will be updated. Fields left as nil are not modified.
// The Version field is supplied via the If-Match header to implement optimistic concurrency.
type UpdateTenantGithubCredsRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID string `json:"-"`
	Version  int    `json:"-"`

	SkipOnboarding  *bool   `json:"SkipOnboarding,omitempty"`
	OAuthToken      *string `json:"OAuthToken,omitempty"`
	RefreshToken    *string `json:"RefreshToken,omitempty"`
	TokenExpiry     *string `json:"TokenExpiry,omitempty"`
	State           *string `json:"State,omitempty"`
	StateExpiry     *string `json:"StateExpiry,omitempty"`
	GithubUserLogin *string `json:"GithubUserLogin,omitempty"`
	GithubUserID    *int    `json:"GithubUserID,omitempty"`
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
