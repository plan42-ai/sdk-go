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
	ObjectTypeTenant ObjectType = "Tenant"
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
