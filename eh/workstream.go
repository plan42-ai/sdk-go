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

// Workstream represents a workstream in Event Horizon.
type Workstream struct {
	WorkstreamID     string    `json:"WorkstreamId"`
	TenantID         string    `json:"TenantId"`
	Name             string    `json:"Name"`
	Description      string    `json:"Description"`
	CreatedAt        time.Time `json:"CreatedAt"`
	UpdatedAt        time.Time `json:"UpdatedAt"`
	Version          int       `json:"Version"`
	Paused           bool      `json:"Paused"`
	Deleted          bool      `json:"Deleted"`
	DefaultShortName string    `json:"DefaultShortName"`
	TaskCounter      int       `json:"TaskCounter"`
}

// GetWorkstreamRequest is the request payload for GetWorkstream.
type GetWorkstreamRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID       string `json:"-"`
	WorkstreamID   string `json:"-"`
	IncludeDeleted *bool  `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *GetWorkstreamRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "WorkstreamID":
		return r.WorkstreamID, true
	case "IncludeDeleted":
		return evalNullable(r.IncludeDeleted)
	default:
		return nil, false
	}
}

// GetWorkstream retrieves a workstream by ID.
// nolint:dupl
func (c *Client) GetWorkstream(ctx context.Context, req *GetWorkstreamRequest) (*Workstream, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.WorkstreamID == "" {
		return nil, fmt.Errorf("workstream id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "workstreams", url.PathEscape(req.WorkstreamID))
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

	var out Workstream
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ObjectType returns the object type for ConflictError handling.
func (Workstream) ObjectType() ObjectType { return ObjectTypeWorkstream }

// CreateWorkstreamRequest is the request payload for CreateWorkstream.
type CreateWorkstreamRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID     string `json:"-"`
	WorkstreamID string `json:"-"`

	Name             string  `json:"Name"`
	Description      string  `json:"Description"`
	DefaultShortName *string `json:"DefaultShortName,omitempty"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *CreateWorkstreamRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "WorkstreamID":
		return r.WorkstreamID, true
	case "Name":
		return r.Name, true
	case "Description":
		return r.Description, true
	case "DefaultShortName":
		return evalNullable(r.DefaultShortName)
	default:
		return nil, false
	}
}

// CreateWorkstream creates a new workstream for a tenant.
// nolint: dupl
func (c *Client) CreateWorkstream(ctx context.Context, req *CreateWorkstreamRequest) (*Workstream, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.WorkstreamID == "" {
		return nil, fmt.Errorf("workstream id is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "workstreams", url.PathEscape(req.WorkstreamID))
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

	var out Workstream
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
