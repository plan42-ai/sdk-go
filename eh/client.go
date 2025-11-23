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

type apiTokenHandler struct {
	Token string
}

func (a apiTokenHandler) Authenticate(req *http.Request) error {
	req.Header.Set("Authorization", fmt.Sprintf("%s %s", AuthorizationTypeAPIToken, a.Token))
	return nil
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

func WithAPIToken(token string) Option {
	return func(c *Client) {
		c.authHandlers = append(c.authHandlers, &apiTokenHandler{Token: token})
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
	FeatureFlags
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
	FeatureFlags
	DelegatedAuthInfo
	TenantID string
}

// GetCurrentUserRequest is the request for GetCurrentUser.
type GetCurrentUserRequest struct {
	FeatureFlags
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

// ListTenantsRequest is the request for ListTenants.
type ListTenantsRequest struct {
	MaxResults *int
	Token      *string
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *ListTenantsRequest) GetField(name string) (any, bool) {
	switch name {
	case "MaxResults":
		return evalNullable(r.MaxResults)
	case "Token":
		return evalNullable(r.Token)
	default:
		return nil, false
	}
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
	ObjectTypeTenant                 ObjectType = "Tenant"
	ObjectTypeEnvironment            ObjectType = "Environment"
	ObjectTypeWebUITokenThumbprint   ObjectType = "WebUITokenThumbprint"
	ObjectTypeTurn                   ObjectType = "Turn"
	ObjectTypeTask                   ObjectType = "Task"
	ObjectTypeRunner                 ObjectType = "Runner"
	ObjectTypeRunnerToken            ObjectType = "RunnerToken"
	ObjectTypeGithubOrg              ObjectType = "GithubOrg"
	ObjectTypeGithubConnection       ObjectType = "GithubConnection"
	ObjectTypeFeatureFlag            ObjectType = "FeatureFlag"
	ObjectTypeFeatureFlagOverride    ObjectType = "FeatureFlagOverride"
	ObjectTypeWorkstream             ObjectType = "Workstream"
	ObjectTypeWorkstreamShortName    ObjectType = "WorkstreamShortName"
	ObjectTypeTenantGithubCreds      ObjectType = "TenantGithubCreds" // #nosec: G101: This is not a hard coded credential, it's an enum member that contains the work "cred".
	ObjectTypeWorkstreamTaskConflict ObjectType = "WorkstreamTaskConflict"
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
	if tmp.Current != nil && tmp.CurrentType != "" {
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
		case ObjectTypeRunner:
			current = &Runner{}
		case ObjectTypeRunnerToken:
			current = RunnerTokenMetadata{}
		case ObjectTypeGithubOrg:
			current = &GithubOrg{}
		case ObjectTypeGithubConnection:
			current = &GithubConnection{}
		case ObjectTypeFeatureFlag:
			current = &FeatureFlag{}
		case ObjectTypeFeatureFlagOverride:
			current = &FeatureFlagOverride{}
		case ObjectTypeWorkstream:
			current = &Workstream{}
		case ObjectTypeWorkstreamShortName:
			current = &WorkstreamShortName{}
		case ObjectTypeWorkstreamTaskConflict:
			current = &WorkstreamTaskConflict{}
		case ObjectTypeTenantGithubCreds:
			current = &TenantGithubCreds{}
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
// nolint: dupl
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

	var tenant Tenant
	if err := json.NewDecoder(resp.Body).Decode(&tenant); err != nil {
		return nil, err
	}
	return &tenant, nil
}

// ListTenants lists tenants in the service.
func (c *Client) ListTenants(ctx context.Context, req *ListTenantsRequest) (*List[*Tenant], error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}

	u := c.BaseURL.JoinPath("v1", "tenants")
	q := u.Query()
	if req.MaxResults != nil {
		q.Set("maxResults", strconv.Itoa(*req.MaxResults))
	}
	if req.Token != nil {
		q.Set("token", *req.Token)
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

	var out List[*Tenant]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTenantFeatureFlagsRequest is the request for GetTenantFeatureFlags.
type GetTenantFeatureFlagsRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID string `json:"-"`
}

// nolint: goconst
func (r *GetTenantFeatureFlagsRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	default:
		return nil, false
	}
}

// GetTenantFeatureFlagsResponse is the response from GetTenantFeatureFlags.
type GetTenantFeatureFlagsResponse struct {
	FeatureFlags map[string]bool `json:"FeatureFlags"`
}

// GetTenantFeatureFlags returns the values of all active feature flags for a tenant.
// nolint: dupl
func (c *Client) GetTenantFeatureFlags(ctx context.Context, req *GetTenantFeatureFlagsRequest) (*GetTenantFeatureFlagsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "featureflags")
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

	var out GetTenantFeatureFlagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GenerateWebUITokenRequest is the request for GenerateWebUIToken.
type GenerateWebUITokenRequest struct {
	FeatureFlags
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
// nolint:dupl
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
	FeatureFlags
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
	FeatureFlags
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

	var out UploadTurnLogsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// StreamTurnLogsRequest is the request payload for StreamTurnLogs.
type StreamTurnLogsRequest struct {
	FeatureFlags
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
	processFeatureFlags(httpReq, req.FeatureFlags)

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

	var out LastTurnLog
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

type List[T any] struct {
	Items     []T     `json:"Items"`
	NextToken *string `json:"NextToken,omitempty"`
}
