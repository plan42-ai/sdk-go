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
