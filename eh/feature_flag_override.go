package eh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// FeatureFlagOverride represents a feature flag override.
type FeatureFlagOverride struct {
	FlagName  string    `json:"FlagName"`
	TenantID  string    `json:"TenantID"`
	Enabled   bool      `json:"Enabled"`
	CreatedAt time.Time `json:"CreatedAt"`
	UpdatedAt time.Time `json:"UpdatedAt"`
	Version   int       `json:"Version"`
	Deleted   bool      `json:"Deleted"`
}

// ObjectType returns the object type for ConflictError handling.
func (FeatureFlagOverride) ObjectType() ObjectType { return ObjectTypeFeatureFlagOverride }

// CreateFeatureFlagOverrideRequest is the request payload for CreateFeatureFlagOverride.
type CreateFeatureFlagOverrideRequest struct {
	DelegatedAuthInfo
	TenantID string `json:"-"`
	FlagName string `json:"-"`
	Enabled  bool   `json:"Enabled"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *CreateFeatureFlagOverrideRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "FlagName":
		return r.FlagName, true
	case "Enabled":
		return r.Enabled, true
	default:
		return nil, false
	}
}

// CreateFeatureFlagOverride creates a new feature flag override for a tenant.
// nolint: dupl
func (c *Client) CreateFeatureFlagOverride(ctx context.Context, req *CreateFeatureFlagOverrideRequest) (*FeatureFlagOverride, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.FlagName == "" {
		return nil, fmt.Errorf("flag name is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "featureFlagOverrides", url.PathEscape(req.FlagName))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")

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

	var out FeatureFlagOverride
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
