package eh

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
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

func (r *CreateTenantRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID": //nolint
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

func (r *GetCurrentUserRequest) GetField(_ string) (any, bool) {
	return nil, false
}

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

// ListPoliciesRequest is the request for ListPolicies.
type ListPoliciesRequest struct {
	DelegatedAuthInfo
	TenantID   string
	MaxResults *int
	Token      *string
}

func (r *ListPoliciesRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "MaxResults":
		return evalNullable(r.MaxResults)
	case "Token":
		return evalNullable(r.Token)
	default:
		return nil, false
	}
}

// ListPoliciesResponse is the response from ListPolicies.
type ListPoliciesResponse struct {
	Policies  []Policy `json:"Policies"`
	NextToken *string  `json:"NextToken"`
}

// ListPolicies retrieves the policies for a tenant.
func (c *Client) ListPolicies(ctx context.Context, req *ListPoliciesRequest) (*ListPoliciesResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "policies")
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

	var out ListPoliciesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GenerateWebUITokenRequest is the request for GenerateWebUIToken.
type GenerateWebUITokenRequest struct {
	DelegatedAuthInfo
	TenantID string
	TokenID  string
}

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

// CreateEnvironmentRequest is the request payload for CreateEnvironment.
type CreateEnvironmentRequest struct {
	DelegatedAuthInfo
	TenantID      string   `json:"-"`
	EnvironmentID string   `json:"-"`
	Name          string   `json:"Name"`
	Description   string   `json:"Description"`
	Context       string   `json:"Context"`
	Repos         []string `json:"Repos"`
	SetupScript   string   `json:"SetupScript"`
	DockerImage   string   `json:"DockerImage"`
	AllowedHosts  []string `json:"AllowedHosts"`
	EnvVars       []EnvVar `json:"EnvVars"`
}

// GetEnvironmentRequest is the request payload for GetEnvironment.
type GetEnvironmentRequest struct {
	DelegatedAuthInfo
	TenantID       string `json:"-"`
	EnvironmentID  string `json:"-"`
	IncludeDeleted *bool  `json:"-"`
}

// UpdateEnvironmentRequest is the request payload for UpdateEnvironment.
type UpdateEnvironmentRequest struct {
	DelegatedAuthInfo
	TenantID      string    `json:"-"`
	EnvironmentID string    `json:"-"`
	Version       int       `json:"-"`
	Name          *string   `json:"Name,omitempty"`
	Description   *string   `json:"Description,omitempty"`
	Context       *string   `json:"Context,omitempty"`
	Repos         *[]string `json:"Repos,omitempty"`
	SetupScript   *string   `json:"SetupScript,omitempty"`
	DockerImage   *string   `json:"DockerImage,omitempty"`
	AllowedHosts  *[]string `json:"AllowedHosts,omitempty"`
	EnvVars       *[]EnvVar `json:"EnvVars,omitempty"`
	Deleted       *bool     `json:"Deleted,omitempty"`
}

// DeleteEnvironmentRequest is the request payload for DeleteEnvironment.
type DeleteEnvironmentRequest struct {
	DelegatedAuthInfo
	TenantID      string `json:"-"`
	EnvironmentID string `json:"-"`
	Version       int    `json:"-"`
}

// GetField retrieves the value of a field by name.
func (r *GetEnvironmentRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "EnvironmentID": //nolint: goconst
		return r.EnvironmentID, true
	case "IncludeDeleted":
		return evalNullable(r.IncludeDeleted)
	default:
		return nil, false
	}
}

// GetField retrieves the value of a field by name.
func (r *CreateEnvironmentRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "EnvironmentID":
		return r.EnvironmentID, true
	case "Name": //nolint: goconst
		return r.Name, true
	case "Description":
		return r.Description, true
	case "Context":
		return r.Context, true
	case "Repos":
		return r.Repos, true
	case "SetupScript":
		return r.SetupScript, true
	case "DockerImage":
		return r.DockerImage, true
	case "AllowedHosts":
		return r.AllowedHosts, true
	case "EnvVars":
		return r.EnvVars, true
	default:
		return nil, false
	}
}

// GetField retrieves the value of a field by name.
func (r *UpdateEnvironmentRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "EnvironmentID":
		return r.EnvironmentID, true
	case "Version":
		return r.Version, true
	case "Name":
		return evalNullable(r.Name)
	case "Description":
		return evalNullable(r.Description)
	case "Context":
		return evalNullable(r.Context)
	case "Repos":
		return evalNullable(r.Repos)
	case "SetupScript":
		return evalNullable(r.SetupScript)
	case "DockerImage":
		return evalNullable(r.DockerImage)
	case "AllowedHosts":
		return evalNullable(r.AllowedHosts)
	case "EnvVars":
		return evalNullable(r.EnvVars)
	case "Deleted":
		return evalNullable(r.Deleted)
	default:
		return nil, false
	}
}

// GetField retrieves the value of a field by name.
func (r *DeleteEnvironmentRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "EnvironmentID":
		return r.EnvironmentID, true
	case "Version":
		return r.Version, true
	default:
		return nil, false
	}
}

// CreateEnvironment creates a new environment for a tenant.
func (c *Client) CreateEnvironment(ctx context.Context, req *CreateEnvironmentRequest) (*Environment, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "environments", url.PathEscape(req.EnvironmentID))
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

	var out Environment
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetEnvironment retrieves an environment by ID.
func (c *Client) GetEnvironment(ctx context.Context, req *GetEnvironmentRequest) (*Environment, error) {
	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "environments", url.PathEscape(req.EnvironmentID))
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

	var out Environment
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateEnvironment updates an existing environment.
func (c *Client) UpdateEnvironment(ctx context.Context, req *UpdateEnvironmentRequest) (*Environment, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.EnvironmentID == "" {
		return nil, fmt.Errorf("environment id is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "environments", url.PathEscape(req.EnvironmentID))
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

	var out Environment
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListEnvironmentsRequest is the request for ListEnvironments.
type ListEnvironmentsRequest struct {
	DelegatedAuthInfo
	TenantID       string
	MaxResults     *int
	Token          *string
	IncludeDeleted *bool
}

// GetField retrieves the value of a field by name.
func (r *ListEnvironmentsRequest) GetField(name string) (any, bool) {
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

// ListEnvironmentsResponse is the response from ListEnvironments.
type ListEnvironmentsResponse struct {
	Environments []Environment `json:"Environments"`
	NextToken    *string       `json:"NextToken"`
}

// ListEnvironments retrieves the environments for a tenant.
func (c *Client) ListEnvironments(ctx context.Context, req *ListEnvironmentsRequest) (*ListEnvironmentsResponse, error) {
	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "environments")
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

	var out ListEnvironmentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteEnvironment deletes an environment.
func (c *Client) DeleteEnvironment(ctx context.Context, req *DeleteEnvironmentRequest) error {
	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "environments", url.PathEscape(req.EnvironmentID))
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
