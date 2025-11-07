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
	Version       int       `json:"Version"`
}

// ObjectType returns the object type for ConflictError handling.
func (Runner) ObjectType() ObjectType { return ObjectTypeRunner }

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
