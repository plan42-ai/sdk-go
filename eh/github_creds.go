package eh

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
