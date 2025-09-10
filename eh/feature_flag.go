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
