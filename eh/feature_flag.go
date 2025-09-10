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

// FeatureFlag represents a feature flag.
type FeatureFlag struct {
	Name        string    `json:"Name"`
	Description string    `json:"Description"`
	DefaultPct  float64   `json:"DefaultPct"`
	CreatedAt   time.Time `json:"CreatedAt"`
	UpdatedAt   time.Time `json:"UpdatedAt"`
	Version     int       `json:"Version"`
	Deleted     bool      `json:"Deleted"`
}

// ObjectType returns the object type for ConflictError handling.
func (FeatureFlag) ObjectType() ObjectType { return ObjectTypeFeatureFlag }

// CreateFeatureFlagRequest is the request payload for CreateFeatureFlag.
type CreateFeatureFlagRequest struct {
	DelegatedAuthInfo
	FlagName    string  `json:"-"`
	Description string  `json:"Description"`
	DefaultPct  float64 `json:"DefaultPct"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *CreateFeatureFlagRequest) GetField(name string) (any, bool) {
	switch name {
	case "FlagName":
		return r.FlagName, true
	case "Description":
		return r.Description, true
	case "DefaultPct":
		return r.DefaultPct, true
	default:
		return nil, false
	}
}

// CreateFeatureFlag creates a new feature flag.
// nolint: dupl
func (c *Client) CreateFeatureFlag(ctx context.Context, req *CreateFeatureFlagRequest) (*FeatureFlag, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.FlagName == "" {
		return nil, fmt.Errorf("flag name is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "featureflags", url.PathEscape(req.FlagName))
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

	var out FeatureFlag
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetFeatureFlagRequest is the request payload for GetFeatureFlag.
type GetFeatureFlagRequest struct {
	DelegatedAuthInfo
	FlagName       string `json:"-"`
	IncludeDeleted *bool  `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *GetFeatureFlagRequest) GetField(name string) (any, bool) {
	switch name {
	case "FlagName":
		return r.FlagName, true
	case "IncludeDeleted":
		return evalNullable(r.IncludeDeleted)
	default:
		return nil, false
	}
}

// GetFeatureFlag retrieves a feature flag by name.
// nolint: dupl
func (c *Client) GetFeatureFlag(ctx context.Context, req *GetFeatureFlagRequest) (*FeatureFlag, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.FlagName == "" {
		return nil, fmt.Errorf("flag name is required")
	}

	u := c.BaseURL.JoinPath("v1", "featureflags", url.PathEscape(req.FlagName))
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

	var out FeatureFlag
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListFeatureFlagsRequest is the request for ListFeatureFlags.
type ListFeatureFlagsRequest struct {
	MaxResults     *int
	Token          *string
	IncludeDeleted *bool
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *ListFeatureFlagsRequest) GetField(name string) (any, bool) {
	switch name {
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

// ListFeatureFlagsResponse is the response from ListFeatureFlags.
type ListFeatureFlagsResponse struct {
	FeatureFlags []FeatureFlag `json:"FeatureFlags"`
	NextToken    *string       `json:"NextToken"`
}

// ListFeatureFlags lists all feature flags in the service.
func (c *Client) ListFeatureFlags(ctx context.Context, req *ListFeatureFlagsRequest) (*ListFeatureFlagsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	u := c.BaseURL.JoinPath("v1", "featureflags")
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

	var out ListFeatureFlagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateFeatureFlagRequest is the request payload for UpdateFeatureFlag.
type UpdateFeatureFlagRequest struct {
	DelegatedAuthInfo
	FlagName    string   `json:"-"`
	Version     int      `json:"-"`
	Description *string  `json:"Description,omitempty"`
	DefaultPct  *float64 `json:"DefaultPct,omitempty"`
	Deleted     *bool    `json:"Deleted,omitempty"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *UpdateFeatureFlagRequest) GetField(name string) (any, bool) {
	switch name {
	case "FlagName":
		return r.FlagName, true
	case "Version": //nolint: goconst
		return r.Version, true
	case "Description":
		return evalNullable(r.Description)
	case "DefaultPct":
		return evalNullable(r.DefaultPct)
	case "Deleted":
		return evalNullable(r.Deleted)
	default:
		return nil, false
	}
}

// UpdateFeatureFlag updates a feature flag.
// nolint: dupl
func (c *Client) UpdateFeatureFlag(ctx context.Context, req *UpdateFeatureFlagRequest) (*FeatureFlag, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.FlagName == "" {
		return nil, fmt.Errorf("flag name is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "featureflags", url.PathEscape(req.FlagName))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("If-Match", strconv.Itoa(req.Version))

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

	var out FeatureFlag
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteFeatureFlagRequest is the request payload for DeleteFeatureFlag.
type DeleteFeatureFlagRequest struct {
	DelegatedAuthInfo
	FlagName string `json:"-"`
	Version  int    `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *DeleteFeatureFlagRequest) GetField(name string) (any, bool) {
	switch name {
	case "FlagName":
		return r.FlagName, true
	case "Version":
		return r.Version, true
	default:
		return nil, false
	}
}

// DeleteFeatureFlag deletes a feature flag.
// nolint: dupl
func (c *Client) DeleteFeatureFlag(ctx context.Context, req *DeleteFeatureFlagRequest) error {
	if req == nil {
		return fmt.Errorf("req is nil")
	}
	if req.FlagName == "" {
		return fmt.Errorf("flag name is required")
	}
	u := c.BaseURL.JoinPath("v1", "featureflags", url.PathEscape(req.FlagName))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, u.String(), nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("If-Match", strconv.Itoa(req.Version))

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

// DeleteFeatureFlagOverrideRequest is the request payload for DeleteFeatureFlagOverride.
type DeleteFeatureFlagOverrideRequest struct {
	DelegatedAuthInfo
	TenantID string `json:"-"`
	FlagName string `json:"-"`
	Version  int    `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *DeleteFeatureFlagOverrideRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "FlagName":
		return r.FlagName, true
	case "Version":
		return r.Version, true
	default:
		return nil, false
	}
}

// DeleteFeatureFlagOverride deletes a feature flag override for a tenant.
// nolint: dupl
func (c *Client) DeleteFeatureFlagOverride(ctx context.Context, req *DeleteFeatureFlagOverrideRequest) error {
	if req == nil {
		return fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return fmt.Errorf("tenant id is required")
	}
	if req.FlagName == "" {
		return fmt.Errorf("flag name is required")
	}
	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "featureFlagOverrides", url.PathEscape(req.FlagName))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, u.String(), nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("If-Match", strconv.Itoa(req.Version))

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

// GetFeatureFlagOverrideRequest is the request payload for GetFeatureFlagOverride.
type GetFeatureFlagOverrideRequest struct {
	DelegatedAuthInfo
	TenantID       string `json:"-"`
	FlagName       string `json:"-"`
	IncludeDeleted *bool  `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *GetFeatureFlagOverrideRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "FlagName":
		return r.FlagName, true
	case "IncludeDeleted":
		return evalNullable(r.IncludeDeleted)
	default:
		return nil, false
	}
}

// GetFeatureFlagOverride retrieves a feature flag override for a tenant.
// nolint: dupl
func (c *Client) GetFeatureFlagOverride(ctx context.Context, req *GetFeatureFlagOverrideRequest) (*FeatureFlagOverride, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.FlagName == "" {
		return nil, fmt.Errorf("flag name is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "featureFlagOverrides", url.PathEscape(req.FlagName))
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

	var out FeatureFlagOverride
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateFeatureFlagOverrideRequest is the request payload for UpdateFeatureFlagOverride.
type UpdateFeatureFlagOverrideRequest struct {
	DelegatedAuthInfo
	TenantID string `json:"-"`
	FlagName string `json:"-"`
	Version  int    `json:"-"`
	Enabled  *bool  `json:"Enabled,omitempty"`
	Deleted  *bool  `json:"Deleted,omitempty"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *UpdateFeatureFlagOverrideRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "FlagName":
		return r.FlagName, true
	case "Version":
		return r.Version, true
	case "Enabled":
		return evalNullable(r.Enabled)
	case "Deleted":
		return evalNullable(r.Deleted)
	default:
		return nil, false
	}
}

// UpdateFeatureFlagOverride updates a feature flag override for a tenant.
// nolint: dupl
func (c *Client) UpdateFeatureFlagOverride(ctx context.Context, req *UpdateFeatureFlagOverrideRequest) (*FeatureFlagOverride, error) {
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
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("If-Match", strconv.Itoa(req.Version))

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

	var out FeatureFlagOverride
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
