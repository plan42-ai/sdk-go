package eh

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/debugging-sucks/clock"
	sigv4clientutil "github.com/debugging-sucks/sigv4util/client"
)

// Option configures a Client.
type Option func(c *Client)

// AuthHandler adds authentication to an HTTP request.
type AuthHandler interface {
	Authenticate(req *http.Request) error
}

type sigv4AuthHandler struct {
	cfg *aws.Config
	clk clock.Clock
}

func (h *sigv4AuthHandler) Authenticate(req *http.Request) error {
	return sigv4clientutil.AddAuthHeaders(req.Context(), req, h.cfg, h.cfg.Region, h.clk)
}

// WithSigv4Auth configures the client to use SigV4 authentication.
func WithSigv4Auth(cfg aws.Config, clk clock.Clock) Option {
	return func(c *Client) {
		c.authHandlers = append(c.authHandlers, &sigv4AuthHandler{cfg: &cfg, clk: clk})
	}
}

func WithInsecureSkipVerify() Option {
	return func(c *Client) {
		if c.HTTPClient == nil {
			c.HTTPClient = &http.Client{}
		}

		transport, _ := c.HTTPClient.Transport.(*http.Transport)
		if transport == nil {
			transport = &http.Transport{}
			c.HTTPClient.Transport = transport
		}

		if transport.TLSClientConfig == nil {
			// #nosec: G402: TLS MinVersion too low.
			//    This is literally configured 6 lines below.
			transport.TLSClientConfig = &tls.Config{}
		}

		transport.TLSClientConfig.InsecureSkipVerify = true
		if transport.TLSClientConfig.MinVersion < tls.VersionTLS12 {
			transport.TLSClientConfig.MinVersion = tls.VersionTLS12
		}
	}
}

// Client is the API client for Event Horizon.
type Client struct {
	// BaseURL is the HTTP base url for the API, e.g. https://api.example.com
	BaseURL *url.URL

	// HTTPClient is the http client used to make requests. If nil, http.DefaultClient is used.
	HTTPClient *http.Client

	// authHandler is used to add authentication to requests.
	authHandlers []AuthHandler
}

// NewClient returns a new API client with the given baseURL.
func NewClient(baseURL string, opts ...Option) *Client {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		panic(fmt.Sprintf("invalid base URL: %v", err))
	}
	c := &Client{BaseURL: parsedURL, HTTPClient: nil}
	for _, opt := range opts {
		opt(c)
	}
	if c.HTTPClient == nil {
		c.HTTPClient = http.DefaultClient
	}
	return c
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *Client) authenticate(delegatedAuth DelegatedAuthInfo, req *http.Request) error {
	for _, handler := range c.authHandlers {
		if err := handler.Authenticate(req); err != nil {
			return fmt.Errorf("failed to authenticate request: %w", err)
		}
	}
	if delegatedAuth.AuthType != nil && delegatedAuth.JWT != nil {
		req.Header.Set("X-Event-Horizon-Delegating-Authorization", fmt.Sprintf("%s %s", *delegatedAuth.AuthType, *delegatedAuth.JWT))
	}
	return nil
}

// TenantType is the type of tenant.
type TenantType string

const (
	TenantTypeUser         TenantType = "User"
	TenantTypeOrganization TenantType = "Organization"
	TenantTypeEnterprise   TenantType = "Enterprise"
)

type DelegatedAuthInfo struct {
	AuthType *AuthorizationType `json:"-"`
	JWT      *string            `json:"-"`
}

// CreateTenantRequest is the request payload for CreateTenant.
type CreateTenantRequest struct {
	DelegatedAuthInfo
	TenantID       string     `json:"-"`
	Type           TenantType `json:"Type"`
	FullName       *string    `json:"FullName,omitempty"`
	OrgName        *string    `json:"OrgName,omitempty"`
	EnterpriseName *string    `json:"EnterpriseName,omitempty"`
	Email          *string    `json:"Email,omitempty"`
	FirstName      *string    `json:"FirstName,omitempty"`
	LastName       *string    `json:"LastName,omitempty"`
	InitialOwner   *string    `json:"InitialOwner,omitempty"`
	PictureURL     *string    `json:"PictureUrl,omitempty"`
}

// nolint: goconst
func (r *CreateTenantRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "Type":
		return r.Type, true
	case "FullName":
		return evalNullable(r.FullName)
	case "OrgName":
		return evalNullable(r.OrgName)
	case "EnterpriseName":
		return evalNullable(r.EnterpriseName)
	case "Email":
		return evalNullable(r.Email)
	case "FirstName":
		return evalNullable(r.FirstName)
	case "LastName":
		return evalNullable(r.LastName)
	case "InitialOwner":
		return evalNullable(r.InitialOwner)
	case "PictureUrl":
		return evalNullable(r.PictureURL)
	default:
		return nil, false
	}
}

// GetTenantRequest is the request for GetTenant.
type GetTenantRequest struct {
	DelegatedAuthInfo
	TenantID string
}

// GetCurrentUserRequest is the request for GetCurrentUser.
type GetCurrentUserRequest struct {
	DelegatedAuthInfo
}

// nolint: goconst
func (r *GetCurrentUserRequest) GetField(_ string) (any, bool) {
	return nil, false
}

// nolint: goconst
func (r *GetTenantRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	default:
		return nil, false
	}
}

// Tenant is the representation of a tenant returned by the API.
type Tenant struct {
	TenantID       string     `json:"TenantId"`
	Type           TenantType `json:"Type"`
	Version        int        `json:"Version"`
	Deleted        bool       `json:"Deleted"`
	CreatedAt      time.Time  `json:"CreatedAt"`
	UpdatedAt      time.Time  `json:"UpdatedAt"`
	FullName       *string    `json:"FullName,omitempty"`
	OrgName        *string    `json:"OrgName,omitempty"`
	EnterpriseName *string    `json:"EnterpriseName,omitempty"`
	Email          *string    `json:"Email,omitempty"`
	FirstName      *string    `json:"FirstName,omitempty"`
	LastName       *string    `json:"LastName,omitempty"`
	PictureURL     *string    `json:"PictureUrl,omitempty"`
}

func (t Tenant) ObjectType() ObjectType {
	return ObjectTypeTenant
}

type HTTPError interface {
	error
	Code() int
	Unwrap() error
}

// Error represents an error returned by the API.
type Error struct {
	ResponseCode int    `json:"ResponseCode"`
	Message      string `json:"Message"`
	ErrorType    string `json:"ErrorType"`
	Cause        error  `json:"-"`
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Code() int {
	return e.ResponseCode
}

func (e *Error) Unwrap() error {
	return e.Cause
}

type ObjectType string

const (
	ObjectTypeTenant               ObjectType = "Tenant"
	ObjectTypeEnvironment          ObjectType = "Environment"
	ObjectTypeWebUITokenThumbprint ObjectType = "WebUITokenThumbprint"
	ObjectTypeTurn                 ObjectType = "Turn"
	ObjectTypeTask                 ObjectType = "Task"
	ObjectTypeGithubOrg            ObjectType = "GithubOrg"
	ObjectTypeTenantGithubOrg      ObjectType = "TenantGithubOrg"
)

type ConflictObj interface {
	ObjectType() ObjectType
}

// ConflictError represents a conflict error returned by the API.
type ConflictError struct {
	ResponseCode int
	Message      string
	ErrorType    string
	Current      ConflictObj
	Cause        error `json:"-"`
}

type conflictError struct {
	ResponseCode int
	Message      string
	ErrorType    string
	CurrentType  ObjectType
	Current      json.RawMessage
}

func (e *ConflictError) Error() string {
	return e.Message
}

func (e *ConflictError) Code() int {
	return e.ResponseCode
}

func (e *ConflictError) Unwrap() error {
	return e.Cause
}

func (e *ConflictError) UnmarshalJSON(b []byte) error {
	var tmp conflictError
	err := json.Unmarshal(b, &tmp)
	if err != nil {
		return err
	}

	var current ConflictObj
	if tmp.Current != nil {
		switch tmp.CurrentType {
		case ObjectTypeTenant:
			current = &Tenant{}
		case ObjectTypeEnvironment:
			current = &Environment{}
		case ObjectTypeWebUITokenThumbprint:
			current = &WebUITokenThumbprint{}
		case ObjectTypeTurn:
			current = &Turn{}
		case ObjectTypeTask:
			current = &Task{}
		case ObjectTypeGithubOrg:
			current = &GithubOrg{}
		case ObjectTypeTenantGithubOrg:
			current = &TenantGithubOrg{}
		default:
			return fmt.Errorf("unknown object type %s", tmp.CurrentType)
		}

		err = json.Unmarshal(tmp.Current, current)
		if err != nil {
			return err
		}
	}

	*e = ConflictError{
		ResponseCode: tmp.ResponseCode,
		Message:      tmp.Message,
		ErrorType:    tmp.ErrorType,
		Current:      current,
	}
	return nil
}

func (e ConflictError) MarshalJSON() ([]byte, error) {
	tmp := conflictError{
		ResponseCode: e.ResponseCode,
		Message:      e.Message,
		ErrorType:    e.ErrorType,
	}
	var err error

	if e.Current != nil {
		tmp.CurrentType = e.Current.ObjectType()
		tmp.Current, err = json.Marshal(e.Current)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(tmp)
}

func coalesce[T comparable](values ...T) T {
	var zero T
	for _, v := range values {
		if v != zero {
			return v
		}
	}
	return zero
}

func decodeError(resp *http.Response) error {
	decoder := json.NewDecoder(resp.Body)
	var err error
	switch resp.StatusCode {
	case http.StatusConflict:
		err = &ConflictError{}
	default:
		err = &Error{}
	}

	err2 := decoder.Decode(err)
	return coalesce(err, err2)
}

// CreateTenant creates a new tenant.
func (c *Client) CreateTenant(ctx context.Context, req *CreateTenantRequest) (*Tenant, error) {
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

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID))
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

	var tenant Tenant
	if err := json.NewDecoder(resp.Body).Decode(&tenant); err != nil {
		return nil, err
	}
	return &tenant, nil
}

// GetTenant retrieves a tenant by ID.
func (c *Client) GetTenant(ctx context.Context, req *GetTenantRequest) (*Tenant, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID))
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

	var tenant Tenant
	if err := json.NewDecoder(resp.Body).Decode(&tenant); err != nil {
		return nil, err
	}
	return &tenant, nil
}

// GetCurrentUser retrieves the tenant information for the current user.
func (c *Client) GetCurrentUser(ctx context.Context, req *GetCurrentUserRequest) (*Tenant, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	u := c.BaseURL.JoinPath("v1", "current-user")
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

	var tenant Tenant
	if err := json.NewDecoder(resp.Body).Decode(&tenant); err != nil {
		return nil, err
	}
	return &tenant, nil
}

// GenerateWebUITokenRequest is the request for GenerateWebUIToken.
type GenerateWebUITokenRequest struct {
	DelegatedAuthInfo
	TenantID string
	TokenID  string
}

// nolint: goconst
func (r *GenerateWebUITokenRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "TokenID":
		return r.TokenID, true
	default:
		return nil, false
	}
}

// GenerateWebUITokenResponse is the response from GenerateWebUIToken.
type GenerateWebUITokenResponse struct {
	JWT string `json:"JWT"`
}

// GenerateWebUIToken generates a new Web UI token.
func (c *Client) GenerateWebUIToken(ctx context.Context, req *GenerateWebUITokenRequest) (*GenerateWebUITokenResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.TokenID == "" {
		return nil, fmt.Errorf("token id is required")
	}
	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "ui-tokens", url.PathEscape(req.TokenID))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), nil)
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

	if resp.StatusCode != http.StatusCreated {
		return nil, decodeError(resp)
	}

	var out GenerateWebUITokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

type WebUITokenThumbprint struct {
	TenantID            string
	TokenID             string
	CreatedAt           time.Time
	ExpiresAt           time.Time
	Revoked             bool
	SignatureHashBase64 []byte
}

func (t WebUITokenThumbprint) ObjectType() ObjectType {
	return ObjectTypeWebUITokenThumbprint
}

// TurnLog represents a single log entry for a turn.
type TurnLog struct {
	Timestamp time.Time `json:"Timestamp"`
	Message   string    `json:"Message"`
}

// LastTurnLog represents the last log entry for a turn.
type LastTurnLog struct {
	Index     int       `json:"Index"`
	Timestamp time.Time `json:"Timestamp"`
	Message   string    `json:"Message"`
}

// UploadTurnLogsRequest is the request payload for UploadTurnLogs.
type UploadTurnLogsRequest struct {
	DelegatedAuthInfo
	TenantID  string    `json:"-"`
	TaskID    string    `json:"-"`
	TurnIndex int       `json:"-"`
	Version   int       `json:"-"`
	Index     int       `json:"Index"`
	Logs      []TurnLog `json:"Logs"`
}

// GetLastTurnLogRequest is the request payload for GetLastTurnLog.
type GetLastTurnLogRequest struct {
	DelegatedAuthInfo
	TenantID       string `json:"-"`
	TaskID         string `json:"-"`
	TurnIndex      int    `json:"-"`
	IncludeDeleted *bool  `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *GetLastTurnLogRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "TaskID":
		return r.TaskID, true
	case "TurnIndex":
		return r.TurnIndex, true
	case "IncludeDeleted":
		return evalNullable(r.IncludeDeleted)
	default:
		return nil, false
	}
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *UploadTurnLogsRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "TaskID":
		return r.TaskID, true
	case "TurnIndex":
		return r.TurnIndex, true
	case "Version":
		return r.Version, true
	case "Index":
		return r.Index, true
	case "Logs":
		return r.Logs, true
	default:
		return nil, false
	}
}

// UploadTurnLogsResponse is the response from UploadTurnLogs.
type UploadTurnLogsResponse struct {
	Version int `json:"Version"`
}

// UploadTurnLogs uploads logs for a turn.
func (c *Client) UploadTurnLogs(ctx context.Context, req *UploadTurnLogsRequest) (*UploadTurnLogsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.TaskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	if req.TurnIndex < 0 {
		return nil, fmt.Errorf("turn index is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "tasks", url.PathEscape(req.TaskID), "turns", strconv.Itoa(req.TurnIndex), "logs")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(bodyBytes))
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

	var out UploadTurnLogsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// StreamTurnLogsRequest is the request payload for StreamTurnLogs.
type StreamTurnLogsRequest struct {
	DelegatedAuthInfo
	TenantID       string `json:"-"`
	TaskID         string `json:"-"`
	TurnIndex      int    `json:"-"`
	LastEventID    *int   `json:"-"`
	IncludeDeleted *bool  `json:"-"`
}

// GetField retrieves the value of a field by name.
func (r *StreamTurnLogsRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "TaskID":
		return r.TaskID, true
	case "TurnIndex":
		return r.TurnIndex, true
	case "LastEventID":
		return evalNullable(r.LastEventID)
	case "IncludeDeleted":
		return evalNullable(r.IncludeDeleted)
	default:
		return nil, false
	}
}

// StreamTurnLogs streams logs for a turn. The caller is responsible for closing the returned reader.
func (c *Client) StreamTurnLogs(ctx context.Context, req *StreamTurnLogsRequest) (io.ReadCloser, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.TaskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	if req.TurnIndex < 0 {
		return nil, fmt.Errorf("turn index is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "tasks", url.PathEscape(req.TaskID), "turns", strconv.Itoa(req.TurnIndex), "logs")
	q := u.Query()
	if req.IncludeDeleted != nil {
		q.Set("includeDeleted", strconv.FormatBool(*req.IncludeDeleted))
	}
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "text/event-stream")
	if req.LastEventID != nil {
		httpReq.Header.Set("Last-Event-ID", strconv.Itoa(*req.LastEventID))
	}

	if err := c.authenticate(req.DelegatedAuthInfo, httpReq); err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNoContent {
		resp.Body.Close()
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, decodeError(resp)
	}

	return resp.Body, nil
}

// GetLastTurnLog retrieves the last log entry for a turn.
func (c *Client) GetLastTurnLog(ctx context.Context, req *GetLastTurnLogRequest) (*LastTurnLog, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.TaskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	if req.TurnIndex < 0 {
		return nil, fmt.Errorf("turn index is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "tasks", url.PathEscape(req.TaskID), "turns", strconv.Itoa(req.TurnIndex), "logs", "last")
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

	var out LastTurnLog
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

// AssociateGithubOrgTenantRequest is the request payload for AssociateGithubOrgTenant.
type AssociateGithubOrgTenantRequest struct {
	DelegatedAuthInfo
	TenantID          string    `json:"-"`
	OrgID             string    `json:"-"`
	GithubUserID      int       `json:"GithubUserID"`
	GithubUsername    string    `json:"GithubUsername"`
	OAuthToken        string    `json:"OAuthToken"`
	OAuthRefreshToken string    `json:"OAuthRefreshToken"`
	ExpiresAt         time.Time `json:"ExpiresAt"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *AssociateGithubOrgTenantRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "OrgID":
		return r.OrgID, true
	case "GithubUserID":
		return r.GithubUserID, true
	case "GithubUsername":
		return r.GithubUsername, true
	case "OAuthToken":
		return r.OAuthToken, true
	case "OAuthRefreshToken":
		return r.OAuthRefreshToken, true
	case "ExpiresAt":
		return r.ExpiresAt, true
	default:
		return nil, false
	}
}

// AssociateGithubOrgTenant associates a github org with a tenant.
func (c *Client) AssociateGithubOrgTenant(ctx context.Context, req *AssociateGithubOrgTenantRequest) (*TenantGithubOrg, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.OrgID == "" {
		return nil, fmt.Errorf("org id is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "github", "orgs", url.PathEscape(req.OrgID))
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

	var out TenantGithubOrg
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListTenantGithubOrgsRequest is the request for ListTenantGithubOrgs.
type ListTenantGithubOrgsRequest struct {
	DelegatedAuthInfo
	TenantID       string
	MaxResults     *int
	Token          *string
	IncludeDeleted *bool
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *ListTenantGithubOrgsRequest) GetField(name string) (any, bool) {
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

// ListTenantGithubOrgsResponse is the response from ListTenantGithubOrgs.
type ListTenantGithubOrgsResponse struct {
	Orgs      []TenantGithubOrg `json:"Orgs"`
	NextToken *string           `json:"NextToken"`
}

// ListTenantGithubOrgs lists all github orgs associated with a tenant.
func (c *Client) ListTenantGithubOrgs(ctx context.Context, req *ListTenantGithubOrgsRequest) (*ListTenantGithubOrgsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "github", "orgs")
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

	var out ListTenantGithubOrgsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateTenantGithubOrgAssociationRequest is the request payload for UpdateTenantGithubOrgAssociation.
type UpdateTenantGithubOrgAssociationRequest struct {
	DelegatedAuthInfo
	TenantID          string     `json:"-"`
	OrgID             string     `json:"-"`
	Version           int        `json:"-"`
	OAuthToken        *string    `json:"OAuthToken,omitempty"`
	OAuthRefreshToken *string    `json:"OAuthRefreshToken,omitempty"`
	ExpiresAt         *time.Time `json:"ExpiresAt,omitempty"`
	Deleted           *bool      `json:"Deleted,omitempty"`
	GithubUsername    *string    `json:"GithubUserName,omitempty"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *UpdateTenantGithubOrgAssociationRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "OrgID":
		return r.OrgID, true
	case "Version":
		return r.Version, true
	case "OAuthToken":
		return r.OAuthToken, true
	case "OAuthRefreshToken":
		return r.OAuthRefreshToken, true
	case "ExpiresAt":
		return r.ExpiresAt, true
	case "Deleted":
		return r.Deleted, true
	case "GithubUsername":
		return r.GithubUsername, true
	default:
		return nil, false
	}
}

// UpdateTenantGithubOrgAssociation updates the association between a tenant and a github org.
// nolint: dupl
func (c *Client) UpdateTenantGithubOrgAssociation(ctx context.Context, req *UpdateTenantGithubOrgAssociationRequest) (*TenantGithubOrg, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.OrgID == "" {
		return nil, fmt.Errorf("org id is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "github", "orgs", url.PathEscape(req.OrgID))
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

	var out TenantGithubOrg
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteTenantGithubOrgAssociationRequest is the request payload for DeleteTenantGithubOrgAssociation.
type DeleteTenantGithubOrgAssociationRequest struct {
	DelegatedAuthInfo
	TenantID string `json:"-"`
	OrgID    string `json:"-"`
	Version  int    `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *DeleteTenantGithubOrgAssociationRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "OrgID":
		return r.OrgID, true
	case "Version":
		return r.Version, true
	default:
		return nil, false
	}
}

// DeleteTenantGithubOrgAssociation soft deletes the association between a github org and a tenant.
// nolint: dupl
func (c *Client) DeleteTenantGithubOrgAssociation(ctx context.Context, req *DeleteTenantGithubOrgAssociationRequest) error {
	if req == nil {
		return fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return fmt.Errorf("tenant id is required")
	}
	if req.OrgID == "" {
		return fmt.Errorf("org id is required")
	}
	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "github", "orgs", url.PathEscape(req.OrgID))
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

// GetTenantGithubOrgAssociationRequest is the request for GetTenantGithubOrgAssociation.
type GetTenantGithubOrgAssociationRequest struct {
	DelegatedAuthInfo
	TenantID       string `json:"-"`
	OrgID          string `json:"-"`
	IncludeDeleted *bool  `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *GetTenantGithubOrgAssociationRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "OrgID":
		return r.OrgID, true
	case "IncludeDeleted":
		return evalNullable(r.IncludeDeleted)
	default:
		return nil, false
	}
}

// GetTenantGithubOrgAssociation retrieves the association between a github org and a tenant.
func (c *Client) GetTenantGithubOrgAssociation(ctx context.Context, req *GetTenantGithubOrgAssociationRequest) (*TenantGithubOrg, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.OrgID == "" {
		return nil, fmt.Errorf("org id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "github", "orgs", url.PathEscape(req.OrgID))
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

	var out TenantGithubOrg
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
