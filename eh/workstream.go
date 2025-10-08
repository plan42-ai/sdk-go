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

// ObjectType returns the object type for ConflictError handling.
func (Workstream) ObjectType() ObjectType { return ObjectTypeWorkstream }

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

// UpdateWorkstreamRequest is the request payload for UpdateWorkstream.
type UpdateWorkstreamRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID     string `json:"-"`
	WorkstreamID string `json:"-"`
	Version      int    `json:"-"`

	Name             *string `json:"Name,omitempty"`
	Description      *string `json:"Description,omitempty"`
	Paused           *bool   `json:"Paused,omitempty"`
	Deleted          *bool   `json:"Deleted,omitempty"`
	DefaultShortName *string `json:"DefaultShortName,omitempty"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *UpdateWorkstreamRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "WorkstreamID":
		return r.WorkstreamID, true
	case "Version":
		return r.Version, true
	case "Name":
		return evalNullable(r.Name)
	case "Description":
		return evalNullable(r.Description)
	case "Paused":
		return evalNullable(r.Paused)
	case "Deleted":
		return evalNullable(r.Deleted)
	case "DefaultShortName":
		return evalNullable(r.DefaultShortName)
	default:
		return nil, false
	}
}

// UpdateWorkstream updates an existing workstream.
// nolint: dupl
func (c *Client) UpdateWorkstream(ctx context.Context, req *UpdateWorkstreamRequest) (*Workstream, error) {
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

	var out Workstream
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListWorkstreamsRequest is the request for ListWorkstreams.
type ListWorkstreamsRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID       string
	MaxResults     *int
	Token          *string
	IncludeDeleted *bool
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *ListWorkstreamsRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
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

// ListWorkstreamsResponse is the response from ListWorkstreams.
type ListWorkstreamsResponse struct {
	Workstreams []Workstream `json:"Workstreams"`
	NextToken   *string      `json:"NextToken"`
}

// ListWorkstreams lists the workstreams for a tenant.
// nolint:dupl
func (c *Client) ListWorkstreams(ctx context.Context, req *ListWorkstreamsRequest) (*ListWorkstreamsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "workstreams")
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

	var out ListWorkstreamsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteWorkstreamRequest is the request payload for DeleteWorkstream.
type DeleteWorkstreamRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID     string `json:"-"`
	WorkstreamID string `json:"-"`
	Version      int    `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *DeleteWorkstreamRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "WorkstreamID":
		return r.WorkstreamID, true
	case "Version":
		return r.Version, true
	default:
		return nil, false
	}
}

// DeleteWorkstream soft-deletes a workstream.
// nolint: dupl
func (c *Client) DeleteWorkstream(ctx context.Context, req *DeleteWorkstreamRequest) error {
	if req == nil {
		return fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return fmt.Errorf("tenant id is required")
	}
	if req.WorkstreamID == "" {
		return fmt.Errorf("workstream id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "workstreams", url.PathEscape(req.WorkstreamID))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, u.String(), nil)
	if err != nil {
		return err
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("If-Match", strconv.Itoa(req.Version))

	processFeatureFlags(httpReq, req.FeatureFlags)

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

// AddWorkstreamShortNameRequest is the request payload for AddWorkstreamShortName.
type AddWorkstreamShortNameRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID     string `json:"-"`
	WorkstreamID string `json:"-"`
	Name         string `json:"-"`
	Version      int    `json:"-"`
}

// WorkstreamShortName represents a short name for a workstream.
type WorkstreamShortName struct {
	Name              string `json:"Name"`
	WorkstreamID      string `json:"WorkstreamID"`
	WorkstreamVersion int    `json:"WorkstreamVersion"`
}

// ObjectType returns the object type for ConflictError handling.
func (WorkstreamShortName) ObjectType() ObjectType { return ObjectTypeWorkstream }

// ListWorkstreamShortNamesRequest is the request for ListWorkstreamShortNames.
type ListWorkstreamShortNamesRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID       string
	MaxResults     *int
	Token          *string
	IncludeDeleted *bool
	WorkstreamID   *string
}

// GetField retrieves the value of a field by name.
// nolint:goconst
func (r *ListWorkstreamShortNamesRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "MaxResults":
		return evalNullable(r.MaxResults)
	case "Token":
		return evalNullable(r.Token)
	case "IncludeDeleted":
		return evalNullable(r.IncludeDeleted)
	case "WorkstreamID":
		return evalNullable(r.WorkstreamID)
	default:
		return nil, false
	}
}

// ListWorkstreamShortNamesResponse is the response from ListWorkstreamShortNames.
type ListWorkstreamShortNamesResponse struct {
	ShortNames []WorkstreamShortName `json:"ShortNames"`
	NextToken  *string               `json:"NextToken"`
}

// ListWorkstreamShortNames lists short names for a tenant (optionally filtered by workstream).
// nolint:dupl
func (c *Client) ListWorkstreamShortNames(ctx context.Context, req *ListWorkstreamShortNamesRequest) (*ListWorkstreamShortNamesResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "shortnames")
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
	if req.WorkstreamID != nil {
		q.Set("workstreamID", *req.WorkstreamID)
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

	var out ListWorkstreamShortNamesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *AddWorkstreamShortNameRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "WorkstreamID":
		return r.WorkstreamID, true
	case "Name":
		return r.Name, true
	case "Version":
		return r.Version, true
	default:
		return nil, false
	}
}

// AddWorkstreamShortName adds a short name to a workstream.
// nolint: dupl
func (c *Client) AddWorkstreamShortName(ctx context.Context, req *AddWorkstreamShortNameRequest) error {
	if req == nil {
		return fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return fmt.Errorf("tenant id is required")
	}
	if req.WorkstreamID == "" {
		return fmt.Errorf("workstream id is required")
	}
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "workstreams", url.PathEscape(req.WorkstreamID), "shortnames", url.PathEscape(req.Name))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), nil)
	if err != nil {
		return err
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("If-Match", strconv.Itoa(req.Version))

	processFeatureFlags(httpReq, req.FeatureFlags)

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
