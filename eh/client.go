package eh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

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
		c.authHandler = &sigv4AuthHandler{cfg: &cfg, clk: clk}
	}
}

// Client is the API client for Event Horizon.
type Client struct {
	// BaseURL is the HTTP base url for the API, e.g. https://api.example.com
	BaseURL *url.URL

	// HTTPClient is the http client used to make requests. If nil, http.DefaultClient is used.
	HTTPClient *http.Client

	// authHandler is used to add authentication to requests.
	authHandler AuthHandler
}

// NewClient returns a new API client with the given baseURL.
func NewClient(baseURL string, opts ...Option) *Client {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		panic(fmt.Sprintf("invalid base URL: %v", err))
	}
	c := &Client{BaseURL: parsedURL, HTTPClient: http.DefaultClient}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *Client) authenticate(req *http.Request) error {
	if c.authHandler == nil {
		return nil
	}
	return c.authHandler.Authenticate(req)
}

// TenantType is the type of tenant.
type TenantType string

const (
	TenantTypeUser         TenantType = "User"
	TenantTypeOrganization TenantType = "Organization"
	TenantTypeEnterprise   TenantType = "Enterprise"
)

// CreateTenantRequest is the request payload for CreateTenant.
type CreateTenantRequest struct {
	TenantID       string     `json:"-"`
	Type           TenantType `json:"Type"`
	FullName       *string    `json:"FullName,omitempty"`
	OrgName        *string    `json:"OrgName,omitempty"`
	EnterpriseName *string    `json:"EnterpriseName,omitempty"`
	Email          *string    `json:"Email,omitempty"`
	FirstName      *string    `json:"FirstName,omitempty"`
	LastName       *string    `json:"LastName,omitempty"`
	InitialOwner   *string    `json:"InitialOwner,omitempty"`
}

// GetTenantRequest is the request for GetTenant.
type GetTenantRequest struct {
	TenantID string
}

// Tenant is the representation of a tenant returned by the API.
type Tenant struct {
	TenantID       string     `json:"TenantId"`
	Type           TenantType `json:"Type"`
	Version        int        `json:"Version"`
	Deleted        bool       `json:"Deleted"`
	CreatedAt      string     `json:"CreatedAt"`
	UpdatedAt      string     `json:"UpdatedAt"`
	FullName       *string    `json:"FullName,omitempty"`
	OrgName        *string    `json:"OrgName,omitempty"`
	EnterpriseName *string    `json:"EnterpriseName,omitempty"`
	Email          *string    `json:"Email,omitempty"`
	FirstName      *string    `json:"FirstName,omitempty"`
	LastName       *string    `json:"LastName,omitempty"`
}

func (t Tenant) ObjectType() ObjectType {
	return ObjectTypeTenant
}

// Error represents an error returned by the API.
type Error struct {
	ResponseCode int    `json:"ResponseCode"`
	Message      string `json:"Message"`
	ErrorType    string `json:"ErrorType"`
}

func (e *Error) Error() string {
	return e.Message
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
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")

	if err := c.authenticate(httpReq); err != nil {
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

	if err := c.authenticate(httpReq); err != nil {
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
