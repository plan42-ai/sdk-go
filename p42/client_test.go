package p42_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/plan42-ai/clock"
	"github.com/plan42-ai/ecies"
	"github.com/plan42-ai/sdk-go/internal/util"
	"github.com/plan42-ai/sdk-go/p42"
	sigv4auth "github.com/plan42-ai/sigv4util/server/sigv4auth"
	"github.com/stretchr/testify/require"
)

const (
	expectedSigCreateTenant   = "UE9TVCAvIEhUVFAvMS4xDQpIb3N0OiBzdHMudXMtd2VzdC0yLmFtYXpvbmF3cy5jb20NClVzZXItQWdlbnQ6IEdvLWh0dHAtY2xpZW50LzEuMQ0KVHJhbnNmZXItRW5jb2Rpbmc6IGNodW5rZWQNCkFjY2VwdDogYXBwbGljYXRpb24vanNvbg0KQWNjZXB0LUVuY29kaW5nOiBpZGVudGl0eQ0KQXV0aG9yaXphdGlvbjogQVdTNC1ITUFDLVNIQTI1NiBDcmVkZW50aWFsPUFLSUQvMjAyNTAxMDEvdXMtd2VzdC0yL3N0cy9hd3M0X3JlcXVlc3QsIFNpZ25lZEhlYWRlcnM9YWNjZXB0O2FjY2VwdC1lbmNvZGluZztjb250ZW50LXR5cGU7aG9zdDt4LWFtei1kYXRlO3gtYW16LXNlY3VyaXR5LXRva2VuO3gtZXZlbnQtaG9yaXpvbi1yZXF1ZXN0LWhhc2gsIFNpZ25hdHVyZT1lZmI1MGZlODE4NzRjOWFkMjI2ODJiNjAzYjI2OTkxNDQ4ZTRjNTIzODJmMTM4ZWM2NzQ1YmIzOTE0YzNiM2E1DQpDb250ZW50LVR5cGU6IGFwcGxpY2F0aW9uL3gtd3d3LWZvcm0tdXJsZW5jb2RlZA0KWC1BbXotRGF0ZTogMjAyNTAxMDFUMDAwMDAwWg0KWC1BbXotU2VjdXJpdHktVG9rZW46IFRPS0VODQpYLUV2ZW50LUhvcml6b24tUmVxdWVzdC1IYXNoOiAyOTdmNzRkYmRkY2RmZGU3NjY4OTYyZGYwM2YxMGMwZjVmYzYwNzA3YmExNGM2ZjJhNDgwMzE0ZDAxMzg5ZjZmDQoNCjJkDQpBY3Rpb249R2V0Q2FsbGVySWRlbnRpdHkmVmVyc2lvbj0yMDExLTA2LTE1DQoNCjANCg0K"
	expectedSigGetTenant      = "UE9TVCAvIEhUVFAvMS4xDQpIb3N0OiBzdHMudXMtd2VzdC0yLmFtYXpvbmF3cy5jb20NClVzZXItQWdlbnQ6IEdvLWh0dHAtY2xpZW50LzEuMQ0KVHJhbnNmZXItRW5jb2Rpbmc6IGNodW5rZWQNCkFjY2VwdDogYXBwbGljYXRpb24vanNvbg0KQWNjZXB0LUVuY29kaW5nOiBpZGVudGl0eQ0KQXV0aG9yaXphdGlvbjogQVdTNC1ITUFDLVNIQTI1NiBDcmVkZW50aWFsPUFLSUQvMjAyNTAxMDEvdXMtd2VzdC0yL3N0cy9hd3M0X3JlcXVlc3QsIFNpZ25lZEhlYWRlcnM9YWNjZXB0O2FjY2VwdC1lbmNvZGluZztjb250ZW50LXR5cGU7aG9zdDt4LWFtei1kYXRlO3gtYW16LXNlY3VyaXR5LXRva2VuO3gtZXZlbnQtaG9yaXpvbi1yZXF1ZXN0LWhhc2gsIFNpZ25hdHVyZT02OGU5ZWZhZTJkNjc0ZTc3NTViZWRiMDViNTYyOWNkZGJiY2QwMGZjYWQzMzE2Mjg5OWQ3ZGM2NzZhZWMxMDg3DQpDb250ZW50LVR5cGU6IGFwcGxpY2F0aW9uL3gtd3d3LWZvcm0tdXJsZW5jb2RlZA0KWC1BbXotRGF0ZTogMjAyNTAxMDFUMDAwMDAwWg0KWC1BbXotU2VjdXJpdHktVG9rZW46IFRPS0VODQpYLUV2ZW50LUhvcml6b24tUmVxdWVzdC1IYXNoOiBiYzhmOWZlNDM3ZjEwZmMwZWQ0YmExOWRkZjYyNmEzN2Y4NmI0Y2Y3Mzg4MTZkOGI1YTQ3ZmRmNmNjNWFiMTFlDQoNCjJkDQpBY3Rpb249R2V0Q2FsbGVySWRlbnRpdHkmVmVyc2lvbj0yMDExLTA2LTE1DQoNCjANCg0K"
	expectedSigGetCurrentUser = "UE9TVCAvIEhUVFAvMS4xDQpIb3N0OiBzdHMudXMtd2VzdC0yLmFtYXpvbmF3cy5jb20NClVzZXItQWdlbnQ6IEdvLWh0dHAtY2xpZW50LzEuMQ0KVHJhbnNmZXItRW5jb2Rpbmc6IGNodW5rZWQNCkFjY2VwdDogYXBwbGljYXRpb24vanNvbg0KQWNjZXB0LUVuY29kaW5nOiBpZGVudGl0eQ0KQXV0aG9yaXphdGlvbjogQVdTNC1ITUFDLVNIQTI1NiBDcmVkZW50aWFsPUFLSUQvMjAyNTAxMDEvdXMtd2VzdC0yL3N0cy9hd3M0X3JlcXVlc3QsIFNpZ25lZEhlYWRlcnM9YWNjZXB0O2FjY2VwdC1lbmNvZGluZztjb250ZW50LXR5cGU7aG9zdDt4LWFtei1kYXRlO3gtYW16LXNlY3VyaXR5LXRva2VuO3gtZXZlbnQtaG9yaXpvbi1yZXF1ZXN0LWhhc2gsIFNpZ25hdHVyZT0yMmEwYmVhMjg1ZTU4NTE3NjExMGJjYjY1NzEwNjM3YzdmYTYwNzVjOTA2MGFhNWIxYTZjZjc2Y2UxMGNmZjM5DQpDb250ZW50LVR5cGU6IGFwcGxpY2F0aW9uL3gtd3d3LWZvcm0tdXJsZW5jb2RlZA0KWC1BbXotRGF0ZTogMjAyNTAxMDFUMDAwMDAwWg0KWC1BbXotU2VjdXJpdHktVG9rZW46IFRPS0VODQpYLUV2ZW50LUhvcml6b24tUmVxdWVzdC1IYXNoOiBjMGRkNGY0ODFhM2I2OTVlMTM2MThkZWZmZjEyMDA0OWNlNWZhM2YxM2UwYTEzZmQ4ZTcwYmI4YWU1MjhmMWJhDQoNCjJkDQpBY3Rpb249R2V0Q2FsbGVySWRlbnRpdHkmVmVyc2lvbj0yMDExLTA2LTE1DQoNCjANCg0K"

	tenantIDThatNeedsEscaping = "foo/../../bar"
	escapedTenantID           = "foo%2F..%2F..%2Fbar"

	tokenIDThatNeedsEscaping            = "tok/../../id"       // #nosec G101: This is not a credential.
	escapedTokenID                      = "tok%2F..%2F..%2Fid" // #nosec G101: This is not a credential.
	environmentIDThatNeedsEscaping      = "env/../../id"
	escapedEnvironmentID                = "env%2F..%2F..%2Fid"
	taskIDThatNeedsEscaping             = "task/../../id"
	escapedTaskID                       = "task%2F..%2F..%2Fid"
	workstreamIDThatNeedsEscaping       = "ws/../../id"
	escapedWorkstreamID                 = "ws%2F..%2F..%2Fid"
	githubOrgIDThatNeedsEscaping        = "org/../../id"
	escapedGithubOrgID                  = "org%2F..%2F..%2Fid"
	featureFlagNameThatNeedsEscaping    = "flag/../../name"
	escapedFeatureFlagName              = "flag%2F..%2F..%2Fname"
	runnerIDThatNeedsEscaping           = "runner/../../id"
	escapedRunnerID                     = "runner%2F..%2F..%2Fid"
	githubConnectionIDThatNeedsEscaping = "conn/../../id"
	escapedGithubConnectionID           = "conn%2F..%2F..%2Fid"
	queueIDThatNeedsEscaping            = "queue/../../id"
	escapedQueueID                      = "queue%2F..%2F..%2Fid"
	messageIDThatNeedsEscaping          = "message/../../id"
	escapedMessageID                    = "message%2F..%2F..%2Fid"

	tokenID         = "tok"
	taskTitle       = "new"
	turnStatus      = "Done"
	githubUserLogin = "octocat"
)

func TestCreateTenant(t *testing.T) {
	t.Parallel()
	// Mock server returns successful response
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "/v1/tenants/abc", r.URL.Path)

			var reqBody p42.CreateTenantRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.Equal(t, p42.TenantTypeUser, reqBody.Type)

			w.WriteHeader(http.StatusCreated)
			resp := p42.Tenant{TenantID: "abc", Type: p42.TenantTypeUser, Version: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	tenant, err := client.CreateTenant(
		context.Background(),
		&p42.CreateTenantRequest{TenantID: "abc", Type: p42.TenantTypeUser},
	)
	require.NoError(t, err)
	require.Equal(t, "abc", tenant.TenantID)
}

func TestCreateTenantError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.CreateTenant(
		context.Background(),
		&p42.CreateTenantRequest{TenantID: "abc", Type: p42.TenantTypeUser},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestCreateTenantConflictError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(
				p42.ConflictError{
					ResponseCode: http.StatusConflict,
					Message:      "exists",
					ErrorType:    "Conflict",
					Current:      &p42.Tenant{TenantID: "abc"},
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.CreateTenant(
		context.Background(),
		&p42.CreateTenantRequest{TenantID: "abc", Type: p42.TenantTypeUser},
	)
	var clientErr *p42.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.Equal(t, "exists", clientErr.Message)
	require.Equal(t, "Conflict", clientErr.ErrorType)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, p42.ObjectTypeTenant, clientErr.Current.ObjectType())
	tenant, ok := clientErr.Current.(*p42.Tenant)
	require.True(t, ok, "Expected Current to be of type *p42.Tenant")
	require.Equal(t, p42.Tenant{TenantID: "abc"}, *tenant)
}

func TestGetTenant(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc", r.URL.Path)

			w.WriteHeader(http.StatusOK)
			resp := p42.Tenant{TenantID: "abc", Type: p42.TenantTypeUser, Version: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	tenant, err := client.GetTenant(context.Background(), &p42.GetTenantRequest{TenantID: "abc"})
	require.NoError(t, err)
	require.Equal(t, "abc", tenant.TenantID)
}

func TestGetTenantError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(
				p42.Error{
					ResponseCode: http.StatusNotFound,
					Message:      "nope",
					ErrorType:    "NotFound",
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetTenant(context.Background(), &p42.GetTenantRequest{TenantID: "abc"})
	require.Error(t, err)
}

func TestGetCurrentUser(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/current-user", r.URL.Path)

			w.WriteHeader(http.StatusOK)
			resp := p42.Tenant{TenantID: "abc", Type: p42.TenantTypeUser, Version: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	tenant, err := client.GetCurrentUser(context.Background(), &p42.GetCurrentUserRequest{})
	require.NoError(t, err)
	require.Equal(t, "abc", tenant.TenantID)
}

func TestGetCurrentUserError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(
				p42.Error{
					ResponseCode: http.StatusForbidden,
					Message:      "nope",
					ErrorType:    "Forbidden",
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetCurrentUser(context.Background(), &p42.GetCurrentUserRequest{})
	require.Error(t, err)
}

func TestListTenants(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants", r.URL.Path)
			require.Equal(t, "123", r.URL.Query().Get("maxResults"))
			require.Equal(t, tokenID, r.URL.Query().Get("token"))

			w.WriteHeader(http.StatusOK)
			resp := p42.List[*p42.Tenant]{Items: []*p42.Tenant{{TenantID: "abc"}}}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	maxResults := 123
	resp, err := client.ListTenants(
		context.Background(),
		&p42.ListTenantsRequest{MaxResults: &maxResults, Token: util.Pointer(tokenID)},
	)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	require.Equal(t, "abc", resp.Items[0].TenantID)
}

func TestListTenantsError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListTenants(context.Background(), &p42.ListTenantsRequest{})
	require.Error(t, err)
}

func TestCreateTenantPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// Get the path and verify proper escaping
			escapedPath := r.URL.EscapedPath()

			// Split the path and check the escaped tenant ID part
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, len(parts), 4, "path doesn't have correct # of parts: %s", escapedPath)

			// The last part should be the escaped tenant ID
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")

			w.WriteHeader(http.StatusCreated)
			resp := p42.Tenant{TenantID: tenantIDThatNeedsEscaping, Type: p42.TenantTypeUser, Version: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.CreateTenant(
		context.Background(), &p42.CreateTenantRequest{
			TenantID: tenantIDThatNeedsEscaping,
			Type:     p42.TenantTypeUser,
		},
	)

	require.NoError(t, err)
}

func TestGetTenantPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// Get the path and verify proper escaping
			escapedPath := r.URL.EscapedPath()

			// Split the path and check the escaped tenant ID part
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, len(parts), 4, "path doesn't have correct # of parts: %s", escapedPath)

			// The last part should be the escaped tenant ID
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")

			w.WriteHeader(http.StatusOK)
			resp := p42.Tenant{TenantID: tenantIDThatNeedsEscaping, Type: p42.TenantTypeUser, Version: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetTenant(
		context.Background(), &p42.GetTenantRequest{
			TenantID: tenantIDThatNeedsEscaping,
		},
	)

	require.NoError(t, err)
}

func TestUpdateTenant(t *testing.T) {
	t.Parallel()

	defaultRunnerID := "runner-123"
	clearConnection := ""

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, "/v1/tenants/tenant-1", r.URL.Path)
			require.Equal(t, "3", r.Header.Get("If-Match"))

			var reqBody p42.UpdateTenantRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&reqBody))
			require.NotNil(t, reqBody.DefaultRunnerID)
			require.Equal(t, defaultRunnerID, *reqBody.DefaultRunnerID)
			require.NotNil(t, reqBody.DefaultGithubConnectionID)
			require.Equal(t, clearConnection, *reqBody.DefaultGithubConnectionID)
			require.Zero(t, reqBody.Version)
			require.Empty(t, reqBody.TenantID)

			w.WriteHeader(http.StatusOK)
			resp := p42.Tenant{
				TenantID:                  "tenant-1",
				Type:                      p42.TenantTypeOrganization,
				Version:                   3,
				DefaultRunnerID:           &defaultRunnerID,
				DefaultGithubConnectionID: nil,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	updated, err := client.UpdateTenant(
		context.Background(), &p42.UpdateTenantRequest{
			TenantID:                  "tenant-1",
			Version:                   3,
			DefaultRunnerID:           &defaultRunnerID,
			DefaultGithubConnectionID: &clearConnection,
		},
	)
	require.NoError(t, err)
	require.Equal(t, "tenant-1", updated.TenantID)
	require.NotNil(t, updated.DefaultRunnerID)
	require.Equal(t, defaultRunnerID, *updated.DefaultRunnerID)
	require.Nil(t, updated.DefaultGithubConnectionID)
}

func TestUpdateTenantError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.UpdateTenant(context.Background(), &p42.UpdateTenantRequest{TenantID: "abc", Version: 1})
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestUpdateTenantPathEscaping(t *testing.T) {
	t.Parallel()

	defaultRunnerID := "runner"
	clearConnection := ""

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, len(parts), 4, "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, "1", r.Header.Get("If-Match"))

			w.WriteHeader(http.StatusOK)
			resp := p42.Tenant{
				TenantID:                  tenantIDThatNeedsEscaping,
				DefaultRunnerID:           &defaultRunnerID,
				DefaultGithubConnectionID: nil,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.UpdateTenant(
		context.Background(), &p42.UpdateTenantRequest{
			TenantID:                  tenantIDThatNeedsEscaping,
			Version:                   1,
			DefaultRunnerID:           &defaultRunnerID,
			DefaultGithubConnectionID: &clearConnection,
		},
	)
	require.NoError(t, err)
}

func verifySigv4(t *testing.T, r *http.Request, cfg *aws.Config, expected string, clk clock.Clock) {
	t.Helper()
	auth := r.Header.Get("Authorization")
	require.NotEmpty(t, auth)

	parts := strings.SplitN(auth, " ", 2)
	require.Len(t, parts, 2)
	require.Equal(t, "sts:GetCallerIdentity", parts[0])
	require.Equal(t, expected, parts[1])

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	require.NoError(t, err)

	stsReq, err := sigv4auth.ParseRequest(string(decoded))
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	require.NoError(t, sigv4auth.VerifyStsReq(r, stsReq, cfg.Region, logger, clk))
}

func TestSigv4Auth(t *testing.T) {
	cfg := aws.Config{
		Credentials: aws.NewCredentialsCache(
			credentials.StaticCredentialsProvider{
				Value: aws.Credentials{
					AccessKeyID:     "AKID",
					SecretAccessKey: "SECRET",
					SessionToken:    "TOKEN",
				},
			},
		),
		Region: "us-west-2",
	}

	clk := clock.NewFakeClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	mux := http.NewServeMux()
	mux.HandleFunc(
		"/v1/tenants/abc", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPut:
				verifySigv4(t, r, &cfg, expectedSigCreateTenant, clk)
				w.WriteHeader(http.StatusCreated)
			case http.MethodGet:
				verifySigv4(t, r, &cfg, expectedSigGetTenant, clk)
				w.WriteHeader(http.StatusOK)
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			resp := p42.Tenant{TenantID: "abc", Type: p42.TenantTypeUser, Version: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)
	mux.HandleFunc(
		"/v1/current-user", func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			verifySigv4(t, r, &cfg, expectedSigGetCurrentUser, clk)
			w.WriteHeader(http.StatusOK)
			resp := p42.Tenant{TenantID: "abc", Type: p42.TenantTypeUser, Version: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewUnstartedServer(mux)
	defer srv.Close()
	_ = srv.Listener.Close()
	var err error
	srv.Listener, err = net.Listen("tcp", "localhost:4242")
	require.NoError(t, err)
	srv.Start()

	client := p42.NewClient(srv.URL, p42.WithSigv4Auth(cfg, clk))
	_, err = client.CreateTenant(
		context.Background(),
		&p42.CreateTenantRequest{TenantID: "abc", Type: p42.TenantTypeUser},
	)
	require.NoError(t, err)

	_, err = client.GetTenant(context.Background(), &p42.GetTenantRequest{TenantID: "abc"})
	require.NoError(t, err)

	_, err = client.GetCurrentUser(context.Background(), &p42.GetCurrentUserRequest{})
	require.NoError(t, err)
}

func TestListPolicies(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/policies", r.URL.Path)
			require.Equal(t, "123", r.URL.Query().Get("maxResults"))
			require.Equal(t, tokenID, r.URL.Query().Get("token"))

			w.WriteHeader(http.StatusOK)
			resp := p42.ListPoliciesResponse{Policies: []p42.Policy{{Name: "p"}}}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	maxResults := 123
	resp, err := client.ListPolicies(
		context.Background(),
		&p42.ListPoliciesRequest{TenantID: "abc", MaxResults: &maxResults, Token: util.Pointer(tokenID)},
	)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	require.Equal(t, "p", resp.Policies[0].Name)
}

func TestListPoliciesError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListPolicies(context.Background(), &p42.ListPoliciesRequest{TenantID: "abc"})
	require.Error(t, err)
}

func TestListPoliciesPathEscaping(t *testing.T) {
	t.Parallel()
	tenantID := tenantIDThatNeedsEscaping
	expectedEscaped := "foo%2F..%2F..%2Fbar"

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, expectedEscaped, parts[3], "TenantID not properly escaped in URL path")

			w.WriteHeader(http.StatusOK)
			resp := p42.ListPoliciesResponse{}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.ListPolicies(context.Background(), &p42.ListPoliciesRequest{TenantID: tenantID})
	require.NoError(t, err)
}

func TestGenerateWebUIToken(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "/v1/tenants/abc/ui-tokens/tok", r.URL.Path)

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(p42.GenerateWebUITokenResponse{JWT: "jwt"})
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	resp, err := client.GenerateWebUIToken(
		context.Background(),
		&p42.GenerateWebUITokenRequest{TenantID: "abc", TokenID: tokenID},
	)
	require.NoError(t, err)
	require.Equal(t, "jwt", resp.JWT)
}

func TestGenerateWebUITokenError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()
	_, err := client.GenerateWebUIToken(
		context.Background(),
		&p42.GenerateWebUITokenRequest{TenantID: "abc", TokenID: tokenID},
	)
	require.Error(t, err)
}

func TestGenerateWebUITokenPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedTokenID, parts[5], "TokenID not properly escaped in URL path")

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(p42.GenerateWebUITokenResponse{JWT: "jwt"})
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GenerateWebUIToken(
		context.Background(),
		&p42.GenerateWebUITokenRequest{TenantID: tenantIDThatNeedsEscaping, TokenID: tokenIDThatNeedsEscaping},
	)
	require.NoError(t, err)
}

func TestGenerateRunnerToken(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "/v1/tenants/abc/runners/runner/tokens/token-id", r.URL.Path)

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(
				p42.GenerateRunnerTokenResponse{
					RunnerTokenMetadata: p42.RunnerTokenMetadata{
						TokenID:   "token-id",
						ExpiresAt: expiresAt,
					},
					Token: "token",
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	resp, err := client.GenerateRunnerToken(
		context.Background(),
		&p42.GenerateRunnerTokenRequest{
			TenantID: "abc",
			RunnerID: "runner",
			TokenID:  "token-id",
		},
	)
	require.NoError(t, err)
	require.Equal(t, "token-id", resp.TokenID)
	require.Equal(t, "token", resp.Token)
	require.True(t, resp.ExpiresAt.Equal(expiresAt))
}

func TestGenerateRunnerTokenWithTTL(t *testing.T) {
	t.Parallel()

	var receivedReq p42.GenerateRunnerTokenRequest

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&receivedReq)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(p42.GenerateRunnerTokenResponse{})
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	ttl := 120
	_, err := client.GenerateRunnerToken(
		context.Background(),
		&p42.GenerateRunnerTokenRequest{
			TenantID: "abc",
			RunnerID: "runner",
			TokenID:  "token-id",
			TTLDays:  &ttl,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, receivedReq.TTLDays)
	require.Equal(t, 120, *receivedReq.TTLDays)
}

func TestGenerateRunnerTokenError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()
	_, err := client.GenerateRunnerToken(
		context.Background(),
		&p42.GenerateRunnerTokenRequest{
			TenantID: "abc",
			RunnerID: "runner",
			TokenID:  "token-id",
		},
	)
	require.Error(t, err)
}

func TestGenerateRunnerTokenPathEscaping(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedRunnerID, parts[5], "RunnerID not properly escaped in URL path")
			require.Equal(t, escapedTokenID, parts[7], "RunnerID not properly escaped in URL path")

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(
				p42.GenerateRunnerTokenResponse{
					RunnerTokenMetadata: p42.RunnerTokenMetadata{
						TokenID:   "token-id",
						ExpiresAt: expiresAt,
					},
					Token: "token",
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GenerateRunnerToken(
		context.Background(),
		&p42.GenerateRunnerTokenRequest{
			TenantID: tenantIDThatNeedsEscaping,
			RunnerID: runnerIDThatNeedsEscaping,
			TokenID:  tokenIDThatNeedsEscaping,
		},
	)
	require.NoError(t, err)
}

func TestCreateEnvironment(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "/v1/tenants/abc/environments/env", r.URL.Path)

			var reqBody p42.CreateEnvironmentRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.Equal(t, "env", reqBody.Name)

			w.WriteHeader(http.StatusCreated)
			resp := p42.Environment{TenantID: "abc", EnvironmentID: "env"}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	env, err := client.CreateEnvironment(
		context.Background(),
		&p42.CreateEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", Name: "env"},
	)
	require.NoError(t, err)
	require.Equal(t, "env", env.EnvironmentID)
}

func TestCreateEnvironmentError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.CreateEnvironment(
		context.Background(),
		&p42.CreateEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", Name: "env"},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func serveEnvironmentConflict() (*httptest.Server, *p42.Client) {
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(
				p42.ConflictError{
					ResponseCode: http.StatusConflict,
					Message:      "exists",
					ErrorType:    "Conflict",
					Current:      &p42.Environment{EnvironmentID: "env"},
				},
			)
		},
	)

	srv := httptest.NewServer(handler)

	client := p42.NewClient(srv.URL)
	return srv, client
}

func serveBadRequest() (*httptest.Server, *p42.Client) {
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(
				p42.Error{
					ResponseCode: http.StatusBadRequest,
					Message:      "bad",
					ErrorType:    "BadRequest",
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	client := p42.NewClient(srv.URL)

	return srv, client
}

func TestCreateEnvironmentConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveEnvironmentConflict()
	defer srv.Close()

	_, err := client.CreateEnvironment(
		context.Background(),
		&p42.CreateEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", Name: "env"},
	)
	verifyEnvironmentConflict(t, err)
}

func verifyEnvironmentConflict(t *testing.T, err error) {
	var clientErr *p42.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.Equal(t, "exists", clientErr.Message)
	require.Equal(t, "Conflict", clientErr.ErrorType)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, p42.ObjectTypeEnvironment, clientErr.Current.ObjectType())
	env, ok := clientErr.Current.(*p42.Environment)
	require.True(t, ok, "Expected Current to be of type *p42.Environment")
	const defaultID = "default"
	require.Equal(t, "env", env.EnvironmentID)
	require.NotNil(t, env.RunnerID)
	require.NotNil(t, env.GithubConnectionID)
	require.Equal(t, defaultID, *env.RunnerID)
	require.Equal(t, defaultID, *env.GithubConnectionID)
}

func serveRunnerConflict() (*httptest.Server, *p42.Client) {
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(
				p42.ConflictError{
					ResponseCode: http.StatusConflict,
					Message:      "exists",
					ErrorType:    "Conflict",
					Current:      &p42.Runner{RunnerID: "runner1"},
				},
			)
		},
	)

	srv := httptest.NewServer(handler)

	client := p42.NewClient(srv.URL)
	return srv, client
}

func verifyRunnerConflict(t *testing.T, err error) {
	var clientErr *p42.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.Equal(t, "exists", clientErr.Message)
	require.Equal(t, "Conflict", clientErr.ErrorType)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, p42.ObjectTypeRunner, clientErr.Current.ObjectType())
	runner, ok := clientErr.Current.(*p42.Runner)
	require.True(t, ok, "Expected Current to be of type *p42.Runner")
	require.Equal(t, "runner1", runner.RunnerID)
}

func serveTaskConflict() (*httptest.Server, *p42.Client) {
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(
				p42.ConflictError{
					ResponseCode: http.StatusConflict,
					Message:      "exists",
					ErrorType:    "Conflict",
					Current: &p42.WorkstreamTaskConflict{
						Task:       &p42.Task{TaskID: "task"},
						Workstream: &p42.Workstream{WorkstreamID: "ws"},
					},
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	client := p42.NewClient(srv.URL)
	return srv, client
}

func verifyTaskConflict(t *testing.T, err error) {
	var clientErr *p42.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.Equal(t, "exists", clientErr.Message)
	require.Equal(t, "Conflict", clientErr.ErrorType)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, p42.ObjectTypeWorkstreamTaskConflict, clientErr.Current.ObjectType())
	conflict, ok := clientErr.Current.(*p42.WorkstreamTaskConflict)
	require.True(t, ok, "Expected Current to be of type *p42.WorkstreamTaskConflict")
	require.NotNil(t, conflict.Task)
	require.Equal(t, "task", conflict.Task.TaskID)
	require.NotNil(t, conflict.Workstream)
	require.Equal(t, "ws", conflict.Workstream.WorkstreamID)
}

func serveGithubOrgConflict() (*httptest.Server, *p42.Client) {
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(
				p42.ConflictError{
					ResponseCode: http.StatusConflict,
					Message:      "exists",
					ErrorType:    "Conflict",
					Current:      &p42.GithubOrg{OrgID: "org"},
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	client := p42.NewClient(srv.URL)
	return srv, client
}

func verifyGithubOrgConflict(t *testing.T, err error) {
	var clientErr *p42.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.Equal(t, "exists", clientErr.Message)
	require.Equal(t, "Conflict", clientErr.ErrorType)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, p42.ObjectTypeGithubOrg, clientErr.Current.ObjectType())
	org, ok := clientErr.Current.(*p42.GithubOrg)
	require.True(t, ok, "Expected Current to be of type *p42.GithubOrg")
	require.Equal(t, p42.GithubOrg{OrgID: "org"}, *org)
}

// nolint: dupl
func TestCreateEnvironmentPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedEnvironmentID, parts[5], "EnvironmentID not properly escaped in URL path")

			w.WriteHeader(http.StatusCreated)
			resp := p42.Environment{TenantID: tenantIDThatNeedsEscaping, EnvironmentID: environmentIDThatNeedsEscaping}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.CreateEnvironment(
		context.Background(),
		&p42.CreateEnvironmentRequest{
			TenantID:      tenantIDThatNeedsEscaping,
			EnvironmentID: environmentIDThatNeedsEscaping,
			Name:          "env",
		},
	)
	require.NoError(t, err)
}

func TestCreateRunner(t *testing.T) {
	t.Parallel()

	description := "runner-desc"

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "/v1/tenants/abc/runners/runner1", r.URL.Path)

			var reqBody p42.CreateRunnerRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.Equal(t, "runner-name", reqBody.Name)
			require.True(t, reqBody.IsCloud)
			require.True(t, reqBody.RunsTasks)
			require.True(t, reqBody.ProxiesGithub)
			require.NotNil(t, reqBody.Description)
			require.Equal(t, description, *reqBody.Description)

			now := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
			w.WriteHeader(http.StatusCreated)
			resp := p42.Runner{
				TenantID:      "abc",
				RunnerID:      "runner1",
				Name:          "runner-name",
				Description:   &description,
				IsCloud:       true,
				RunsTasks:     true,
				ProxiesGithub: true,
				CreatedAt:     now,
				UpdatedAt:     now,
				Version:       1,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	runner, err := client.CreateRunner(
		context.Background(), &p42.CreateRunnerRequest{
			TenantID:      "abc",
			RunnerID:      "runner1",
			Name:          "runner-name",
			Description:   &description,
			IsCloud:       true,
			RunsTasks:     true,
			ProxiesGithub: true,
		},
	)
	require.NoError(t, err)
	require.Equal(t, "runner1", runner.RunnerID)
	require.NotNil(t, runner.Description)
	require.Equal(t, description, *runner.Description)
}

func TestCreateRunnerError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.CreateRunner(
		context.Background(),
		&p42.CreateRunnerRequest{TenantID: "abc", RunnerID: "runner1", Name: "runner-name"},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestCreateRunnerConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveRunnerConflict()
	defer srv.Close()

	_, err := client.CreateRunner(
		context.Background(),
		&p42.CreateRunnerRequest{TenantID: "abc", RunnerID: "runner1", Name: "runner-name"},
	)
	verifyRunnerConflict(t, err)
}

func TestCreateRunnerPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedRunnerID, parts[5], "RunnerID not properly escaped in URL path")

			w.WriteHeader(http.StatusCreated)
			now := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
			resp := p42.Runner{
				TenantID:  tenantIDThatNeedsEscaping,
				RunnerID:  runnerIDThatNeedsEscaping,
				Name:      "runner",
				CreatedAt: now,
				UpdatedAt: now,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.CreateRunner(
		context.Background(),
		&p42.CreateRunnerRequest{
			TenantID: tenantIDThatNeedsEscaping,
			RunnerID: runnerIDThatNeedsEscaping,
			Name:     "runner",
		},
	)
	require.NoError(t, err)
}

func TestListRunnerQueues(t *testing.T) {
	t.Parallel()

	tenantID := "abc"
	runnerID := "runner1"
	includeHealthy := true
	includeDrained := true
	maxResults := 25
	token := "next"
	minQueueID := "queue-0001"
	maxQueueID := "queue-0100"
	now := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/runner-queues", r.URL.Path)
			require.Equal(t, "application/json", r.Header.Get("Accept"))

			query := r.URL.Query()
			require.Equal(t, tenantID, query.Get("tenantID"))
			require.Equal(t, runnerID, query.Get("runnerID"))
			require.Equal(t, "true", query.Get("includeHealthy"))
			require.Equal(t, "true", query.Get("includeDrained"))
			require.Equal(t, "25", query.Get("maxResults"))
			require.Equal(t, token, query.Get("token"))
			require.Equal(t, minQueueID, query.Get("minQueueID"))
			require.Equal(t, maxQueueID, query.Get("maxQueueID"))

			w.WriteHeader(http.StatusOK)
			resp := p42.List[*p42.RunnerQueue]{
				NextToken: util.Pointer("more"),
				Items: []*p42.RunnerQueue{
					{
						TenantID:  tenantID,
						RunnerID:  runnerID,
						QueueID:   "queue-1",
						PublicKey: "key",
						CreatedAt: now,
						Version:   1,
						IsHealthy: false,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	resp, err := client.ListRunnerQueues(
		context.Background(), &p42.ListRunnerQueuesRequest{
			TenantID:       &tenantID,
			RunnerID:       &runnerID,
			IncludeHealthy: &includeHealthy,
			IncludeDrained: &includeDrained,
			MaxResults:     &maxResults,
			Token:          &token,
			MinQueueID:     &minQueueID,
			MaxQueueID:     &maxQueueID,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, resp.NextToken)
	require.Equal(t, "more", *resp.NextToken)
	require.Len(t, resp.Items, 1)
	require.Equal(t, "queue-1", resp.Items[0].QueueID)
	require.Equal(t, tenantID, resp.Items[0].TenantID)
}

func TestListRunnerQueuesError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListRunnerQueues(context.Background(), &p42.ListRunnerQueuesRequest{})
	require.Error(t, err)
}

func TestListRunnerQueuesValidation(t *testing.T) {
	t.Parallel()

	client := p42.NewClient("http://example.com")

	_, err := client.ListRunnerQueues(context.Background(), nil)
	require.EqualError(t, err, "req is nil")

	tenantID := "tenant"
	_, err = client.ListRunnerQueues(context.Background(), &p42.ListRunnerQueuesRequest{TenantID: &tenantID})
	require.EqualError(t, err, "tenant id and runner id must be provided together")

	runnerID := "runner"
	_, err = client.ListRunnerQueues(context.Background(), &p42.ListRunnerQueuesRequest{RunnerID: &runnerID})
	require.EqualError(t, err, "tenant id and runner id must be provided together")

	minQueueID := "queue-1"
	_, err = client.ListRunnerQueues(context.Background(), &p42.ListRunnerQueuesRequest{MinQueueID: &minQueueID})
	require.EqualError(t, err, "min queue id and max queue id must be provided together")

	maxQueueID := "queue-2"
	_, err = client.ListRunnerQueues(context.Background(), &p42.ListRunnerQueuesRequest{MaxQueueID: &maxQueueID})
	require.EqualError(t, err, "min queue id and max queue id must be provided together")
}

func TestRegisterRunnerQueue(t *testing.T) {
	t.Parallel()

	publicKey := "-----BEGIN PUBLIC KEY-----"
	now := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	lastHealthCheck := now.Add(time.Minute)

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "/v1/tenants/abc/runners/runner1/queues/queue1", r.URL.Path)
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			require.Equal(t, "application/json", r.Header.Get("Accept"))

			var reqBody struct {
				PublicKey string `json:"PublicKey"`
			}
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.Equal(t, publicKey, reqBody.PublicKey)

			w.WriteHeader(http.StatusCreated)
			resp := p42.RunnerQueue{
				TenantID:                           "abc",
				RunnerID:                           "runner1",
				QueueID:                            "queue1",
				PublicKey:                          publicKey,
				CreatedAt:                          now,
				Version:                            1,
				IsHealthy:                          true,
				Draining:                           false,
				NConsecutiveFailedHealthChecks:     0,
				NConsecutiveSuccessfulHealthChecks: 1,
				LastHealthCheckAt:                  lastHealthCheck,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	queue, err := client.RegisterRunnerQueue(
		context.Background(), &p42.RegisterRunnerQueueRequest{
			TenantID:  "abc",
			RunnerID:  "runner1",
			QueueID:   "queue1",
			PublicKey: publicKey,
		},
	)
	require.NoError(t, err)
	require.Equal(t, "queue1", queue.QueueID)
	require.Equal(t, 1, queue.Version)
	require.Equal(t, lastHealthCheck, queue.LastHealthCheckAt)
}

func TestRegisterRunnerQueueError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.RegisterRunnerQueue(
		context.Background(), &p42.RegisterRunnerQueueRequest{
			TenantID:  "abc",
			RunnerID:  "runner1",
			QueueID:   "queue1",
			PublicKey: "key",
		},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestRegisterRunnerQueuePathEscaping(t *testing.T) {
	t.Parallel()

	publicKey := "key"

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedRunnerID, parts[5], "RunnerID not properly escaped in URL path")
			require.Equal(t, escapedQueueID, parts[7], "QueueID not properly escaped in URL path")

			w.WriteHeader(http.StatusCreated)
			resp := p42.RunnerQueue{
				TenantID:  tenantIDThatNeedsEscaping,
				RunnerID:  runnerIDThatNeedsEscaping,
				QueueID:   queueIDThatNeedsEscaping,
				PublicKey: publicKey,
				CreatedAt: time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
				Version:   1,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.RegisterRunnerQueue(
		context.Background(), &p42.RegisterRunnerQueueRequest{
			TenantID:  tenantIDThatNeedsEscaping,
			RunnerID:  runnerIDThatNeedsEscaping,
			QueueID:   queueIDThatNeedsEscaping,
			PublicKey: publicKey,
		},
	)
	require.NoError(t, err)
}

func TestUpdateRunnerQueue(t *testing.T) {
	t.Parallel()

	isHealthy := false
	draining := true
	failedChecks := 3
	successfulChecks := 1
	lastHealthCheck := time.Date(2024, time.January, 1, 12, 30, 0, 0, time.UTC)

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, "/v1/tenants/abc/runners/runner1/queues/queue1", r.URL.Path)
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			require.Equal(t, "application/json", r.Header.Get("Accept"))
			require.Equal(t, "2", r.Header.Get("If-Match"))

			var reqBody p42.UpdateRunnerQueueRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.NotNil(t, reqBody.IsHealthy)
			require.Equal(t, isHealthy, *reqBody.IsHealthy)
			require.NotNil(t, reqBody.Draining)
			require.Equal(t, draining, *reqBody.Draining)
			require.NotNil(t, reqBody.NConsecutiveFailedHealthChecks)
			require.Equal(t, failedChecks, *reqBody.NConsecutiveFailedHealthChecks)
			require.NotNil(t, reqBody.NConsecutiveSuccessfulHealthChecks)
			require.Equal(t, successfulChecks, *reqBody.NConsecutiveSuccessfulHealthChecks)
			require.NotNil(t, reqBody.LastHealthCheckAt)
			require.Equal(t, lastHealthCheck, *reqBody.LastHealthCheckAt)
			require.Empty(t, reqBody.TenantID)
			require.Empty(t, reqBody.RunnerID)
			require.Empty(t, reqBody.QueueID)
			require.Zero(t, reqBody.Version)

			w.WriteHeader(http.StatusOK)
			resp := p42.RunnerQueue{
				TenantID:                           "abc",
				RunnerID:                           "runner1",
				QueueID:                            "queue1",
				PublicKey:                          "pub",
				CreatedAt:                          time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
				Version:                            3,
				IsHealthy:                          isHealthy,
				Draining:                           draining,
				NConsecutiveFailedHealthChecks:     failedChecks,
				NConsecutiveSuccessfulHealthChecks: successfulChecks,
				LastHealthCheckAt:                  lastHealthCheck,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	queue, err := client.UpdateRunnerQueue(
		context.Background(), &p42.UpdateRunnerQueueRequest{
			TenantID:                           "abc",
			RunnerID:                           "runner1",
			QueueID:                            "queue1",
			Version:                            2,
			IsHealthy:                          util.Pointer(isHealthy),
			Draining:                           util.Pointer(draining),
			NConsecutiveFailedHealthChecks:     util.Pointer(failedChecks),
			NConsecutiveSuccessfulHealthChecks: util.Pointer(successfulChecks),
			LastHealthCheckAt:                  util.Pointer(lastHealthCheck),
		},
	)
	require.NoError(t, err)
	require.Equal(t, "queue1", queue.QueueID)
	require.Equal(t, 3, queue.Version)
	require.Equal(t, lastHealthCheck, queue.LastHealthCheckAt)
}

func TestUpdateRunnerQueueError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.UpdateRunnerQueue(
		context.Background(), &p42.UpdateRunnerQueueRequest{
			TenantID: "abc",
			RunnerID: "runner1",
			QueueID:  "queue1",
			Version:  1,
		},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestUpdateRunnerQueuePathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedRunnerID, parts[5], "RunnerID not properly escaped in URL path")
			require.Equal(t, escapedQueueID, parts[7], "QueueID not properly escaped in URL path")
			require.Equal(t, "1", r.Header.Get("If-Match"))

			w.WriteHeader(http.StatusOK)
			resp := p42.RunnerQueue{
				TenantID:  tenantIDThatNeedsEscaping,
				RunnerID:  runnerIDThatNeedsEscaping,
				QueueID:   queueIDThatNeedsEscaping,
				PublicKey: "pub",
				CreatedAt: time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
				Version:   2,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.UpdateRunnerQueue(
		context.Background(), &p42.UpdateRunnerQueueRequest{
			TenantID: tenantIDThatNeedsEscaping,
			RunnerID: runnerIDThatNeedsEscaping,
			QueueID:  queueIDThatNeedsEscaping,
			Version:  1,
		},
	)
	require.NoError(t, err)
}

func TestDeleteRunnerQueue(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, "/v1/tenants/abc/runners/runner1/queues/queue1", r.URL.Path)
			require.Equal(t, "1", r.Header.Get("If-Match"))
			require.Equal(t, "application/json", r.Header.Get("Accept"))

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteRunnerQueue(
		context.Background(), &p42.DeleteRunnerQueueRequest{
			TenantID: "abc",
			RunnerID: "runner1",
			QueueID:  "queue1",
			Version:  1,
		},
	)
	require.NoError(t, err)
}

func TestDeleteRunnerQueueError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	err := client.DeleteRunnerQueue(
		context.Background(), &p42.DeleteRunnerQueueRequest{
			TenantID: "abc",
			RunnerID: "runner1",
			QueueID:  "queue1",
			Version:  1,
		},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestDeleteRunnerQueuePathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(
				t,
				"/v1/tenants/"+escapedTenantID+"/runners/"+escapedRunnerID+"/queues/"+escapedQueueID,
				r.URL.EscapedPath(),
			)
			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteRunnerQueue(
		context.Background(), &p42.DeleteRunnerQueueRequest{
			TenantID: tenantIDThatNeedsEscaping,
			RunnerID: runnerIDThatNeedsEscaping,
			QueueID:  queueIDThatNeedsEscaping,
			Version:  1,
		},
	)
	require.NoError(t, err)
}

func TestGetRunnerQueue(t *testing.T) {
	t.Parallel()

	now := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/runners/runner1/queues/queue1", r.URL.Path)
			require.Equal(t, "application/json", r.Header.Get("Accept"))

			w.WriteHeader(http.StatusOK)
			resp := p42.RunnerQueue{
				TenantID:  "abc",
				RunnerID:  "runner1",
				QueueID:   "queue1",
				PublicKey: "pub",
				CreatedAt: now,
				Version:   1,
				IsHealthy: true,
				Draining:  false,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	queue, err := client.GetRunnerQueue(
		context.Background(), &p42.GetRunnerQueueRequest{
			TenantID: "abc",
			RunnerID: "runner1",
			QueueID:  "queue1",
		},
	)
	require.NoError(t, err)
	require.Equal(t, "queue1", queue.QueueID)
	require.Equal(t, now, queue.CreatedAt)
}

func TestGetRunnerQueueError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.GetRunnerQueue(
		context.Background(), &p42.GetRunnerQueueRequest{
			TenantID: "abc",
			RunnerID: "runner1",
			QueueID:  "queue1",
		},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestGetRunnerQueuePathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedRunnerID, parts[5], "RunnerID not properly escaped in URL path")
			require.Equal(t, escapedQueueID, parts[7], "QueueID not properly escaped in URL path")

			w.WriteHeader(http.StatusOK)
			resp := p42.RunnerQueue{
				TenantID:  tenantIDThatNeedsEscaping,
				RunnerID:  runnerIDThatNeedsEscaping,
				QueueID:   queueIDThatNeedsEscaping,
				PublicKey: "pub",
				CreatedAt: time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
				Version:   1,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetRunnerQueue(
		context.Background(), &p42.GetRunnerQueueRequest{
			TenantID: tenantIDThatNeedsEscaping,
			RunnerID: runnerIDThatNeedsEscaping,
			QueueID:  queueIDThatNeedsEscaping,
		},
	)
	require.NoError(t, err)
}

func TestGetRunnerQueueValidation(t *testing.T) {
	t.Parallel()

	client := p42.NewClient("http://example.com")

	_, err := client.GetRunnerQueue(context.Background(), nil)
	require.EqualError(t, err, "req is nil")

	_, err = client.GetRunnerQueue(context.Background(), &p42.GetRunnerQueueRequest{})
	require.EqualError(t, err, "tenant id is required")

	_, err = client.GetRunnerQueue(context.Background(), &p42.GetRunnerQueueRequest{TenantID: "abc"})
	require.EqualError(t, err, "runner id is required")

	_, err = client.GetRunnerQueue(
		context.Background(),
		&p42.GetRunnerQueueRequest{TenantID: "abc", RunnerID: "runner1"},
	)
	require.EqualError(t, err, "queue id is required")
}

func TestGetRunner(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		setInclude     bool
		includeDeleted bool
		expectedValue  string
	}{
		{name: "default", expectedValue: ""},
		{name: "include-deleted", setInclude: true, includeDeleted: true, expectedValue: "true"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(
			tc.name, func(t *testing.T) {
				t.Parallel()

				handler := http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						require.Equal(t, http.MethodGet, r.Method)
						require.Equal(t, "/v1/tenants/abc/runners/run", r.URL.Path)
						require.Equal(t, tc.expectedValue, r.URL.Query().Get("includeDeleted"))

						w.WriteHeader(http.StatusOK)
						resp := p42.Runner{TenantID: "abc", RunnerID: "run"}
						_ = json.NewEncoder(w).Encode(resp)
					},
				)

				srv := httptest.NewServer(handler)
				defer srv.Close()

				client := p42.NewClient(srv.URL)
				req := &p42.GetRunnerRequest{TenantID: "abc", RunnerID: "run"}
				if tc.setInclude {
					includeDeleted := tc.includeDeleted
					req.IncludeDeleted = &includeDeleted
				}

				runner, err := client.GetRunner(context.Background(), req)
				require.NoError(t, err)
				require.Equal(t, "run", runner.RunnerID)
			},
		)
	}
}

func TestListRunners(t *testing.T) {
	t.Parallel()

	maxResults := 20
	token := tokenID

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/runners", r.URL.Path)
			require.Equal(t, "20", r.URL.Query().Get("maxResults"))
			require.Equal(t, tokenID, r.URL.Query().Get("token"))
			require.Empty(t, r.URL.Query().Get("includeDeleted"))
			require.Empty(t, r.URL.Query().Get("runsTasks"))
			require.Empty(t, r.URL.Query().Get("proxiesGithub"))

			w.WriteHeader(http.StatusOK)
			resp := p42.List[*p42.Runner]{
				Items: []*p42.Runner{{RunnerID: "run"}},
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.ListRunners(
		context.Background(),
		&p42.ListRunnersRequest{TenantID: "abc", MaxResults: &maxResults, Token: &token},
	)
	require.NoError(t, err)
}

func TestListRunnersIncludeDeleted(t *testing.T) {
	t.Parallel()

	maxResults := 20
	token := tokenID
	includeDeleted := true
	runsTasks := true
	proxiesGithub := false

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/runners", r.URL.Path)
			require.Equal(t, "20", r.URL.Query().Get("maxResults"))
			require.Equal(t, tokenID, r.URL.Query().Get("token"))
			require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))
			require.Equal(t, "true", r.URL.Query().Get("runsTasks"))
			require.Equal(t, "false", r.URL.Query().Get("proxiesGithub"))

			w.WriteHeader(http.StatusOK)
			resp := p42.List[*p42.Runner]{
				Items: []*p42.Runner{{RunnerID: "run"}},
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.ListRunners(
		context.Background(), &p42.ListRunnersRequest{
			TenantID:       "abc",
			MaxResults:     &maxResults,
			Token:          &token,
			IncludeDeleted: &includeDeleted,
			RunsTasks:      &runsTasks,
			ProxiesGithub:  &proxiesGithub,
		},
	)
	require.NoError(t, err)
}

func TestDeleteRunner(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, "/v1/tenants/abc/runners/runner1", r.URL.Path)
			require.Equal(t, "1", r.Header.Get("If-Match"))
			require.Equal(t, "application/json", r.Header.Get("Accept"))

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteRunner(
		context.Background(), &p42.DeleteRunnerRequest{
			TenantID: "abc",
			RunnerID: "runner1",
			Version:  1,
		},
	)
	require.NoError(t, err)
}

func TestDeleteRunnerError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	err := client.DeleteRunner(
		context.Background(), &p42.DeleteRunnerRequest{
			TenantID: "abc",
			RunnerID: "runner1",
			Version:  1,
		},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestDeleteRunnerConflictError(t *testing.T) {
	t.Parallel()

	srv, client := serveRunnerConflict()
	defer srv.Close()

	err := client.DeleteRunner(
		context.Background(), &p42.DeleteRunnerRequest{
			TenantID: "abc",
			RunnerID: "runner1",
			Version:  1,
		},
	)
	verifyRunnerConflict(t, err)
}

func TestDeleteRunnerPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedRunnerID, parts[5], "RunnerID not properly escaped in URL path")

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteRunner(
		context.Background(), &p42.DeleteRunnerRequest{
			TenantID: tenantIDThatNeedsEscaping,
			RunnerID: runnerIDThatNeedsEscaping,
			Version:  1,
		},
	)
	require.NoError(t, err)
}

func TestGetRunnerBasic(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/runners/runner", r.URL.Path)

			w.WriteHeader(http.StatusOK)
			resp := p42.Runner{TenantID: "abc", RunnerID: "runner"}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	runner, err := client.GetRunner(context.Background(), &p42.GetRunnerRequest{TenantID: "abc", RunnerID: "runner"})
	require.NoError(t, err)
	require.Equal(t, "runner", runner.RunnerID)
}

func TestGetRunnerError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(
				p42.Error{
					ResponseCode: http.StatusNotFound,
					Message:      "nope",
					ErrorType:    "NotFound",
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetRunner(context.Background(), &p42.GetRunnerRequest{TenantID: "abc", RunnerID: "runner"})
	require.Error(t, err)
}

// nolint:dupl
func TestGetRunnerPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedRunnerID, parts[5], "RunnerID not properly escaped in URL path")

			w.WriteHeader(http.StatusOK)
			resp := p42.Runner{TenantID: tenantIDThatNeedsEscaping, RunnerID: runnerIDThatNeedsEscaping}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetRunner(
		context.Background(),
		&p42.GetRunnerRequest{TenantID: tenantIDThatNeedsEscaping, RunnerID: runnerIDThatNeedsEscaping},
	)
	require.NoError(t, err)
}

func TestUpdateRunner(t *testing.T) {
	t.Parallel()

	name := "runner-updated"
	description := "runner-desc"
	isCloud := true
	runsTasks := false
	proxiesGithub := true

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, "/v1/tenants/abc/runners/runner1", r.URL.Path)
			require.Equal(t, "2", r.Header.Get("If-Match"))

			var reqBody p42.UpdateRunnerRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.NotNil(t, reqBody.Name)
			require.Equal(t, name, *reqBody.Name)
			require.NotNil(t, reqBody.Description)
			require.Equal(t, description, *reqBody.Description)
			require.NotNil(t, reqBody.IsCloud)
			require.True(t, *reqBody.IsCloud)
			require.NotNil(t, reqBody.RunsTasks)
			require.False(t, *reqBody.RunsTasks)
			require.NotNil(t, reqBody.ProxiesGithub)
			require.True(t, *reqBody.ProxiesGithub)
			require.Zero(t, reqBody.Version)
			require.Empty(t, reqBody.TenantID)
			require.Empty(t, reqBody.RunnerID)

			now := time.Date(2024, time.January, 2, 0, 0, 0, 0, time.UTC)
			w.WriteHeader(http.StatusOK)
			resp := p42.Runner{
				TenantID:      "abc",
				RunnerID:      "runner1",
				Name:          name,
				Description:   &description,
				IsCloud:       true,
				RunsTasks:     false,
				ProxiesGithub: true,
				CreatedAt:     now,
				UpdatedAt:     now,
				Version:       2,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	runner, err := client.UpdateRunner(
		context.Background(), &p42.UpdateRunnerRequest{
			TenantID:      "abc",
			RunnerID:      "runner1",
			Version:       2,
			Name:          &name,
			Description:   &description,
			IsCloud:       &isCloud,
			RunsTasks:     &runsTasks,
			ProxiesGithub: &proxiesGithub,
		},
	)
	require.NoError(t, err)
	require.Equal(t, "runner1", runner.RunnerID)
	require.NotNil(t, runner.Description)
	require.Equal(t, description, *runner.Description)
}

func TestUpdateRunnerError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.UpdateRunner(
		context.Background(),
		&p42.UpdateRunnerRequest{TenantID: "abc", RunnerID: "runner1", Version: 1},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestUpdateRunnerConflictError(t *testing.T) {
	t.Parallel()

	srv, client := serveRunnerConflict()
	defer srv.Close()

	_, err := client.UpdateRunner(
		context.Background(),
		&p42.UpdateRunnerRequest{TenantID: "abc", RunnerID: "runner1", Version: 1},
	)
	verifyRunnerConflict(t, err)
}

func TestUpdateRunnerPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedRunnerID, parts[5], "RunnerID not properly escaped in URL path")
			require.Equal(t, "1", r.Header.Get("If-Match"))

			w.WriteHeader(http.StatusOK)
			now := time.Date(2024, time.January, 3, 0, 0, 0, 0, time.UTC)
			resp := p42.Runner{
				TenantID:  tenantIDThatNeedsEscaping,
				RunnerID:  runnerIDThatNeedsEscaping,
				Name:      "runner",
				CreatedAt: now,
				UpdatedAt: now,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	isCloud := true
	_, err := client.UpdateRunner(
		context.Background(), &p42.UpdateRunnerRequest{
			TenantID: tenantIDThatNeedsEscaping,
			RunnerID: runnerIDThatNeedsEscaping,
			Version:  1,
			IsCloud:  &isCloud,
		},
	)
	require.NoError(t, err)
}

func TestRevokeRunnerToken(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/v1/tenants/abc/runners/runner1/tokens/token1/revoke", r.URL.Path)
			require.Equal(t, "application/json", r.Header.Get("Accept"))

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.RevokeRunnerToken(
		context.Background(), &p42.RevokeRunnerTokenRequest{
			TenantID: "abc",
			RunnerID: "runner1",
			TokenID:  "token1",
		},
	)
	require.NoError(t, err)
}

func TestRevokeRunnerTokenError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	err := client.RevokeRunnerToken(
		context.Background(),
		&p42.RevokeRunnerTokenRequest{TenantID: "abc", RunnerID: "runner1", TokenID: "token1"},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestRevokeRunnerTokenPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 9, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedRunnerID, parts[5], "RunnerID not properly escaped in URL path")
			require.Equal(t, escapedTokenID, parts[7], "TokenID not properly escaped in URL path")

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.RevokeRunnerToken(
		context.Background(), &p42.RevokeRunnerTokenRequest{
			TenantID: tenantIDThatNeedsEscaping,
			RunnerID: runnerIDThatNeedsEscaping,
			TokenID:  tokenIDThatNeedsEscaping,
		},
	)
	require.NoError(t, err)
}

func TestGetRunnerToken(t *testing.T) {
	t.Parallel()

	now := time.Date(2024, time.March, 3, 0, 0, 0, 0, time.UTC)

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/runners/runner1/tokens/token1", r.URL.Path)
			require.Empty(t, r.URL.RawQuery)

			w.WriteHeader(http.StatusOK)
			resp := p42.RunnerTokenMetadata{
				TenantID:      "abc",
				RunnerID:      "runner1",
				TokenID:       "token1",
				CreatedAt:     now,
				ExpiresAt:     now.Add(time.Hour),
				Revoked:       false,
				Version:       1,
				SignatureHash: "hash",
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	token, err := client.GetRunnerToken(
		context.Background(), &p42.GetRunnerTokenRequest{
			TenantID: "abc",
			RunnerID: "runner1",
			TokenID:  "token1",
		},
	)
	require.NoError(t, err)
	require.Equal(t, "token1", token.TokenID)
	require.Equal(t, now, token.CreatedAt)
	require.False(t, token.Revoked)
}

func TestGetRunnerTokenIncludeDeleted(t *testing.T) {
	t.Parallel()

	now := time.Date(2024, time.March, 3, 0, 0, 0, 0, time.UTC)
	includeDeleted := true

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/runners/runner1/tokens/token1", r.URL.Path)
			require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.RunnerTokenMetadata{
				TenantID:      "abc",
				RunnerID:      "runner1",
				TokenID:       "token1",
				CreatedAt:     now,
				ExpiresAt:     now.Add(time.Hour),
				Revoked:       true,
				Version:       2,
				SignatureHash: "hash",
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetRunnerToken(
		context.Background(), &p42.GetRunnerTokenRequest{
			TenantID:       "abc",
			RunnerID:       "runner1",
			TokenID:        "token1",
			IncludeDeleted: &includeDeleted,
		},
	)
	require.NoError(t, err)
}

func TestGetRunnerTokenError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.GetRunnerToken(
		context.Background(),
		&p42.GetRunnerTokenRequest{TenantID: "abc", RunnerID: "runner1", TokenID: "token1"},
	)
	require.Error(t, err)
}

func TestGetRunnerTokenPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedRunnerID, parts[5], "RunnerID not properly escaped in URL path")
			require.Equal(t, escapedTokenID, parts[7], "TokenID not properly escaped in URL path")

			w.WriteHeader(http.StatusOK)
			resp := p42.RunnerTokenMetadata{
				TenantID: tenantIDThatNeedsEscaping,
				RunnerID: runnerIDThatNeedsEscaping,
				TokenID:  tokenIDThatNeedsEscaping,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetRunnerToken(
		context.Background(), &p42.GetRunnerTokenRequest{
			TenantID: tenantIDThatNeedsEscaping,
			RunnerID: runnerIDThatNeedsEscaping,
			TokenID:  tokenIDThatNeedsEscaping,
		},
	)
	require.NoError(t, err)
}

func TestGetRunnerTokenValidation(t *testing.T) {
	t.Parallel()

	client := p42.NewClient("http://example.com")
	_, err := client.GetRunnerToken(context.Background(), nil)
	require.Error(t, err)

	_, err = client.GetRunnerToken(
		context.Background(),
		&p42.GetRunnerTokenRequest{RunnerID: "runner", TokenID: "token"},
	)
	require.EqualError(t, err, "tenant id is required")

	_, err = client.GetRunnerToken(
		context.Background(),
		&p42.GetRunnerTokenRequest{TenantID: "tenant", TokenID: "token"},
	)
	require.EqualError(t, err, "runner id is required")

	_, err = client.GetRunnerToken(
		context.Background(),
		&p42.GetRunnerTokenRequest{TenantID: "tenant", RunnerID: "runner"},
	)
	require.EqualError(t, err, "token id is required")
}

func generatePublicKey(t *testing.T) *ecdsa.PublicKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return &priv.PublicKey
}

func newTestWrappedSecret(t *testing.T) *ecies.WrappedSecret {
	t.Helper()
	return &ecies.WrappedSecret{
		EncryptedData:      []byte("payload"),
		EphemeralPublicKey: generatePublicKey(t),
	}
}

func TestGetMessagesBatch(t *testing.T) {
	t.Parallel()

	createdAt := time.Unix(0, 0).UTC()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/runners/run/queues/queue/messages", r.URL.Path)
			require.Equal(t, "application/json", r.Header.Get("Accept"))

			w.WriteHeader(http.StatusOK)
			resp := p42.GetMessagesBatchResponse{
				Messages: []*p42.RunnerMessage{
					{
						TenantID:        "abc",
						RunnerID:        "run",
						QueueID:         "queue",
						MessageID:       "msg",
						CallerID:        "caller",
						CallerPublicKey: "pk",
						CreatedAt:       createdAt,
						Payload: &ecies.WrappedSecret{
							EncryptedData:      []byte("data"),
							EphemeralPublicKey: generatePublicKey(t),
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	resp, err := client.GetMessagesBatch(
		context.Background(), &p42.GetMessagesBatchRequest{
			TenantID: "abc",
			RunnerID: "run",
			QueueID:  "queue",
		},
	)
	require.NoError(t, err)
	require.Len(t, resp.Messages, 1)
	msg := resp.Messages[0]
	require.Equal(t, "msg", msg.MessageID)
	require.True(t, msg.CreatedAt.Equal(createdAt))
}

func TestGetMessagesBatchError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(
				p42.Error{
					ResponseCode: http.StatusBadRequest,
					Message:      "bad",
					ErrorType:    "BadRequest",
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetMessagesBatch(
		context.Background(), &p42.GetMessagesBatchRequest{
			TenantID: "tenant",
			RunnerID: "runner",
			QueueID:  "queue",
		},
	)
	require.Error(t, err)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestGetMessagesBatchPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(
				t,
				"/v1/tenants/"+escapedTenantID+"/runners/"+escapedRunnerID+"/queues/"+escapedQueueID+"/messages",
				r.URL.EscapedPath(),
			)

			w.WriteHeader(http.StatusOK)
			resp := p42.GetMessagesBatchResponse{}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetMessagesBatch(
		context.Background(), &p42.GetMessagesBatchRequest{
			TenantID: tenantIDThatNeedsEscaping,
			RunnerID: runnerIDThatNeedsEscaping,
			QueueID:  queueIDThatNeedsEscaping,
		},
	)
	require.NoError(t, err)
}

func TestGetMessagesBatchValidation(t *testing.T) {
	t.Parallel()

	client := p42.NewClient("http://example.com")
	_, err := client.GetMessagesBatch(context.Background(), nil)
	require.Error(t, err)

	_, err = client.GetMessagesBatch(
		context.Background(),
		&p42.GetMessagesBatchRequest{RunnerID: "runner", QueueID: "queue"},
	)
	require.EqualError(t, err, "tenant id is required")

	_, err = client.GetMessagesBatch(
		context.Background(),
		&p42.GetMessagesBatchRequest{TenantID: "tenant", QueueID: "queue"},
	)
	require.EqualError(t, err, "runner id is required")

	_, err = client.GetMessagesBatch(
		context.Background(),
		&p42.GetMessagesBatchRequest{TenantID: "tenant", RunnerID: "runner"},
	)
	require.EqualError(t, err, "queue id is required")
}

func TestListRunnerTokens(t *testing.T) {
	t.Parallel()

	maxResults := 25
	includeRevoked := true
	nextToken := "next-token"
	now := time.Date(2024, time.February, 2, 0, 0, 0, 0, time.UTC)
	revoked := now.Add(2 * time.Hour)

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/runners/runner1/tokens", r.URL.Path)
			require.Equal(t, "25", r.URL.Query().Get("maxResults"))
			require.Equal(t, nextToken, r.URL.Query().Get("nextPageToken"))
			require.Equal(t, "true", r.URL.Query().Get("includeRevoked"))

			w.WriteHeader(http.StatusOK)
			resp := p42.List[*p42.RunnerTokenMetadata]{
				NextToken: util.Pointer("more"),
				Items: []*p42.RunnerTokenMetadata{
					{
						TokenID:       "token1",
						CreatedAt:     now,
						ExpiresAt:     now.Add(time.Hour),
						RevokedAt:     util.Pointer(revoked),
						SignatureHash: "hash",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	resp, err := client.ListRunnerTokens(
		context.Background(), &p42.ListRunnerTokensRequest{
			TenantID:       "abc",
			RunnerID:       "runner1",
			MaxResults:     &maxResults,
			NextPageToken:  &nextToken,
			IncludeRevoked: &includeRevoked,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, resp.NextToken)
	require.Equal(t, "more", *resp.NextToken)
	require.Len(t, resp.Items, 1)
	require.Equal(t, "token1", resp.Items[0].TokenID)
	require.NotNil(t, resp.Items[0].RevokedAt)
	if resp.Items[0].RevokedAt != nil {
		require.True(t, resp.Items[0].RevokedAt.Equal(revoked))
	}
}

func TestListRunnerTokensError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListRunnerTokens(
		context.Background(),
		&p42.ListRunnerTokensRequest{TenantID: "abc", RunnerID: "runner1"},
	)
	require.Error(t, err)
}

func TestListRunnerTokensPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 7, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedRunnerID, parts[5], "RunnerID not properly escaped in URL path")
			require.Equal(t, "tokens", parts[6], "tokens segment missing in URL path")

			w.WriteHeader(http.StatusOK)
			resp := p42.List[*p42.RunnerTokenMetadata]{}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.ListRunnerTokens(
		context.Background(), &p42.ListRunnerTokensRequest{
			TenantID: tenantIDThatNeedsEscaping,
			RunnerID: runnerIDThatNeedsEscaping,
		},
	)
	require.NoError(t, err)
}

func TestListRunnerTokensValidation(t *testing.T) {
	t.Parallel()

	client := p42.NewClient("http://example.com")
	_, err := client.ListRunnerTokens(context.Background(), nil)
	require.Error(t, err)

	_, err = client.ListRunnerTokens(context.Background(), &p42.ListRunnerTokensRequest{RunnerID: "runner"})
	require.EqualError(t, err, "tenant id is required")

	_, err = client.ListRunnerTokens(context.Background(), &p42.ListRunnerTokensRequest{TenantID: "tenant"})
	require.EqualError(t, err, "runner id is required")
}

func TestWriteResponse(t *testing.T) {
	t.Parallel()

	payload := newTestWrappedSecret(t)

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "/v1/tenants/abc/runners/runner1/queues/queue1/messages/message1/response", r.URL.Path)
			require.Equal(t, "application/json", r.Header.Get("Accept"))
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))

			var req p42.WriteResponseRequest

			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			require.Equal(t, "caller", req.CallerID)
			require.NotNil(t, req.Payload)
			reqPayload := req.Payload.(*ecies.WrappedSecret)
			require.Equal(t, payload.EncryptedData, reqPayload.EncryptedData)
			expectedPem, err := ecies.PubKeyToPem(payload.EphemeralPublicKey)
			require.NoError(t, err)
			actualPem, err := ecies.PubKeyToPem(reqPayload.EphemeralPublicKey)
			require.NoError(t, err)
			require.Equal(t, expectedPem, actualPem)

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.WriteResponse(
		context.Background(), &p42.WriteResponseRequest{
			TenantID:  "abc",
			RunnerID:  "runner1",
			QueueID:   "queue1",
			MessageID: "message1",
			CallerID:  "caller",
			Payload:   payload,
		},
	)
	require.NoError(t, err)
}

func TestWriteResponseError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	err := client.WriteResponse(
		context.Background(), &p42.WriteResponseRequest{
			TenantID:  "abc",
			RunnerID:  "runner1",
			QueueID:   "queue1",
			MessageID: "message1",
			CallerID:  "caller",
			Payload:   newTestWrappedSecret(t),
		},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestWriteResponsePathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 11, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedRunnerID, parts[5], "RunnerID not properly escaped in URL path")
			require.Equal(t, escapedQueueID, parts[7], "QueueID not properly escaped in URL path")
			require.Equal(t, escapedMessageID, parts[9], "MessageID not properly escaped in URL path")
			require.Equal(t, "response", parts[10], "response segment missing in URL path")

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.WriteResponse(
		context.Background(), &p42.WriteResponseRequest{
			TenantID:  tenantIDThatNeedsEscaping,
			RunnerID:  runnerIDThatNeedsEscaping,
			QueueID:   queueIDThatNeedsEscaping,
			MessageID: messageIDThatNeedsEscaping,
			CallerID:  "caller",
			Payload:   newTestWrappedSecret(t),
		},
	)
	require.NoError(t, err)
}

func TestWriteResponseValidation(t *testing.T) {
	t.Parallel()

	client := p42.NewClient("http://example.com")

	err := client.WriteResponse(context.Background(), nil)
	require.EqualError(t, err, "req is nil")

	err = client.WriteResponse(context.Background(), &p42.WriteResponseRequest{})
	require.EqualError(t, err, "tenant id is required")

	err = client.WriteResponse(context.Background(), &p42.WriteResponseRequest{TenantID: "tenant"})
	require.EqualError(t, err, "runner id is required")

	err = client.WriteResponse(context.Background(), &p42.WriteResponseRequest{TenantID: "tenant", RunnerID: "runner"})
	require.EqualError(t, err, "queue id is required")

	err = client.WriteResponse(
		context.Background(),
		&p42.WriteResponseRequest{TenantID: "tenant", RunnerID: "runner", QueueID: "queue"},
	)
	require.EqualError(t, err, "message id is required")

	err = client.WriteResponse(
		context.Background(),
		&p42.WriteResponseRequest{TenantID: "tenant", RunnerID: "runner", QueueID: "queue", MessageID: "message"},
	)
	require.EqualError(t, err, "caller id is required")

	err = client.WriteResponse(
		context.Background(),
		&p42.WriteResponseRequest{
			TenantID:  "tenant",
			RunnerID:  "runner",
			QueueID:   "queue",
			MessageID: "message",
			CallerID:  "caller",
		},
	)
	require.EqualError(t, err, "payload is required")
}

// nolint: dupl
func TestGetEnvironment(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/environments/env", r.URL.Path)
			require.Empty(t, r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.Environment{TenantID: "abc", EnvironmentID: "env"}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	env, err := client.GetEnvironment(
		context.Background(),
		&p42.GetEnvironmentRequest{TenantID: "abc", EnvironmentID: "env"},
	)
	require.NoError(t, err)
	require.Equal(t, "env", env.EnvironmentID)
}

func TestGetEnvironmentIncludeDeleted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/environments/env", r.URL.Path)
			require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.Environment{TenantID: "abc", EnvironmentID: "env"}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	includeDeleted := true
	_, err := client.GetEnvironment(
		context.Background(),
		&p42.GetEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", IncludeDeleted: &includeDeleted},
	)
	require.NoError(t, err)
}

func TestGetEnvironmentError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(
				p42.Error{
					ResponseCode: http.StatusNotFound,
					Message:      "nope",
					ErrorType:    "NotFound",
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetEnvironment(
		context.Background(),
		&p42.GetEnvironmentRequest{TenantID: "abc", EnvironmentID: "env"},
	)
	require.Error(t, err)
}

// nolint: dupl
func TestGetEnvironmentPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedEnvironmentID, parts[5], "EnvironmentID not properly escaped in URL path")

			w.WriteHeader(http.StatusOK)
			resp := p42.Environment{TenantID: tenantIDThatNeedsEscaping, EnvironmentID: environmentIDThatNeedsEscaping}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetEnvironment(
		context.Background(),
		&p42.GetEnvironmentRequest{TenantID: tenantIDThatNeedsEscaping, EnvironmentID: environmentIDThatNeedsEscaping},
	)
	require.NoError(t, err)
}

// nolint: dupl
func TestListEnvironments(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/environments", r.URL.Path)
			require.Equal(t, "123", r.URL.Query().Get("maxResults"))
			require.Equal(t, tokenID, r.URL.Query().Get("token"))
			require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.List[p42.Environment]{Items: []p42.Environment{{EnvironmentID: "env"}}}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	maxResults := 123
	includeDeleted := true
	resp, err := client.ListEnvironments(
		context.Background(),
		&p42.ListEnvironmentsRequest{
			TenantID:       "abc",
			MaxResults:     &maxResults,
			Token:          util.Pointer(tokenID),
			IncludeDeleted: &includeDeleted,
		},
	)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	require.Equal(t, "env", resp.Items[0].EnvironmentID)
}

func TestListEnvironmentsError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()
	_, err := client.ListEnvironments(context.Background(), &p42.ListEnvironmentsRequest{TenantID: "abc"})
	require.Error(t, err)
}

func TestListEnvironmentsPathEscaping(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")

			w.WriteHeader(http.StatusOK)
			resp := p42.List[p42.Environment]{}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.ListEnvironments(
		context.Background(),
		&p42.ListEnvironmentsRequest{TenantID: tenantIDThatNeedsEscaping},
	)
	require.NoError(t, err)
}

// nolint: dupl
func TestUpdateEnvironment(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, "/v1/tenants/abc/environments/env", r.URL.Path)
			require.Equal(t, "1", r.Header.Get("If-Match"))

			var reqBody p42.UpdateEnvironmentRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.NotNil(t, reqBody.Name)
			require.Equal(t, "env2", *reqBody.Name)

			w.WriteHeader(http.StatusOK)
			resp := p42.Environment{TenantID: "abc", EnvironmentID: "env"}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	name := "env2"
	env, err := client.UpdateEnvironment(
		context.Background(),
		&p42.UpdateEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", Version: 1, Name: &name},
	)
	require.NoError(t, err)
	require.Equal(t, "env", env.EnvironmentID)
}

func TestUpdateEnvironmentError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()
	_, err := client.UpdateEnvironment(
		context.Background(),
		&p42.UpdateEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", Version: 1},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestUpdateEnvironmentConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveEnvironmentConflict()
	defer srv.Close()
	_, err := client.UpdateEnvironment(
		context.Background(),
		&p42.UpdateEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", Version: 1},
	)
	verifyEnvironmentConflict(t, err)
}

func TestUpdateEnvironmentPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedEnvironmentID, parts[5], "EnvironmentID not properly escaped in URL path")

			w.WriteHeader(http.StatusOK)
			resp := p42.Environment{TenantID: tenantIDThatNeedsEscaping, EnvironmentID: environmentIDThatNeedsEscaping}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	name := "env"
	_, err := client.UpdateEnvironment(
		context.Background(),
		&p42.UpdateEnvironmentRequest{
			TenantID:      tenantIDThatNeedsEscaping,
			EnvironmentID: environmentIDThatNeedsEscaping,
			Version:       1,
			Name:          &name,
		},
	)
	require.NoError(t, err)
}

func TestDeleteEnvironment(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, "/v1/tenants/abc/environments/env", r.URL.Path)
			require.Equal(t, "1", r.Header.Get("If-Match"))

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteEnvironment(
		context.Background(),
		&p42.DeleteEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", Version: 1},
	)
	require.NoError(t, err)
}

func TestDeleteEnvironmentError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()
	err := client.DeleteEnvironment(
		context.Background(),
		&p42.DeleteEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", Version: 1},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestDeleteEnvironmentConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveEnvironmentConflict()
	defer srv.Close()
	err := client.DeleteEnvironment(
		context.Background(),
		&p42.DeleteEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", Version: 1},
	)
	verifyEnvironmentConflict(t, err)
}

func TestDeleteEnvironmentPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedEnvironmentID, parts[5], "EnvironmentID not properly escaped in URL path")

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteEnvironment(
		context.Background(),
		&p42.DeleteEnvironmentRequest{
			TenantID:      tenantIDThatNeedsEscaping,
			EnvironmentID: environmentIDThatNeedsEscaping,
			Version:       1,
		},
	)
	require.NoError(t, err)
}

func TestCreateTask(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "/v1/tenants/abc/tasks/task", r.URL.Path)

			var reqBody p42.CreateTaskRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.Equal(t, "title", reqBody.Title)

			w.WriteHeader(http.StatusCreated)
			resp := p42.Task{TenantID: "abc", TaskID: "task"}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	model := p42.ModelTypeCodexMini
	task, err := client.CreateTask(
		context.Background(), &p42.CreateTaskRequest{
			TenantID:      "abc",
			TaskID:        "task",
			Title:         "title",
			EnvironmentID: util.Pointer("env"),
			Prompt:        "do",
			Model:         &model,
			RepoInfo:      map[string]*p42.RepoInfo{},
		},
	)
	require.NoError(t, err)
	require.Equal(t, "task", task.TaskID)
}

func TestCreateWorkstreamTask(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "/v1/tenants/abc/workstreams/ws/tasks/task", r.URL.Path)

			var reqBody p42.CreateWorkstreamTaskRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.Equal(t, "title", reqBody.Title)
			require.NotNil(t, reqBody.EnvironmentID)
			require.Equal(t, "env", *reqBody.EnvironmentID)
			require.NotNil(t, reqBody.Prompt)
			require.Equal(t, "do", *reqBody.Prompt)
			require.NotNil(t, reqBody.Parallel)
			require.True(t, *reqBody.Parallel)
			require.NotNil(t, reqBody.Model)
			require.Equal(t, p42.ModelTypeCodexMini, *reqBody.Model)
			require.Nil(t, reqBody.AssignedToTenantID)
			require.True(t, reqBody.AssignedToAI)
			require.NotNil(t, reqBody.State)
			require.Equal(t, p42.TaskStatePending, *reqBody.State)
			require.Contains(t, reqBody.RepoInfo, "repo")
			repo := reqBody.RepoInfo["repo"]
			require.NotNil(t, repo)
			require.Equal(t, "feature", repo.FeatureBranch)
			require.Equal(t, "main", repo.TargetBranch)

			w.WriteHeader(http.StatusCreated)
			resp := p42.Task{TenantID: "abc", TaskID: "task"}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	model := p42.ModelTypeCodexMini
	parallel := true
	state := p42.TaskStatePending
	task, err := client.CreateWorkstreamTask(
		context.Background(), &p42.CreateWorkstreamTaskRequest{
			TenantID:      "abc",
			WorkstreamID:  "ws",
			TaskID:        "task",
			Title:         "title",
			EnvironmentID: util.Pointer("env"),
			Prompt:        util.Pointer("do"),
			Parallel:      &parallel,
			Model:         &model,
			AssignedToAI:  true,
			RepoInfo: map[string]*p42.RepoInfo{
				"repo": {
					FeatureBranch: "feature",
					TargetBranch:  "main",
				},
			},
			State: &state,
		},
	)
	require.NoError(t, err)
	require.Equal(t, "task", task.TaskID)
}

func TestCreateWorkstreamTaskError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.CreateWorkstreamTask(
		context.Background(), &p42.CreateWorkstreamTaskRequest{
			TenantID:     "abc",
			WorkstreamID: "ws",
			TaskID:       "task",
			Title:        "title",
			AssignedToAI: true,
		},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestCreateWorkstreamTaskConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveTaskConflict()
	defer srv.Close()

	_, err := client.CreateWorkstreamTask(
		context.Background(), &p42.CreateWorkstreamTaskRequest{
			TenantID:     "abc",
			WorkstreamID: "ws",
			TaskID:       "task",
			Title:        "title",
			AssignedToAI: true,
		},
	)
	verifyTaskConflict(t, err)
}

func TestCreateWorkstreamTaskPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedWorkstreamID, parts[5], "WorkstreamID not properly escaped in URL path")
			require.Equal(t, escapedTaskID, parts[7], "TaskID not properly escaped in URL path")

			w.WriteHeader(http.StatusCreated)
			resp := p42.Task{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	parallel := true
	_, err := client.CreateWorkstreamTask(
		context.Background(), &p42.CreateWorkstreamTaskRequest{
			TenantID:     tenantIDThatNeedsEscaping,
			WorkstreamID: workstreamIDThatNeedsEscaping,
			TaskID:       taskIDThatNeedsEscaping,
			Title:        "title",
			Parallel:     &parallel,
			AssignedToAI: true,
		},
	)
	require.NoError(t, err)
}

// nolint:dupl
func TestUpdateWorkstreamTask(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, "/v1/tenants/abc/workstreams/ws/tasks/task", r.URL.Path)
			require.Equal(t, "3", r.Header.Get("If-Match"))

			var reqBody p42.UpdateWorkstreamTaskRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.NotNil(t, reqBody.Title)
			require.Equal(t, taskTitle, *reqBody.Title)
			require.NotNil(t, reqBody.EnvironmentID)
			require.NotNil(t, *reqBody.EnvironmentID)
			require.Equal(t, "env", *reqBody.EnvironmentID)
			require.NotNil(t, reqBody.Prompt)
			require.Equal(t, "prompt", *reqBody.Prompt)
			require.NotNil(t, reqBody.Parallel)
			require.True(t, *reqBody.Parallel)
			require.NotNil(t, reqBody.Model)
			require.Equal(t, p42.ModelTypeCodexMini, *reqBody.Model)
			require.NotNil(t, reqBody.AssignedToTenantID)
			require.NotNil(t, *reqBody.AssignedToTenantID)
			require.Equal(t, "tenant-b", *reqBody.AssignedToTenantID)
			require.NotNil(t, reqBody.AssignedToAI)
			require.False(t, *reqBody.AssignedToAI)
			require.NotNil(t, reqBody.RepoInfo)
			require.Contains(t, *reqBody.RepoInfo, "repo")
			repo := (*reqBody.RepoInfo)["repo"]
			require.NotNil(t, repo)
			require.Equal(t, "feature", repo.FeatureBranch)
			require.Equal(t, "main", repo.TargetBranch)
			require.NotNil(t, reqBody.State)
			require.Equal(t, p42.TaskStateExecuting, *reqBody.State)
			require.NotNil(t, reqBody.BeforeTaskID)
			require.Equal(t, "before", *reqBody.BeforeTaskID)
			require.Nil(t, reqBody.AfterTaskID)
			require.NotNil(t, reqBody.Deleted)
			require.True(t, *reqBody.Deleted)

			w.WriteHeader(http.StatusOK)
			resp := p42.Task{TenantID: "abc", TaskID: "task"}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	title := util.Pointer(taskTitle)
	prompt := util.Pointer("prompt")
	parallel := true
	model := p42.ModelTypeCodexMini
	assignedToAI := false
	repoInfo := map[string]*p42.RepoInfo{
		"repo": {
			FeatureBranch: "feature",
			TargetBranch:  "main",
		},
	}
	state := p42.TaskStateExecuting
	before := util.Pointer("before")
	deleted := true

	task, err := client.UpdateWorkstreamTask(
		context.Background(), &p42.UpdateWorkstreamTaskRequest{
			TenantID:           "abc",
			WorkstreamID:       "ws",
			TaskID:             "task",
			Version:            3,
			Title:              title,
			EnvironmentID:      util.Pointer("env"),
			Prompt:             prompt,
			Parallel:           &parallel,
			Model:              &model,
			AssignedToTenantID: util.Pointer("tenant-b"),
			AssignedToAI:       util.Pointer(assignedToAI),
			RepoInfo:           &repoInfo,
			State:              &state,
			BeforeTaskID:       before,
			Deleted:            &deleted,
		},
	)
	require.NoError(t, err)
	require.Equal(t, "task", task.TaskID)
}

func TestUpdateWorkstreamTaskError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.UpdateWorkstreamTask(
		context.Background(), &p42.UpdateWorkstreamTaskRequest{
			TenantID:     "abc",
			WorkstreamID: "ws",
			TaskID:       "task",
			Version:      1,
			Title:        util.Pointer(taskTitle),
		},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestUpdateWorkstreamTaskConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveTaskConflict()
	defer srv.Close()

	_, err := client.UpdateWorkstreamTask(
		context.Background(), &p42.UpdateWorkstreamTaskRequest{
			TenantID:     "abc",
			WorkstreamID: "ws",
			TaskID:       "task",
			Version:      1,
			Title:        util.Pointer(taskTitle),
		},
	)
	verifyTaskConflict(t, err)
}

// nolint:dupl
func TestUpdateWorkstreamTaskPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedWorkstreamID, parts[5], "WorkstreamID not properly escaped in URL path")
			require.Equal(t, escapedTaskID, parts[7], "TaskID not properly escaped in URL path")

			w.WriteHeader(http.StatusOK)
			resp := p42.Task{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.UpdateWorkstreamTask(
		context.Background(), &p42.UpdateWorkstreamTaskRequest{
			TenantID:     tenantIDThatNeedsEscaping,
			WorkstreamID: workstreamIDThatNeedsEscaping,
			TaskID:       taskIDThatNeedsEscaping,
			Version:      1,
			Title:        util.Pointer("title"),
		},
	)
	require.NoError(t, err)
}

func TestDeleteWorkstreamTask(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, "/v1/tenants/abc/workstreams/ws/tasks/task", r.URL.Path)
			require.Equal(t, "1", r.Header.Get("If-Match"))

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteWorkstreamTask(
		context.Background(), &p42.DeleteWorkstreamTaskRequest{
			TenantID:     "abc",
			WorkstreamID: "ws",
			TaskID:       "task",
			Version:      1,
		},
	)
	require.NoError(t, err)
}

func TestDeleteWorkstreamTaskError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	err := client.DeleteWorkstreamTask(
		context.Background(), &p42.DeleteWorkstreamTaskRequest{
			TenantID:     "abc",
			WorkstreamID: "ws",
			TaskID:       "task",
			Version:      1,
		},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestDeleteWorkstreamTaskConflictError(t *testing.T) {
	t.Parallel()

	srv, client := serveTaskConflict()
	defer srv.Close()

	err := client.DeleteWorkstreamTask(
		context.Background(), &p42.DeleteWorkstreamTaskRequest{
			TenantID:     "abc",
			WorkstreamID: "ws",
			TaskID:       "task",
			Version:      1,
		},
	)
	verifyTaskConflict(t, err)
}

func TestDeleteWorkstreamTaskPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedWorkstreamID, parts[5], "WorkstreamID not properly escaped in URL path")
			require.Equal(t, escapedTaskID, parts[7], "TaskID not properly escaped in URL path")

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteWorkstreamTask(
		context.Background(), &p42.DeleteWorkstreamTaskRequest{
			TenantID:     tenantIDThatNeedsEscaping,
			WorkstreamID: workstreamIDThatNeedsEscaping,
			TaskID:       taskIDThatNeedsEscaping,
			Version:      1,
		},
	)
	require.NoError(t, err)
}

func TestCreateTaskError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()
	model := p42.ModelTypeCodexMini
	_, err := client.CreateTask(
		context.Background(), &p42.CreateTaskRequest{
			TenantID:      "abc",
			TaskID:        "task",
			Title:         "title",
			EnvironmentID: util.Pointer("env"),
			Prompt:        "do",
			Model:         &model,
			RepoInfo:      map[string]*p42.RepoInfo{},
		},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestCreateTaskConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveTaskConflict()
	defer srv.Close()
	model := p42.ModelTypeCodexMini
	_, err := client.CreateTask(
		context.Background(), &p42.CreateTaskRequest{
			TenantID:      "abc",
			TaskID:        "task",
			Title:         "title",
			EnvironmentID: util.Pointer("env"),
			Prompt:        "do",
			Model:         &model,
			RepoInfo:      map[string]*p42.RepoInfo{},
		},
	)
	verifyTaskConflict(t, err)
}

func TestCreateTaskPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedTaskID, parts[5], "TaskID not properly escaped in URL path")

			w.WriteHeader(http.StatusCreated)
			resp := p42.Task{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	model := p42.ModelTypeCodexMini
	_, err := client.CreateTask(
		context.Background(), &p42.CreateTaskRequest{
			TenantID:      tenantIDThatNeedsEscaping,
			TaskID:        taskIDThatNeedsEscaping,
			Title:         "title",
			EnvironmentID: util.Pointer("env"),
			Prompt:        "do",
			Model:         &model,
			RepoInfo:      map[string]*p42.RepoInfo{},
		},
	)
	require.NoError(t, err)
}

func TestDeleteTask(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, "/v1/tenants/abc/tasks/task", r.URL.Path)
			require.Equal(t, "1", r.Header.Get("If-Match"))

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteTask(context.Background(), &p42.DeleteTaskRequest{TenantID: "abc", TaskID: "task", Version: 1})
	require.NoError(t, err)
}

func TestDeleteTaskError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()
	err := client.DeleteTask(context.Background(), &p42.DeleteTaskRequest{TenantID: "abc", TaskID: "task", Version: 1})
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestDeleteTaskConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveTaskConflict()
	defer srv.Close()
	err := client.DeleteTask(context.Background(), &p42.DeleteTaskRequest{TenantID: "abc", TaskID: "task", Version: 1})
	verifyTaskConflict(t, err)
}

func TestDeleteTaskPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedTaskID, parts[5], "TaskID not properly escaped in URL path")

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteTask(
		context.Background(),
		&p42.DeleteTaskRequest{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping, Version: 1},
	)
	require.NoError(t, err)
}

// nolint: dupl
func TestGetTask(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/tasks/task", r.URL.Path)
			require.Empty(t, r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.Task{TenantID: "abc", TaskID: "task"}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	task, err := client.GetTask(context.Background(), &p42.GetTaskRequest{TenantID: "abc", TaskID: "task"})
	require.NoError(t, err)
	require.Equal(t, "task", task.TaskID)
}

func TestGetTaskIncludeDeleted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/tasks/task", r.URL.Path)
			require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.Task{TenantID: "abc", TaskID: "task"}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	includeDeleted := true
	_, err := client.GetTask(
		context.Background(),
		&p42.GetTaskRequest{TenantID: "abc", TaskID: "task", IncludeDeleted: &includeDeleted},
	)
	require.NoError(t, err)
}

func TestGetTaskError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(
				p42.Error{
					ResponseCode: http.StatusNotFound,
					Message:      "nope",
					ErrorType:    "NotFound",
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetTask(context.Background(), &p42.GetTaskRequest{TenantID: "abc", TaskID: "task"})
	require.Error(t, err)
}

// nolint: dupl
func TestGetTaskPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedTaskID, parts[5], "TaskID not properly escaped in URL path")

			w.WriteHeader(http.StatusOK)
			resp := p42.Task{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetTask(
		context.Background(),
		&p42.GetTaskRequest{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping},
	)
	require.NoError(t, err)
}

func TestGetWorkstreamTask(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/workstreams/ws/tasks/task", r.URL.Path)
			require.Empty(t, r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.Task{TenantID: "abc", TaskID: "task", WorkstreamID: util.Pointer("ws")}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetWorkstreamTask(
		context.Background(),
		&p42.GetWorkstreamTaskRequest{TenantID: "abc", WorkstreamID: "ws", TaskID: "task"},
	)
	require.NoError(t, err)
}

func TestGetWorkstreamTaskIncludeDeleted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/workstreams/ws/tasks/task", r.URL.Path)
			require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.Task{TenantID: "abc", TaskID: "task", WorkstreamID: util.Pointer("ws")}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	includeDeleted := true
	_, err := client.GetWorkstreamTask(
		context.Background(),
		&p42.GetWorkstreamTaskRequest{
			TenantID:       "abc",
			WorkstreamID:   "ws",
			TaskID:         "task",
			IncludeDeleted: &includeDeleted,
		},
	)
	require.NoError(t, err)
}

func TestGetWorkstreamTaskError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(
				p42.Error{
					ResponseCode: http.StatusNotFound,
					Message:      "nope",
					ErrorType:    "NotFound",
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetWorkstreamTask(
		context.Background(),
		&p42.GetWorkstreamTaskRequest{TenantID: "abc", WorkstreamID: "ws", TaskID: "task"},
	)
	require.Error(t, err)
}

func TestGetWorkstreamTaskPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedWorkstreamID, parts[5], "WorkstreamID not properly escaped in URL path")
			require.Equal(t, escapedTaskID, parts[7], "TaskID not properly escaped in URL path")

			w.WriteHeader(http.StatusOK)
			resp := p42.Task{
				TenantID:     tenantIDThatNeedsEscaping,
				TaskID:       taskIDThatNeedsEscaping,
				WorkstreamID: util.Pointer(workstreamIDThatNeedsEscaping),
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetWorkstreamTask(
		context.Background(),
		&p42.GetWorkstreamTaskRequest{
			TenantID:     tenantIDThatNeedsEscaping,
			WorkstreamID: workstreamIDThatNeedsEscaping,
			TaskID:       taskIDThatNeedsEscaping,
		},
	)
	require.NoError(t, err)
}

// nolint: dupl
func TestUpdateTask(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, "/v1/tenants/abc/tasks/task", r.URL.Path)
			require.Equal(t, "1", r.Header.Get("If-Match"))

			var reqBody p42.UpdateTaskRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.NotNil(t, reqBody.Title)
			require.Equal(t, taskTitle, *reqBody.Title)

			w.WriteHeader(http.StatusOK)
			resp := p42.Task{TenantID: "abc", TaskID: "task"}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	task, err := client.UpdateTask(
		context.Background(),
		&p42.UpdateTaskRequest{TenantID: "abc", TaskID: "task", Version: 1, Title: util.Pointer(taskTitle)},
	)
	require.NoError(t, err)
	require.Equal(t, "task", task.TaskID)
}

func TestUpdateTaskWithRepoStatus(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, "/v1/tenants/abc/tasks/task", r.URL.Path)
			require.Equal(t, "2", r.Header.Get("If-Match"))

			var reqBody p42.UpdateTaskRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.NotNil(t, reqBody.RepoInfo)
			repo := (*reqBody.RepoInfo)["octo/demo"]
			require.NotNil(t, repo)
			require.Equal(t, "closed", *repo.PRStatus)
			require.NotNil(t, repo.PRStatusUpdatedAt)
			require.Equal(t, "123", *repo.PRID)
			require.Equal(t, 99, *repo.PRNumber)

			w.WriteHeader(http.StatusOK)
			resp := p42.Task{TenantID: "abc", TaskID: "task"}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	status := "closed"
	prID := "123"
	prNumber := 99
	statusAt := time.Now().Add(-time.Minute)
	repoInfo := map[string]*p42.RepoInfo{
		"octo/demo": {
			FeatureBranch:     "feature",
			TargetBranch:      "main",
			PRStatus:          &status,
			PRStatusUpdatedAt: &statusAt,
			PRID:              &prID,
			PRNumber:          &prNumber,
		},
	}
	_, err := client.UpdateTask(
		context.Background(),
		&p42.UpdateTaskRequest{TenantID: "abc", TaskID: "task", Version: 2, RepoInfo: &repoInfo},
	)
	require.NoError(t, err)
}

func TestUpdateTaskError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()
	_, err := client.UpdateTask(
		context.Background(),
		&p42.UpdateTaskRequest{TenantID: "abc", TaskID: "task", Version: 1, Title: util.Pointer(taskTitle)},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestUpdateTaskConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveTaskConflict()
	defer srv.Close()
	_, err := client.UpdateTask(
		context.Background(),
		&p42.UpdateTaskRequest{TenantID: "abc", TaskID: "task", Version: 1, Title: util.Pointer(taskTitle)},
	)
	verifyTaskConflict(t, err)
}

// nolint: dupl
func TestUpdateTaskPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedTaskID, parts[5], "TaskID not properly escaped in URL path")

			w.WriteHeader(http.StatusOK)
			resp := p42.Task{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.UpdateTask(
		context.Background(),
		&p42.UpdateTaskRequest{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping, Version: 1},
	)
	require.NoError(t, err)
}

func TestCreateTurn(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "/v1/tenants/abc/tasks/task1/turns/2", r.URL.Path)
			require.Equal(t, "1", r.Header.Get("If-Match"))

			var reqBody p42.CreateTurnRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.Equal(t, "prompt", reqBody.Prompt)

			w.WriteHeader(http.StatusCreated)
			resp := p42.Turn{TenantID: "abc", TaskID: "task1", TurnIndex: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	turn, err := client.CreateTurn(
		context.Background(),
		&p42.CreateTurnRequest{TenantID: "abc", TaskID: "task1", TurnIndex: 2, Prompt: "prompt", TaskVersion: 1},
	)
	require.NoError(t, err)
	require.Equal(t, 1, turn.TurnIndex)
}

func TestCreateTurnError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()
	_, err := client.CreateTurn(
		context.Background(),
		&p42.CreateTurnRequest{TenantID: "abc", TaskID: "task1", TurnIndex: 2, Prompt: "prompt", TaskVersion: 1},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func serveTurnConflict() (*httptest.Server, *p42.Client) {
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(
				p42.ConflictError{
					ResponseCode: http.StatusConflict,
					Message:      "exists",
					ErrorType:    "Conflict",
					Current:      &p42.Turn{TurnIndex: 1, TaskID: "task1"},
				},
			)
		},
	)

	srv := httptest.NewServer(handler)

	client := p42.NewClient(srv.URL)
	return srv, client
}

func verifyTurnConflict(t *testing.T, err error) {
	var clientErr *p42.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.Equal(t, "exists", clientErr.Message)
	require.Equal(t, "Conflict", clientErr.ErrorType)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, p42.ObjectTypeTurn, clientErr.Current.ObjectType())
	turn, ok := clientErr.Current.(*p42.Turn)
	require.True(t, ok, "Expected Current to be of type *p42.Turn")
	require.Equal(t, p42.Turn{TurnIndex: 1, TaskID: "task1"}, *turn)
}

func TestCreateTurnConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveTurnConflict()
	defer srv.Close()

	_, err := client.CreateTurn(
		context.Background(),
		&p42.CreateTurnRequest{TenantID: "abc", TaskID: "task1", TurnIndex: 2, Prompt: "prompt", TaskVersion: 1},
	)
	verifyTurnConflict(t, err)
}

func TestCreateTurnPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedTaskID, parts[5], "TaskID not properly escaped in URL path")

			w.WriteHeader(http.StatusCreated)
			resp := p42.Turn{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping, TurnIndex: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.CreateTurn(
		context.Background(),
		&p42.CreateTurnRequest{
			TenantID:    tenantIDThatNeedsEscaping,
			TaskID:      taskIDThatNeedsEscaping,
			TurnIndex:   2,
			Prompt:      "prompt",
			TaskVersion: 1,
		},
	)
	require.NoError(t, err)
}

func TestGetTurn(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/tasks/task/turns/1", r.URL.Path)
			require.Empty(t, r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.Turn{TenantID: "abc", TaskID: "task", TurnIndex: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	turn, err := client.GetTurn(
		context.Background(),
		&p42.GetTurnRequest{TenantID: "abc", TaskID: "task", TurnIndex: 1},
	)
	require.NoError(t, err)
	require.Equal(t, 1, turn.TurnIndex)
}

func TestGetTurnIncludeDeleted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/tasks/task/turns/1", r.URL.Path)
			require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.Turn{TenantID: "abc", TaskID: "task", TurnIndex: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	includeDeleted := true
	_, err := client.GetTurn(
		context.Background(),
		&p42.GetTurnRequest{TenantID: "abc", TaskID: "task", TurnIndex: 1, IncludeDeleted: &includeDeleted},
	)
	require.NoError(t, err)
}

func TestGetTurnError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(
				p42.Error{
					ResponseCode: http.StatusNotFound,
					Message:      "nope",
					ErrorType:    "NotFound",
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetTurn(context.Background(), &p42.GetTurnRequest{TenantID: "abc", TaskID: "task", TurnIndex: 1})
	require.Error(t, err)
}

func TestGetTurnPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedTaskID, parts[5], "TaskID not properly escaped in URL path")

			w.WriteHeader(http.StatusOK)
			resp := p42.Turn{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping, TurnIndex: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetTurn(
		context.Background(),
		&p42.GetTurnRequest{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping, TurnIndex: 1},
	)
	require.NoError(t, err)
}

func TestGetLastTurn(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/tasks/task/turns/last", r.URL.Path)
			require.Empty(t, r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.Turn{TenantID: "abc", TaskID: "task", TurnIndex: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	turn, err := client.GetLastTurn(context.Background(), &p42.GetLastTurnRequest{TenantID: "abc", TaskID: "task"})
	require.NoError(t, err)
	require.Equal(t, 1, turn.TurnIndex)
}

func TestGetLastTurnIncludeDeleted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/tasks/task/turns/last", r.URL.Path)
			require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.Turn{TenantID: "abc", TaskID: "task", TurnIndex: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	includeDeleted := true
	_, err := client.GetLastTurn(
		context.Background(),
		&p42.GetLastTurnRequest{TenantID: "abc", TaskID: "task", IncludeDeleted: &includeDeleted},
	)
	require.NoError(t, err)
}

func TestGetLastTurnError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(
				p42.Error{
					ResponseCode: http.StatusNotFound,
					Message:      "nope",
					ErrorType:    "NotFound",
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetLastTurn(context.Background(), &p42.GetLastTurnRequest{TenantID: "abc", TaskID: "task"})
	require.Error(t, err)
}

func TestGetLastTurnPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedTaskID, parts[5], "TaskID not properly escaped in URL path")

			w.WriteHeader(http.StatusOK)
			resp := p42.Turn{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping, TurnIndex: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetLastTurn(
		context.Background(),
		&p42.GetLastTurnRequest{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping},
	)
	require.NoError(t, err)
}

func TestGetLastTurnLog(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/tasks/task/turns/1/logs/last", r.URL.Path)
			require.Empty(t, r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.LastTurnLog{Index: 2, Timestamp: time.Unix(0, 0), Message: "msg"}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	log, err := client.GetLastTurnLog(
		context.Background(), &p42.GetLastTurnLogRequest{
			TenantID:  "abc",
			TaskID:    "task",
			TurnIndex: 1,
		},
	)
	require.NoError(t, err)
	require.Equal(t, 2, log.Index)
}

func TestGetLastTurnLogIncludeDeleted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(p42.LastTurnLog{})
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	includeDeleted := true
	_, err := client.GetLastTurnLog(
		context.Background(), &p42.GetLastTurnLogRequest{
			TenantID:       "abc",
			TaskID:         "task",
			TurnIndex:      1,
			IncludeDeleted: &includeDeleted,
		},
	)
	require.NoError(t, err)
}

func TestGetLastTurnLogPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 10, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3])
			require.Equal(t, escapedTaskID, parts[5])
			require.Equal(t, "1", parts[7])

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(p42.LastTurnLog{})
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetLastTurnLog(
		context.Background(), &p42.GetLastTurnLogRequest{
			TenantID:  tenantIDThatNeedsEscaping,
			TaskID:    taskIDThatNeedsEscaping,
			TurnIndex: 1,
		},
	)
	require.NoError(t, err)
}

func TestGetLastTurnLogError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.GetLastTurnLog(
		context.Background(), &p42.GetLastTurnLogRequest{
			TenantID:  "abc",
			TaskID:    "task",
			TurnIndex: 1,
		},
	)
	require.Error(t, err)
}

func TestUploadTurnLogs(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/v1/tenants/abc/tasks/task/turns/0/logs", r.URL.Path)
			require.Equal(t, "1", r.Header.Get("If-Match"))

			var reqBody p42.UploadTurnLogsRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.Equal(t, 0, reqBody.Index)
			require.Len(t, reqBody.Logs, 1)
			require.Equal(t, "msg", reqBody.Logs[0].Message)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(p42.UploadTurnLogsResponse{Version: 2})
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	logs := []p42.TurnLog{{Timestamp: time.Unix(0, 0), Message: "msg"}}
	resp, err := client.UploadTurnLogs(
		context.Background(), &p42.UploadTurnLogsRequest{
			TenantID:  "abc",
			TaskID:    "task",
			TurnIndex: 0,
			Version:   1,
			Index:     0,
			Logs:      logs,
		},
	)
	require.NoError(t, err)
	require.Equal(t, 2, resp.Version)
}

func TestUploadTurnLogsError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.UploadTurnLogs(
		context.Background(), &p42.UploadTurnLogsRequest{
			TenantID:  "abc",
			TaskID:    "task",
			TurnIndex: 0,
			Version:   1,
			Index:     0,
		},
	)
	require.Error(t, err)
}

func TestUploadTurnLogsConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveTurnConflict()
	defer srv.Close()

	_, err := client.UploadTurnLogs(
		context.Background(), &p42.UploadTurnLogsRequest{
			TenantID:  "abc",
			TaskID:    "task",
			TurnIndex: 0,
			Version:   1,
			Index:     0,
		},
	)
	var clientErr *p42.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.Equal(t, "exists", clientErr.Message)
	require.Equal(t, "Conflict", clientErr.ErrorType)
}

func TestUploadTurnLogsPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 9, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3])
			require.Equal(t, escapedTaskID, parts[5])
			require.Equal(t, "0", parts[7])

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(p42.UploadTurnLogsResponse{Version: 1})
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.UploadTurnLogs(
		context.Background(), &p42.UploadTurnLogsRequest{
			TenantID:  tenantIDThatNeedsEscaping,
			TaskID:    taskIDThatNeedsEscaping,
			TurnIndex: 0,
			Version:   1,
			Index:     0,
		},
	)
	require.NoError(t, err)
}

// nolint: dupl
func TestListTasks(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/tasks", r.URL.Path)
			require.Equal(t, "123", r.URL.Query().Get("maxResults"))
			require.Equal(t, tokenID, r.URL.Query().Get("token"))
			require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.ListTasksResponse{Tasks: []p42.Task{{TaskID: "task"}}}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	maxResults := 123
	includeDeleted := true
	resp, err := client.ListTasks(
		context.Background(), &p42.ListTasksRequest{
			TenantID:       "abc",
			MaxResults:     &maxResults,
			Token:          util.Pointer(tokenID),
			IncludeDeleted: &includeDeleted,
		},
	)
	require.NoError(t, err)
	require.Len(t, resp.Tasks, 1)
	require.Equal(t, "task", resp.Tasks[0].TaskID)
}

func TestListTasksError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListTasks(context.Background(), &p42.ListTasksRequest{TenantID: "abc"})
	require.Error(t, err)
}

func TestListTasksPathEscaping(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3])

			w.WriteHeader(http.StatusOK)
			resp := p42.ListTasksResponse{}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.ListTasks(context.Background(), &p42.ListTasksRequest{TenantID: tenantIDThatNeedsEscaping})
	require.NoError(t, err)
}

func TestListWorkstreamTasks(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/workstreams/ws/tasks", r.URL.Path)
			require.Equal(t, "100", r.URL.Query().Get("maxResults"))
			require.Equal(t, tokenID, r.URL.Query().Get("token"))
			require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.List[p42.Task]{Items: []p42.Task{{TaskID: "task"}}}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	maxResults := 100
	includeDeleted := true
	resp, err := client.ListWorkstreamTasks(
		context.Background(), &p42.ListWorkstreamTasksRequest{
			TenantID:       "abc",
			WorkstreamID:   "ws",
			MaxResults:     &maxResults,
			Token:          util.Pointer(tokenID),
			IncludeDeleted: &includeDeleted,
		},
	)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	require.Equal(t, "task", resp.Items[0].TaskID)
}

func TestListWorkstreamTasksError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListWorkstreamTasks(
		context.Background(),
		&p42.ListWorkstreamTasksRequest{TenantID: "abc", WorkstreamID: "ws"},
	)
	require.Error(t, err)
}

func TestListWorkstreamTasksPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 7, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
			require.Equal(t, escapedWorkstreamID, parts[5], "WorkstreamID not properly escaped in URL path")

			w.WriteHeader(http.StatusOK)
			resp := p42.List[p42.Task]{}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.ListWorkstreamTasks(
		context.Background(), &p42.ListWorkstreamTasksRequest{
			TenantID:     tenantIDThatNeedsEscaping,
			WorkstreamID: workstreamIDThatNeedsEscaping,
		},
	)
	require.NoError(t, err)
}

func TestListTurns(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/tasks/task1/turns", r.URL.Path)
			require.Equal(t, "20", r.URL.Query().Get("maxResults"))
			require.Equal(t, "tok", r.URL.Query().Get("token"))
			require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.ListTurnsResponse{Turns: []p42.Turn{{TurnIndex: 1}}}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	maxResults := 20
	tok := "tok"
	includeDeleted := true
	resp, err := client.ListTurns(
		context.Background(), &p42.ListTurnsRequest{
			TenantID:       "abc",
			TaskID:         "task1",
			MaxResults:     &maxResults,
			Token:          &tok,
			IncludeDeleted: &includeDeleted,
		},
	)
	require.NoError(t, err)
	require.Len(t, resp.Turns, 1)
	require.Equal(t, 1, resp.Turns[0].TurnIndex)
}

func TestListTurnsError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListTurns(context.Background(), &p42.ListTurnsRequest{TenantID: "abc", TaskID: "task"})
	require.Error(t, err)
}

func TestListTurnsPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 7, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3])
			require.Equal(t, escapedTaskID, parts[5])

			w.WriteHeader(http.StatusOK)
			resp := p42.ListTurnsResponse{}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.ListTurns(
		context.Background(), &p42.ListTurnsRequest{
			TenantID: tenantIDThatNeedsEscaping,
			TaskID:   taskIDThatNeedsEscaping,
		},
	)
	require.NoError(t, err)
}

func TestUpdateTurn(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, "/v1/tenants/abc/tasks/task1/turns/1", r.URL.Path)
			require.Equal(t, "1", r.Header.Get("If-Match"))

			var reqBody p42.UpdateTurnRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.NotNil(t, reqBody.Status)
			require.Equal(t, turnStatus, *reqBody.Status)

			w.WriteHeader(http.StatusOK)
			resp := p42.Turn{TenantID: "abc", TaskID: "task1", TurnIndex: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	status := turnStatus
	turn, err := client.UpdateTurn(
		context.Background(), &p42.UpdateTurnRequest{
			TenantID:  "abc",
			TaskID:    "task1",
			TurnIndex: 1,
			Version:   1,
			Status:    &status,
		},
	)
	require.NoError(t, err)
	require.Equal(t, 1, turn.TurnIndex)
}

func TestUpdateTurnError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.UpdateTurn(
		context.Background(), &p42.UpdateTurnRequest{
			TenantID:  "abc",
			TaskID:    "task1",
			TurnIndex: 1,
			Version:   1,
			Status:    util.Pointer(turnStatus),
		},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestUpdateTurnConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveTurnConflict()
	defer srv.Close()

	_, err := client.UpdateTurn(
		context.Background(), &p42.UpdateTurnRequest{
			TenantID:  "abc",
			TaskID:    "task1",
			TurnIndex: 1,
			Version:   1,
			Status:    util.Pointer(turnStatus),
		},
	)
	verifyTurnConflict(t, err)
}

func TestUpdateTurnPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3])
			require.Equal(t, escapedTaskID, parts[5])
			require.Equal(t, "1", parts[7])

			w.WriteHeader(http.StatusOK)
			resp := p42.Turn{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping, TurnIndex: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.UpdateTurn(
		context.Background(), &p42.UpdateTurnRequest{
			TenantID:  tenantIDThatNeedsEscaping,
			TaskID:    taskIDThatNeedsEscaping,
			TurnIndex: 1,
			Version:   1,
			Status:    util.Pointer(turnStatus),
		},
	)
	require.NoError(t, err)
}

func TestStreamTurnLogs(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/tasks/task/turns/0/logs", r.URL.Path)
			require.Equal(t, "text/event-stream", r.Header.Get("Accept"))
			require.Empty(t, r.Header.Get("Last-Event-ID"))

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("data: {}\n\n"))
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	body, err := client.StreamTurnLogs(
		context.Background(), &p42.StreamTurnLogsRequest{
			TenantID:  "abc",
			TaskID:    "task",
			TurnIndex: 0,
		},
	)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, "data: {}\n\n", string(data))
	_ = body.Close()
}

func TestStreamTurnLogsIncludeDeleted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))
			w.WriteHeader(http.StatusOK)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	includeDeleted := true
	body, err := client.StreamTurnLogs(
		context.Background(), &p42.StreamTurnLogsRequest{
			TenantID:       "abc",
			TaskID:         "task",
			TurnIndex:      0,
			IncludeDeleted: &includeDeleted,
		},
	)
	require.NoError(t, err)
	_ = body.Close()
}

func TestStreamTurnLogsLastEventID(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "5", r.Header.Get("Last-Event-ID"))
			w.WriteHeader(http.StatusOK)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	id := 5
	body, err := client.StreamTurnLogs(
		context.Background(), &p42.StreamTurnLogsRequest{
			TenantID:    "abc",
			TaskID:      "task",
			TurnIndex:   0,
			LastEventID: &id,
		},
	)
	require.NoError(t, err)
	_ = body.Close()
}

func TestStreamTurnLogsNoContent(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	body, err := client.StreamTurnLogs(
		context.Background(), &p42.StreamTurnLogsRequest{
			TenantID:  "abc",
			TaskID:    "task",
			TurnIndex: 0,
		},
	)
	require.NoError(t, err)
	require.Nil(t, body)
}

func TestStreamTurnLogsError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.StreamTurnLogs(
		context.Background(), &p42.StreamTurnLogsRequest{
			TenantID:  "abc",
			TaskID:    "task",
			TurnIndex: 0,
		},
	)
	require.Error(t, err)
}

func TestStreamTurnLogsPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 9, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3])
			require.Equal(t, escapedTaskID, parts[5])
			require.Equal(t, "0", parts[7])

			w.WriteHeader(http.StatusOK)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	body, err := client.StreamTurnLogs(
		context.Background(), &p42.StreamTurnLogsRequest{
			TenantID:  tenantIDThatNeedsEscaping,
			TaskID:    taskIDThatNeedsEscaping,
			TurnIndex: 0,
		},
	)
	require.NoError(t, err)
	_ = body.Close()
}

func TestCreateGithubConnection(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	tokenExpiry := now.Add(time.Hour)
	stateExpiry := now.Add(2 * time.Hour)

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "/v1/tenants/abc/github-connections/conn", r.URL.Path)
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			require.Equal(t, "application/json", r.Header.Get("Accept"))

			var reqBody p42.CreateGithubConnectionRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.False(t, reqBody.Private)
			require.Nil(t, reqBody.RunnerID)
			require.NotNil(t, reqBody.GithubUserLogin)
			require.Equal(t, githubUserLogin, *reqBody.GithubUserLogin)
			require.NotNil(t, reqBody.GithubUserID)
			require.Equal(t, 123, *reqBody.GithubUserID)

			w.WriteHeader(http.StatusCreated)
			resp := p42.GithubConnection{
				TenantID:        "abc",
				ConnectionID:    "conn",
				Private:         false,
				GithubUserLogin: util.Pointer(githubUserLogin),
				GithubUserID:    util.Pointer(123),
				OAuthToken:      util.Pointer("token"),
				RefreshToken:    util.Pointer("refresh"),
				TokenExpiry:     &tokenExpiry,
				State:           util.Pointer("pending"),
				StateExpiry:     &stateExpiry,
				CreatedAt:       now,
				UpdatedAt:       now,
				Version:         1,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	login := githubUserLogin
	userID := 123

	resp, err := client.CreateGithubConnection(
		context.Background(), &p42.CreateGithubConnectionRequest{
			TenantID:        "abc",
			ConnectionID:    "conn",
			GithubUserLogin: &login,
			GithubUserID:    &userID,
		},
	)
	require.NoError(t, err)
	require.Equal(t, "conn", resp.ConnectionID)
	require.NotNil(t, resp.GithubUserLogin)
	require.Equal(t, githubUserLogin, *resp.GithubUserLogin)
	require.NotNil(t, resp.TokenExpiry)
	require.Equal(t, tokenExpiry, *resp.TokenExpiry)
	require.Equal(t, 1, resp.Version)
}

func TestCreateGithubConnectionError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	login := githubUserLogin
	userID := 123
	_, err := client.CreateGithubConnection(
		context.Background(), &p42.CreateGithubConnectionRequest{
			TenantID:        "abc",
			ConnectionID:    "conn",
			GithubUserLogin: &login,
			GithubUserID:    &userID,
		},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
}

func TestCreateGithubConnectionConflictError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(
				p42.ConflictError{
					ResponseCode: http.StatusConflict,
					Message:      "exists",
					ErrorType:    "Conflict",
					Current:      &p42.GithubConnection{ConnectionID: "conn"},
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	login := githubUserLogin
	userID := 123

	_, err := client.CreateGithubConnection(
		context.Background(), &p42.CreateGithubConnectionRequest{
			TenantID:        "abc",
			ConnectionID:    "conn",
			GithubUserLogin: &login,
			GithubUserID:    &userID,
		},
	)
	var clientErr *p42.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, p42.ObjectTypeGithubConnection, clientErr.Current.ObjectType())
	connection, ok := clientErr.Current.(*p42.GithubConnection)
	require.True(t, ok, "Expected Current to be of type *p42.GithubConnection")
	require.Equal(t, "conn", connection.ConnectionID)
}

func TestCreateGithubConnectionPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3])
			require.Equal(t, escapedGithubConnectionID, parts[5])

			w.WriteHeader(http.StatusCreated)
			now := time.Now().UTC()
			resp := p42.GithubConnection{
				TenantID:     tenantIDThatNeedsEscaping,
				ConnectionID: githubConnectionIDThatNeedsEscaping,
				CreatedAt:    now,
				UpdatedAt:    now,
				Version:      1,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.CreateGithubConnection(
		context.Background(), &p42.CreateGithubConnectionRequest{
			TenantID:        tenantIDThatNeedsEscaping,
			ConnectionID:    githubConnectionIDThatNeedsEscaping,
			GithubUserLogin: util.Pointer(githubUserLogin),
			GithubUserID:    util.Pointer(123),
		},
	)
	require.NoError(t, err)
}

func TestCreateGithubConnectionPrivateValidation(t *testing.T) {
	t.Parallel()

	client := p42.NewClient("https://api.example.com")

	_, err := client.CreateGithubConnection(
		context.Background(), &p42.CreateGithubConnectionRequest{
			TenantID:     "abc",
			ConnectionID: "conn",
			Private:      true,
		},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "runner id is required when private is true")

	_, err = client.CreateGithubConnection(
		context.Background(), &p42.CreateGithubConnectionRequest{
			TenantID:        "abc",
			ConnectionID:    "conn",
			Private:         true,
			RunnerID:        util.Pointer("runner"),
			GithubUserLogin: util.Pointer(githubUserLogin),
		},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "github user login must be nil when private is true")

	_, err = client.CreateGithubConnection(
		context.Background(), &p42.CreateGithubConnectionRequest{
			TenantID:     "abc",
			ConnectionID: "conn",
			Private:      true,
			RunnerID:     util.Pointer("runner"),
			GithubUserID: util.Pointer(123),
		},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "github user id must be nil when private is true")
}

func TestListGithubConnections(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/github-connections", r.URL.Path)
			require.Equal(t, "application/json", r.Header.Get("Accept"))
			require.Equal(t, "10", r.URL.Query().Get("maxResults"))
			require.Equal(t, tokenID, r.URL.Query().Get("token"))

			w.WriteHeader(http.StatusOK)
			resp := p42.List[p42.GithubConnection]{
				Items: []p42.GithubConnection{{ConnectionID: "conn-1"}},
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	maxResults := 10
	resp, err := client.ListGithubConnections(
		context.Background(), &p42.ListGithubConnectionsRequest{
			TenantID:   "abc",
			MaxResults: &maxResults,
			Token:      util.Pointer(tokenID),
		},
	)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	require.Equal(t, "conn-1", resp.Items[0].ConnectionID)
}

func TestListGithubConnectionsError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListGithubConnections(context.Background(), &p42.ListGithubConnectionsRequest{TenantID: "abc"})
	require.Error(t, err)
}

func TestListGithubConnectionsPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3])
			require.Equal(t, "github-connections", parts[4])

			w.WriteHeader(http.StatusOK)
			resp := p42.List[p42.GithubConnection]{Items: []p42.GithubConnection{}}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.ListGithubConnections(
		context.Background(), &p42.ListGithubConnectionsRequest{
			TenantID: tenantIDThatNeedsEscaping,
		},
	)
	require.NoError(t, err)
}

func TestDeleteGithubConnection(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, "/v1/tenants/abc/github-connections/conn", r.URL.Path)
			require.Equal(t, "application/json", r.Header.Get("Accept"))
			require.Equal(t, "1", r.Header.Get("If-Match"))

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteGithubConnection(
		context.Background(), &p42.DeleteGithubConnectionRequest{
			TenantID:     "abc",
			ConnectionID: "conn",
			Version:      1,
		},
	)
	require.NoError(t, err)
}

func TestDeleteGithubConnectionError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	err := client.DeleteGithubConnection(
		context.Background(), &p42.DeleteGithubConnectionRequest{
			TenantID:     "abc",
			ConnectionID: "conn",
			Version:      1,
		},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestDeleteGithubConnectionConflictError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(
				p42.ConflictError{
					ResponseCode: http.StatusConflict,
					Message:      "exists",
					ErrorType:    "Conflict",
					Current:      &p42.GithubConnection{ConnectionID: "conn"},
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteGithubConnection(
		context.Background(), &p42.DeleteGithubConnectionRequest{
			TenantID:     "abc",
			ConnectionID: "conn",
			Version:      1,
		},
	)
	var conflictErr *p42.ConflictError
	require.ErrorAs(t, err, &conflictErr)
	require.Equal(t, http.StatusConflict, conflictErr.ResponseCode)
	require.NotNil(t, conflictErr.Current)
	require.Equal(t, p42.ObjectTypeGithubConnection, conflictErr.Current.ObjectType())
	connection, ok := conflictErr.Current.(*p42.GithubConnection)
	require.True(t, ok, "Expected Current to be of type *p42.GithubConnection")
	require.Equal(t, "conn", connection.ConnectionID)
}

func TestDeleteGithubConnectionPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3])
			require.Equal(t, escapedGithubConnectionID, parts[5])

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteGithubConnection(
		context.Background(), &p42.DeleteGithubConnectionRequest{
			TenantID:     tenantIDThatNeedsEscaping,
			ConnectionID: githubConnectionIDThatNeedsEscaping,
			Version:      1,
		},
	)
	require.NoError(t, err)
}

func TestGetGithubConnection(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	tokenExpiry := now.Add(time.Hour)
	stateExpiry := now.Add(2 * time.Hour)

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/github-connections/conn", r.URL.Path)
			require.Equal(t, "application/json", r.Header.Get("Accept"))

			w.WriteHeader(http.StatusOK)
			resp := p42.GithubConnection{
				TenantID:        "abc",
				ConnectionID:    "conn",
				Private:         false,
				GithubUserLogin: util.Pointer(githubUserLogin),
				GithubUserID:    util.Pointer(123),
				OAuthToken:      util.Pointer("token"),
				RefreshToken:    util.Pointer("refresh"),
				TokenExpiry:     &tokenExpiry,
				State:           util.Pointer("pending"),
				StateExpiry:     &stateExpiry,
				CreatedAt:       now,
				UpdatedAt:       now,
				Version:         2,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	resp, err := client.GetGithubConnection(
		context.Background(), &p42.GetGithubConnectionRequest{
			TenantID:     "abc",
			ConnectionID: "conn",
		},
	)
	require.NoError(t, err)
	require.Equal(t, "conn", resp.ConnectionID)
	require.NotNil(t, resp.GithubUserLogin)
	require.Equal(t, githubUserLogin, *resp.GithubUserLogin)
	require.Equal(t, 2, resp.Version)
	require.NotNil(t, resp.TokenExpiry)
	require.Equal(t, tokenExpiry, *resp.TokenExpiry)
}

func TestGetGithubConnectionError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(
				p42.Error{
					ResponseCode: http.StatusNotFound,
					Message:      "nope",
					ErrorType:    "NotFound",
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetGithubConnection(
		context.Background(), &p42.GetGithubConnectionRequest{
			TenantID:     "abc",
			ConnectionID: "conn",
		},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusNotFound, clientErr.ResponseCode)
}

func TestGetGithubConnectionPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3])
			require.Equal(t, escapedGithubConnectionID, parts[5])

			w.WriteHeader(http.StatusOK)
			now := time.Now().UTC()
			resp := p42.GithubConnection{
				TenantID:     tenantIDThatNeedsEscaping,
				ConnectionID: githubConnectionIDThatNeedsEscaping,
				CreatedAt:    now,
				UpdatedAt:    now,
				Version:      1,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetGithubConnection(
		context.Background(), &p42.GetGithubConnectionRequest{
			TenantID:     tenantIDThatNeedsEscaping,
			ConnectionID: githubConnectionIDThatNeedsEscaping,
		},
	)
	require.NoError(t, err)
}

func TestUpdateGithubConnection(t *testing.T) {
	t.Parallel()

	stateExpiry := time.Now().UTC().Add(time.Hour).Truncate(time.Second)

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, "/v1/tenants/abc/github-connections/conn", r.URL.Path)
			require.Equal(t, "2", r.Header.Get("If-Match"))

			var reqBody p42.UpdateGithubConnectionRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.NotNil(t, reqBody.Private)
			require.True(t, *reqBody.Private)
			require.NotNil(t, reqBody.RunnerID)
			require.Equal(t, "runner", *reqBody.RunnerID)
			require.NotNil(t, reqBody.OAuthToken)
			require.Equal(t, "oauth", *reqBody.OAuthToken)
			require.NotNil(t, reqBody.RefreshToken)
			require.Equal(t, "refresh", *reqBody.RefreshToken)
			require.NotNil(t, reqBody.State)
			require.Equal(t, "pending", *reqBody.State)
			require.NotNil(t, reqBody.StateExpiry)
			require.WithinDuration(t, stateExpiry, *reqBody.StateExpiry, time.Second)

			w.WriteHeader(http.StatusOK)
			resp := p42.GithubConnection{TenantID: "abc", ConnectionID: "conn"}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	resp, err := client.UpdateGithubConnection(
		context.Background(), &p42.UpdateGithubConnectionRequest{
			TenantID:     "abc",
			ConnectionID: "conn",
			Version:      2,
			Private:      util.Pointer(true),
			RunnerID:     util.Pointer("runner"),
			OAuthToken:   util.Pointer("oauth"),
			RefreshToken: util.Pointer("refresh"),
			State:        util.Pointer("pending"),
			StateExpiry:  util.Pointer(stateExpiry),
		},
	)
	require.NoError(t, err)
	require.Equal(t, "conn", resp.ConnectionID)
}

func TestUpdateGithubConnectionError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.UpdateGithubConnection(
		context.Background(),
		&p42.UpdateGithubConnectionRequest{TenantID: "abc", ConnectionID: "conn", Version: 1},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestUpdateGithubConnectionConflictError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(
				p42.ConflictError{
					ResponseCode: http.StatusConflict,
					Message:      "exists",
					ErrorType:    "Conflict",
					Current:      &p42.GithubConnection{ConnectionID: "conn"},
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.UpdateGithubConnection(
		context.Background(),
		&p42.UpdateGithubConnectionRequest{TenantID: "abc", ConnectionID: "conn", Version: 1},
	)
	var clientErr *p42.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, p42.ObjectTypeGithubConnection, clientErr.Current.ObjectType())
	connection, ok := clientErr.Current.(*p42.GithubConnection)
	require.True(t, ok, "Expected Current to be of type *p42.GithubConnection")
	require.Equal(t, "conn", connection.ConnectionID)
}

func TestUpdateGithubConnectionPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3])
			require.Equal(t, escapedGithubConnectionID, parts[5])

			w.WriteHeader(http.StatusOK)
			resp := p42.GithubConnection{
				TenantID:     tenantIDThatNeedsEscaping,
				ConnectionID: githubConnectionIDThatNeedsEscaping,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.UpdateGithubConnection(
		context.Background(), &p42.UpdateGithubConnectionRequest{
			TenantID:     tenantIDThatNeedsEscaping,
			ConnectionID: githubConnectionIDThatNeedsEscaping,
			Version:      1,
		},
	)
	require.NoError(t, err)
}

func TestAddGithubOrg(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "/v1/github/orgs/abc", r.URL.Path)

			var reqBody p42.AddGithubOrgRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.Equal(t, "MyOrg", reqBody.OrgName)
			require.Equal(t, 123, reqBody.ExternalOrgID)
			require.Equal(t, 456, reqBody.InstallationID)

			w.WriteHeader(http.StatusCreated)
			resp := p42.GithubOrg{OrgID: "abc", OrgName: "MyOrg", ExternalOrgID: 123, InstallationID: 456, Version: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	org, err := client.AddGithubOrg(
		context.Background(), &p42.AddGithubOrgRequest{
			OrgID:          "abc",
			OrgName:        "MyOrg",
			ExternalOrgID:  123,
			InstallationID: 456,
		},
	)
	require.NoError(t, err)
	require.Equal(t, "abc", org.OrgID)
}

func TestAddGithubOrgError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.AddGithubOrg(
		context.Background(), &p42.AddGithubOrgRequest{
			OrgID:          "abc",
			OrgName:        "MyOrg",
			ExternalOrgID:  123,
			InstallationID: 456,
		},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func TestAddGithubOrgPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedGithubOrgID, parts[4])

			w.WriteHeader(http.StatusCreated)
			resp := p42.GithubOrg{
				OrgID:          githubOrgIDThatNeedsEscaping,
				OrgName:        "name",
				ExternalOrgID:  1,
				InstallationID: 1,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.AddGithubOrg(
		context.Background(), &p42.AddGithubOrgRequest{
			OrgID:          githubOrgIDThatNeedsEscaping,
			OrgName:        "name",
			ExternalOrgID:  1,
			InstallationID: 1,
		},
	)
	require.NoError(t, err)
}

// nolint: dupl
func TestListGithubOrgs(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/github/orgs", r.URL.Path)
			require.Equal(t, "123", r.URL.Query().Get("maxResults"))
			require.Equal(t, tokenID, r.URL.Query().Get("token"))
			require.Empty(t, r.URL.Query().Get("name"))
			require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.ListGithubOrgsResponse{Orgs: []p42.GithubOrg{{OrgID: "org"}}}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	maxResults := 123
	includeDeleted := true
	resp, err := client.ListGithubOrgs(
		context.Background(),
		&p42.ListGithubOrgsRequest{
			MaxResults:     &maxResults,
			Token:          util.Pointer(tokenID),
			IncludeDeleted: &includeDeleted,
		},
	)
	require.NoError(t, err)
	require.Len(t, resp.Orgs, 1)
	require.Equal(t, "org", resp.Orgs[0].OrgID)
}

func TestListGithubOrgsWithName(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/github/orgs", r.URL.Path)
			require.Equal(t, "the-name", r.URL.Query().Get("name"))
			require.Empty(t, r.URL.Query().Get("maxResults"))
			require.Empty(t, r.URL.Query().Get("token"))
			require.Empty(t, r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.ListGithubOrgsResponse{Orgs: []p42.GithubOrg{{OrgID: "org"}}}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	name := "the-name"
	resp, err := client.ListGithubOrgs(context.Background(), &p42.ListGithubOrgsRequest{Name: &name})
	require.NoError(t, err)
	require.Len(t, resp.Orgs, 1)
	require.Equal(t, "org", resp.Orgs[0].OrgID)
}

func TestListGithubOrgsError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListGithubOrgs(context.Background(), &p42.ListGithubOrgsRequest{})
	require.Error(t, err)
}

func TestGetGithubOrg(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/github/orgs/abc", r.URL.Path)
			require.Empty(t, r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.GithubOrg{OrgID: "abc", OrgName: "name", ExternalOrgID: 1, InstallationID: 2}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	org, err := client.GetGithubOrg(context.Background(), &p42.GetGithubOrgRequest{OrgID: "abc"})
	require.NoError(t, err)
	require.Equal(t, "abc", org.OrgID)
}

func TestGetGithubOrgIncludeDeleted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/github/orgs/abc", r.URL.Path)
			require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.GithubOrg{OrgID: "abc"}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	includeDeleted := true
	_, err := client.GetGithubOrg(
		context.Background(),
		&p42.GetGithubOrgRequest{OrgID: "abc", IncludeDeleted: &includeDeleted},
	)
	require.NoError(t, err)
}

func TestGetGithubOrgError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.GetGithubOrg(context.Background(), &p42.GetGithubOrgRequest{OrgID: "abc"})
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

// nolint: dupl
func TestGetGithubOrgPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedGithubOrgID, parts[4])

			w.WriteHeader(http.StatusOK)
			resp := p42.GithubOrg{OrgID: githubOrgIDThatNeedsEscaping}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetGithubOrg(context.Background(), &p42.GetGithubOrgRequest{OrgID: githubOrgIDThatNeedsEscaping})
	require.NoError(t, err)
}

func TestUpdateGithubOrg(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, "/v1/github/orgs/abc", r.URL.Path)
			require.Equal(t, "1", r.Header.Get("If-Match"))

			var reqBody p42.UpdateGithubOrgRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.NotNil(t, reqBody.OrgName)
			require.Equal(t, "MyOrg", *reqBody.OrgName)

			w.WriteHeader(http.StatusOK)
			resp := p42.GithubOrg{OrgID: "abc", OrgName: "MyOrg", ExternalOrgID: 123, InstallationID: 456, Version: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	name := "MyOrg"
	org, err := client.UpdateGithubOrg(
		context.Background(), &p42.UpdateGithubOrgRequest{
			OrgID:   "abc",
			Version: 1,
			OrgName: &name,
		},
	)
	require.NoError(t, err)
	require.Equal(t, "abc", org.OrgID)
}

func TestUpdateGithubOrgError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.UpdateGithubOrg(context.Background(), &p42.UpdateGithubOrgRequest{OrgID: "abc", Version: 1})
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func TestUpdateGithubOrgConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveGithubOrgConflict()
	defer srv.Close()

	_, err := client.UpdateGithubOrg(context.Background(), &p42.UpdateGithubOrgRequest{OrgID: "abc", Version: 1})
	verifyGithubOrgConflict(t, err)
}

// nolint: dupl
func TestUpdateGithubOrgPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedGithubOrgID, parts[4])

			w.WriteHeader(http.StatusOK)
			resp := p42.GithubOrg{OrgID: githubOrgIDThatNeedsEscaping}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.UpdateGithubOrg(
		context.Background(), &p42.UpdateGithubOrgRequest{
			OrgID:   githubOrgIDThatNeedsEscaping,
			Version: 1,
		},
	)
	require.NoError(t, err)
}

func TestDeleteGithubOrg(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, "/v1/github/orgs/abc", r.URL.Path)
			require.Equal(t, "1", r.Header.Get("If-Match"))

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteGithubOrg(context.Background(), &p42.DeleteGithubOrgRequest{OrgID: "abc", Version: 1})
	require.NoError(t, err)
}

func TestDeleteGithubOrgError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	err := client.DeleteGithubOrg(context.Background(), &p42.DeleteGithubOrgRequest{OrgID: "abc", Version: 1})
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func TestDeleteGithubOrgConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveGithubOrgConflict()
	defer srv.Close()

	err := client.DeleteGithubOrg(context.Background(), &p42.DeleteGithubOrgRequest{OrgID: "abc", Version: 1})
	verifyGithubOrgConflict(t, err)
}

func TestDeleteGithubOrgPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedGithubOrgID, parts[4])

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteGithubOrg(
		context.Background(),
		&p42.DeleteGithubOrgRequest{OrgID: githubOrgIDThatNeedsEscaping, Version: 1},
	)
	require.NoError(t, err)
}

func TestCreateFeatureFlag(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "/v1/featureflags/flag", r.URL.Path)

			var reqBody p42.CreateFeatureFlagRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.Equal(t, "desc", reqBody.Description)
			require.Equal(t, 0.5, reqBody.DefaultPct)

			w.WriteHeader(http.StatusCreated)
			resp := p42.FeatureFlag{Name: "flag"}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	flag, err := client.CreateFeatureFlag(
		context.Background(),
		&p42.CreateFeatureFlagRequest{FlagName: "flag", Description: "desc", DefaultPct: 0.5},
	)
	require.NoError(t, err)
	require.Equal(t, "flag", flag.Name)
}

func TestCreateFeatureFlagError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.CreateFeatureFlag(
		context.Background(),
		&p42.CreateFeatureFlagRequest{FlagName: "flag", Description: "d", DefaultPct: 0.1},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func serveFeatureFlagConflict() (*httptest.Server, *p42.Client) {
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(
				p42.ConflictError{
					ResponseCode: http.StatusConflict,
					Message:      "exists",
					ErrorType:    "Conflict",
					Current:      &p42.FeatureFlag{Name: "flag"},
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	client := p42.NewClient(srv.URL)
	return srv, client
}

func verifyFeatureFlagConflict(t *testing.T, err error) {
	var clientErr *p42.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.Equal(t, "exists", clientErr.Message)
	require.Equal(t, "Conflict", clientErr.ErrorType)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, p42.ObjectTypeFeatureFlag, clientErr.Current.ObjectType())
	ff, ok := clientErr.Current.(*p42.FeatureFlag)
	require.True(t, ok, "Expected Current to be of type *p42.FeatureFlag")
	require.Equal(t, p42.FeatureFlag{Name: "flag"}, *ff)
}

func TestCreateFeatureFlagConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveFeatureFlagConflict()
	defer srv.Close()

	_, err := client.CreateFeatureFlag(
		context.Background(),
		&p42.CreateFeatureFlagRequest{FlagName: "flag", Description: "desc", DefaultPct: 0.5},
	)
	verifyFeatureFlagConflict(t, err)
}

func TestCreateFeatureFlagPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 4, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedFeatureFlagName, parts[3])

			w.WriteHeader(http.StatusCreated)
			resp := p42.FeatureFlag{Name: featureFlagNameThatNeedsEscaping}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.CreateFeatureFlag(
		context.Background(),
		&p42.CreateFeatureFlagRequest{FlagName: featureFlagNameThatNeedsEscaping, Description: "desc", DefaultPct: 0.5},
	)
	require.NoError(t, err)
}

func TestCreateFeatureFlagOverride(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "/v1/tenants/abc/featureFlagOverrides/flag", r.URL.Path)

			var reqBody p42.CreateFeatureFlagOverrideRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.True(t, reqBody.Enabled)

			w.WriteHeader(http.StatusCreated)
			resp := p42.FeatureFlagOverride{FlagName: "flag", TenantID: "abc", Enabled: true}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	resp, err := client.CreateFeatureFlagOverride(
		context.Background(),
		&p42.CreateFeatureFlagOverrideRequest{TenantID: "abc", FlagName: "flag", Enabled: true},
	)
	require.NoError(t, err)
	require.Equal(t, "flag", resp.FlagName)
	require.True(t, resp.Enabled)
}

func TestCreateFeatureFlagOverrideError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.CreateFeatureFlagOverride(
		context.Background(),
		&p42.CreateFeatureFlagOverrideRequest{TenantID: "abc", FlagName: "flag", Enabled: true},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func serveFeatureFlagOverrideConflict() (*httptest.Server, *p42.Client) {
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(
				p42.ConflictError{
					ResponseCode: http.StatusConflict,
					Message:      "exists",
					ErrorType:    "Conflict",
					Current:      &p42.FeatureFlagOverride{FlagName: "flag", TenantID: "abc", Enabled: true},
				},
			)
		},
	)

	srv := httptest.NewServer(handler)
	client := p42.NewClient(srv.URL)
	return srv, client
}

func verifyFeatureFlagOverrideConflict(t *testing.T, err error) {
	var clientErr *p42.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.Equal(t, "exists", clientErr.Message)
	require.Equal(t, "Conflict", clientErr.ErrorType)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, p42.ObjectTypeFeatureFlagOverride, clientErr.Current.ObjectType())
	ovr, ok := clientErr.Current.(*p42.FeatureFlagOverride)
	require.True(t, ok, "Expected Current to be of type *p42.FeatureFlagOverride")
	require.Equal(t, p42.FeatureFlagOverride{FlagName: "flag", TenantID: "abc", Enabled: true}, *ovr)
}

func TestCreateFeatureFlagOverrideConflictError(t *testing.T) {
	t.Parallel()

	srv, client := serveFeatureFlagOverrideConflict()
	defer srv.Close()

	_, err := client.CreateFeatureFlagOverride(
		context.Background(),
		&p42.CreateFeatureFlagOverrideRequest{TenantID: "abc", FlagName: "flag", Enabled: true},
	)
	verifyFeatureFlagOverrideConflict(t, err)
}

// nolint: dupl
func TestGetFeatureFlagPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3])
			require.Equal(t, escapedFeatureFlagName, parts[5])

			w.WriteHeader(http.StatusCreated)
			resp := p42.FeatureFlagOverride{
				FlagName: featureFlagNameThatNeedsEscaping,
				TenantID: tenantIDThatNeedsEscaping,
				Enabled:  true,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.CreateFeatureFlagOverride(
		context.Background(),
		&p42.CreateFeatureFlagOverrideRequest{
			TenantID: tenantIDThatNeedsEscaping,
			FlagName: featureFlagNameThatNeedsEscaping,
			Enabled:  true,
		},
	)
	require.NoError(t, err)
}

func TestGetFeatureFlagOverride(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/featureFlagOverrides/flag", r.URL.Path)
			require.Empty(t, r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.FeatureFlagOverride{FlagName: "flag", TenantID: "abc", Enabled: true}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	resp, err := client.GetFeatureFlagOverride(
		context.Background(),
		&p42.GetFeatureFlagOverrideRequest{TenantID: "abc", FlagName: "flag"},
	)
	require.NoError(t, err)
	require.Equal(t, "flag", resp.FlagName)
	require.True(t, resp.Enabled)
}

func TestGetFeatureFlagOverrideIncludeDeleted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/featureFlagOverrides/flag", r.URL.Path)
			require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.FeatureFlagOverride{FlagName: "flag", TenantID: "abc", Enabled: true}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	includeDeleted := true
	_, err := client.GetFeatureFlagOverride(
		context.Background(),
		&p42.GetFeatureFlagOverrideRequest{TenantID: "abc", FlagName: "flag", IncludeDeleted: &includeDeleted},
	)
	require.NoError(t, err)
}

func TestGetFeatureFlagOverrideError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.GetFeatureFlagOverride(
		context.Background(),
		&p42.GetFeatureFlagOverrideRequest{TenantID: "abc", FlagName: "flag"},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func TestGetFeatureFlagOverridePathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3])
			require.Equal(t, escapedFeatureFlagName, parts[5])

			w.WriteHeader(http.StatusOK)
			resp := p42.FeatureFlagOverride{
				FlagName: featureFlagNameThatNeedsEscaping,
				TenantID: tenantIDThatNeedsEscaping,
				Enabled:  true,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetFeatureFlagOverride(
		context.Background(),
		&p42.GetFeatureFlagOverrideRequest{
			TenantID: tenantIDThatNeedsEscaping,
			FlagName: featureFlagNameThatNeedsEscaping,
		},
	)
	require.NoError(t, err)
}

func TestUpdateFeatureFlagOverride(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, "/v1/tenants/abc/featureFlagOverrides/flag", r.URL.Path)
			require.Equal(t, "1", r.Header.Get("If-Match"))

			var reqBody p42.UpdateFeatureFlagOverrideRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.NotNil(t, reqBody.Enabled)
			require.True(t, *reqBody.Enabled)

			w.WriteHeader(http.StatusOK)
			resp := p42.FeatureFlagOverride{FlagName: "flag", TenantID: "abc", Enabled: true}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	enabled := true
	resp, err := client.UpdateFeatureFlagOverride(
		context.Background(), &p42.UpdateFeatureFlagOverrideRequest{
			TenantID: "abc",
			FlagName: "flag",
			Version:  1,
			Enabled:  &enabled,
		},
	)
	require.NoError(t, err)
	require.Equal(t, "flag", resp.FlagName)
	require.True(t, resp.Enabled)
}

func TestUpdateFeatureFlagOverrideError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	enabled := true
	_, err := client.UpdateFeatureFlagOverride(
		context.Background(), &p42.UpdateFeatureFlagOverrideRequest{
			TenantID: "abc", FlagName: "flag", Version: 1, Enabled: &enabled,
		},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func TestUpdateFeatureFlagOverrideConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveFeatureFlagOverrideConflict()
	defer srv.Close()

	enabled := true
	_, err := client.UpdateFeatureFlagOverride(
		context.Background(), &p42.UpdateFeatureFlagOverrideRequest{
			TenantID: "abc", FlagName: "flag", Version: 1, Enabled: &enabled,
		},
	)
	verifyFeatureFlagOverrideConflict(t, err)
}

// nolint: dupl
func TestUpdateFeatureFlagOverridePathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3])
			require.Equal(t, escapedFeatureFlagName, parts[5])

			w.WriteHeader(http.StatusOK)
			resp := p42.FeatureFlagOverride{
				FlagName: featureFlagNameThatNeedsEscaping,
				TenantID: tenantIDThatNeedsEscaping,
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	enabled := true
	_, err := client.UpdateFeatureFlagOverride(
		context.Background(), &p42.UpdateFeatureFlagOverrideRequest{
			TenantID: tenantIDThatNeedsEscaping,
			FlagName: featureFlagNameThatNeedsEscaping,
			Version:  1,
			Enabled:  &enabled,
		},
	)
	require.NoError(t, err)
}

// nolint: dupl
func TestListFeatureFlags(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/featureflags", r.URL.Path)
			require.Equal(t, "123", r.URL.Query().Get("maxResults"))
			require.Equal(t, tokenID, r.URL.Query().Get("token"))
			require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.ListFeatureFlagsResponse{FeatureFlags: []p42.FeatureFlag{{Name: "flag"}}}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	maxResults := 123
	includeDeleted := true
	resp, err := client.ListFeatureFlags(
		context.Background(),
		&p42.ListFeatureFlagsRequest{
			MaxResults:     &maxResults,
			Token:          util.Pointer(tokenID),
			IncludeDeleted: &includeDeleted,
		},
	)
	require.NoError(t, err)
	require.Len(t, resp.FeatureFlags, 1)
	require.Equal(t, "flag", resp.FeatureFlags[0].Name)
}

func TestListFeatureFlagsError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListFeatureFlags(context.Background(), &p42.ListFeatureFlagsRequest{})
	require.Error(t, err)
}

func TestUpdateFeatureFlag(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, "/v1/featureflags/flag", r.URL.Path)
			require.Equal(t, "1", r.Header.Get("If-Match"))

			var reqBody p42.UpdateFeatureFlagRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			require.NotNil(t, reqBody.Description)
			require.Equal(t, "new", *reqBody.Description)

			w.WriteHeader(http.StatusOK)
			resp := p42.FeatureFlag{Name: "flag", Description: "new", Version: 1}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	desc := "new"
	flag, err := client.UpdateFeatureFlag(
		context.Background(), &p42.UpdateFeatureFlagRequest{
			FlagName:    "flag",
			Version:     1,
			Description: &desc,
		},
	)
	require.NoError(t, err)
	require.Equal(t, "flag", flag.Name)
}

func TestUpdateFeatureFlagError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.UpdateFeatureFlag(
		context.Background(),
		&p42.UpdateFeatureFlagRequest{FlagName: "flag", Version: 1},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func TestUpdateFeatureFlagConflictError(t *testing.T) {
	t.Parallel()

	srv, client := serveFeatureFlagConflict()
	defer srv.Close()

	_, err := client.UpdateFeatureFlag(
		context.Background(),
		&p42.UpdateFeatureFlagRequest{FlagName: "flag", Version: 1},
	)
	verifyFeatureFlagConflict(t, err)
}

// nolint: dupl
func TestUpdateFeatureFlagPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 4, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedFeatureFlagName, parts[3])

			w.WriteHeader(http.StatusOK)
			resp := p42.FeatureFlag{Name: featureFlagNameThatNeedsEscaping}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.UpdateFeatureFlag(
		context.Background(), &p42.UpdateFeatureFlagRequest{
			FlagName: featureFlagNameThatNeedsEscaping,
			Version:  1,
		},
	)
	require.NoError(t, err)
}

func TestGetTenantFeatureFlags(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/featureflags", r.URL.Path)

			w.WriteHeader(http.StatusOK)
			resp := p42.GetTenantFeatureFlagsResponse{FeatureFlags: map[string]bool{"foo": true}}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	resp, err := client.GetTenantFeatureFlags(context.Background(), &p42.GetTenantFeatureFlagsRequest{TenantID: "abc"})
	require.NoError(t, err)
	require.True(t, resp.FeatureFlags["foo"])
}

func TestGetTenantFeatureFlagsError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.GetTenantFeatureFlags(context.Background(), &p42.GetTenantFeatureFlagsRequest{TenantID: "abc"})
	require.Error(t, err)
}

func TestGetTenantFeatureFlagsPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")

			w.WriteHeader(http.StatusOK)
			resp := p42.GetTenantFeatureFlagsResponse{}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.GetTenantFeatureFlags(
		context.Background(),
		&p42.GetTenantFeatureFlagsRequest{TenantID: tenantIDThatNeedsEscaping},
	)
	require.NoError(t, err)
}

func TestDeleteFeatureFlag(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, "/v1/featureflags/flag", r.URL.Path)
			require.Equal(t, "1", r.Header.Get("If-Match"))

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteFeatureFlag(context.Background(), &p42.DeleteFeatureFlagRequest{FlagName: "flag", Version: 1})
	require.NoError(t, err)
}

func TestDeleteFeatureFlagError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	err := client.DeleteFeatureFlag(context.Background(), &p42.DeleteFeatureFlagRequest{FlagName: "flag", Version: 1})
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func TestDeleteFeatureFlagConflictError(t *testing.T) {
	t.Parallel()

	srv, client := serveFeatureFlagConflict()
	defer srv.Close()

	err := client.DeleteFeatureFlag(context.Background(), &p42.DeleteFeatureFlagRequest{FlagName: "flag", Version: 1})
	verifyFeatureFlagConflict(t, err)
}

func TestDeleteFeatureFlagPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 4, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedFeatureFlagName, parts[3])

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteFeatureFlag(
		context.Background(),
		&p42.DeleteFeatureFlagRequest{FlagName: featureFlagNameThatNeedsEscaping, Version: 1},
	)
	require.NoError(t, err)
}
func TestDeleteFeatureFlagOverride(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, "/v1/tenants/abc/featureFlagOverrides/flag", r.URL.Path)
			require.Equal(t, "1", r.Header.Get("If-Match"))

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteFeatureFlagOverride(
		context.Background(),
		&p42.DeleteFeatureFlagOverrideRequest{TenantID: "abc", FlagName: "flag", Version: 1},
	)
	require.NoError(t, err)
}

func TestDeleteFeatureFlagOverrideError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	err := client.DeleteFeatureFlagOverride(
		context.Background(),
		&p42.DeleteFeatureFlagOverrideRequest{TenantID: "abc", FlagName: "flag", Version: 1},
	)
	var clientErr *p42.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func TestDeleteFeatureFlagOverrideConflictError(t *testing.T) {
	t.Parallel()

	srv, client := serveFeatureFlagOverrideConflict()
	defer srv.Close()

	err := client.DeleteFeatureFlagOverride(
		context.Background(),
		&p42.DeleteFeatureFlagOverrideRequest{TenantID: "abc", FlagName: "flag", Version: 1},
	)
	verifyFeatureFlagOverrideConflict(t, err)
}

func TestDeleteFeatureFlagOverridePathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3])
			require.Equal(t, escapedFeatureFlagName, parts[5])

			w.WriteHeader(http.StatusNoContent)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	err := client.DeleteFeatureFlagOverride(
		context.Background(),
		&p42.DeleteFeatureFlagOverrideRequest{
			TenantID: tenantIDThatNeedsEscaping,
			FlagName: featureFlagNameThatNeedsEscaping,
			Version:  1,
		},
	)
	require.NoError(t, err)
}
func TestListFeatureFlagOverrides(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/tenants/abc/featureFlagOverrides", r.URL.Path)
			require.Equal(t, "123", r.URL.Query().Get("maxResults"))
			require.Equal(t, tokenID, r.URL.Query().Get("token"))
			require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

			w.WriteHeader(http.StatusOK)
			resp := p42.ListFeatureFlagOverridesResponse{
				FeatureFlagOverrides: []p42.FeatureFlagOverride{
					{
						FlagName: "flag",
						TenantID: "abc",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	maxResults := 123
	includeDeleted := true
	resp, err := client.ListFeatureFlagOverrides(
		context.Background(), &p42.ListFeatureFlagOverridesRequest{
			TenantID:       "abc",
			MaxResults:     &maxResults,
			Token:          util.Pointer(tokenID),
			IncludeDeleted: &includeDeleted,
		},
	)
	require.NoError(t, err)
	require.Len(t, resp.FeatureFlagOverrides, 1)
	require.Equal(t, "flag", resp.FeatureFlagOverrides[0].FlagName)
}

func TestListFeatureFlagOverridesPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			escapedPath := r.URL.EscapedPath()
			parts := strings.Split(escapedPath, "/")
			require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
			require.Equal(t, escapedTenantID, parts[3])

			w.WriteHeader(http.StatusOK)
			resp := p42.ListFeatureFlagOverridesResponse{}
			_ = json.NewEncoder(w).Encode(resp)
		},
	)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := p42.NewClient(srv.URL)
	_, err := client.ListFeatureFlagOverrides(
		context.Background(),
		&p42.ListFeatureFlagOverridesRequest{TenantID: tenantIDThatNeedsEscaping},
	)
	require.NoError(t, err)
}

func TestListFeatureFlagOverridesError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListFeatureFlagOverrides(
		context.Background(),
		&p42.ListFeatureFlagOverridesRequest{TenantID: "abc"},
	)
	require.Error(t, err)
}

func TestFeatureFlagsHeader(t *testing.T) {
	t.Parallel()
	ffHeader := `{"ff":true}`

	cases := []struct {
		name   string
		status int
		resp   any
		call   func(*p42.Client) error
	}{
		{
			name:   "CreateTenant",
			status: http.StatusCreated,
			resp:   p42.Tenant{},
			call: func(c *p42.Client) error {
				_, err := c.CreateTenant(
					context.Background(), &p42.CreateTenantRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
						Type:         p42.TenantTypeUser,
					},
				)
				return err
			},
		},
		{
			name:   "GetTenant",
			status: http.StatusOK,
			resp:   p42.Tenant{},
			call: func(c *p42.Client) error {
				_, err := c.GetTenant(
					context.Background(), &p42.GetTenantRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
					},
				)
				return err
			},
		},
		{
			name:   "GetCurrentUser",
			status: http.StatusOK,
			resp:   p42.Tenant{},
			call: func(c *p42.Client) error {
				_, err := c.GetCurrentUser(
					context.Background(), &p42.GetCurrentUserRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					},
				)
				return err
			},
		},
		{
			name:   "GetTenantFeatureFlags",
			status: http.StatusOK,
			resp:   p42.GetTenantFeatureFlagsResponse{},
			call: func(c *p42.Client) error {
				_, err := c.GetTenantFeatureFlags(
					context.Background(), &p42.GetTenantFeatureFlagsRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
					},
				)
				return err
			},
		},
		{
			name:   "GenerateWebUIToken",
			status: http.StatusCreated,
			resp:   p42.GenerateWebUITokenResponse{JWT: "x"},
			call: func(c *p42.Client) error {
				_, err := c.GenerateWebUIToken(
					context.Background(), &p42.GenerateWebUITokenRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
						TokenID:      "id",
					},
				)
				return err
			},
		},
		{
			name:   "GenerateRunnerToken",
			status: http.StatusCreated,
			resp: p42.GenerateRunnerTokenResponse{
				RunnerTokenMetadata: p42.RunnerTokenMetadata{
					TokenID:   "id",
					ExpiresAt: time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC),
				},
				Token: "tok",
			},
			call: func(c *p42.Client) error {
				_, err := c.GenerateRunnerToken(
					context.Background(),
					&p42.GenerateRunnerTokenRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
						RunnerID:     "runner",
						TokenID:      "token-id",
					},
				)
				return err
			},
		},
		{
			name:   "CreateEnvironment",
			status: http.StatusCreated,
			resp:   p42.Environment{},
			call: func(c *p42.Client) error {
				_, err := c.CreateEnvironment(
					context.Background(), &p42.CreateEnvironmentRequest{
						FeatureFlags:  p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:      "t",
						EnvironmentID: "e",
					},
				)
				return err
			},
		},
		{
			name:   "GetRunner",
			status: http.StatusOK,
			resp:   p42.Runner{},
			call: func(c *p42.Client) error {
				_, err := c.GetRunner(
					context.Background(), &p42.GetRunnerRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
						RunnerID:     "r",
					},
				)
				return err
			},
		},
		{
			name:   "GetEnvironment",
			status: http.StatusOK,
			resp:   p42.Environment{},
			call: func(c *p42.Client) error {
				_, err := c.GetEnvironment(
					context.Background(), &p42.GetEnvironmentRequest{
						FeatureFlags:  p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:      "t",
						EnvironmentID: "e",
					},
				)
				return err
			},
		},
		{
			name:   "UpdateEnvironment",
			status: http.StatusOK,
			resp:   p42.Environment{},
			call: func(c *p42.Client) error {
				_, err := c.UpdateEnvironment(
					context.Background(), &p42.UpdateEnvironmentRequest{
						FeatureFlags:  p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:      "t",
						EnvironmentID: "e",
						Version:       1,
					},
				)
				return err
			},
		},
		{
			name:   "ListEnvironments",
			status: http.StatusOK,
			resp:   p42.List[p42.Environment]{},
			call: func(c *p42.Client) error {
				_, err := c.ListEnvironments(
					context.Background(), &p42.ListEnvironmentsRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
					},
				)
				return err
			},
		},
		{
			name:   "DeleteEnvironment",
			status: http.StatusNoContent,
			resp:   nil,
			call: func(c *p42.Client) error {
				return c.DeleteEnvironment(
					context.Background(), &p42.DeleteEnvironmentRequest{
						FeatureFlags:  p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:      "t",
						EnvironmentID: "e",
						Version:       1,
					},
				)
			},
		},
		{
			name:   "UploadTurnLogs",
			status: http.StatusOK,
			resp:   p42.UploadTurnLogsResponse{},
			call: func(c *p42.Client) error {
				_, err := c.UploadTurnLogs(
					context.Background(), &p42.UploadTurnLogsRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
						TaskID:       "task",
						TurnIndex:    1,
						Version:      1,
						Index:        0,
						Logs:         []p42.TurnLog{},
					},
				)
				return err
			},
		},
		{
			name:   "GetLastTurnLog",
			status: http.StatusOK,
			resp:   p42.LastTurnLog{},
			call: func(c *p42.Client) error {
				_, err := c.GetLastTurnLog(
					context.Background(), &p42.GetLastTurnLogRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
						TaskID:       "task",
						TurnIndex:    1,
					},
				)
				return err
			},
		},
		{
			name:   "StreamTurnLogs",
			status: http.StatusNoContent,
			resp:   nil,
			call: func(c *p42.Client) error {
				body, err := c.StreamTurnLogs(
					context.Background(), &p42.StreamTurnLogsRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
						TaskID:       "task",
						TurnIndex:    1,
					},
				)
				if body != nil {
					_ = body.Close()
				}
				return err
			},
		},
		{
			name:   "GetTask",
			status: http.StatusOK,
			resp:   p42.Task{},
			call: func(c *p42.Client) error {
				_, err := c.GetTask(
					context.Background(), &p42.GetTaskRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
						TaskID:       "task",
					},
				)
				return err
			},
		},
		{
			name:   "CreateTask",
			status: http.StatusCreated,
			resp:   p42.Task{},
			call: func(c *p42.Client) error {
				_, err := c.CreateTask(
					context.Background(), &p42.CreateTaskRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
						TaskID:       "task",
						RepoInfo:     map[string]*p42.RepoInfo{},
					},
				)
				return err
			},
		},
		{
			name:   "UpdateTask",
			status: http.StatusOK,
			resp:   p42.Task{},
			call: func(c *p42.Client) error {
				title := ""
				_, err := c.UpdateTask(
					context.Background(), &p42.UpdateTaskRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
						TaskID:       "task",
						Version:      1,
						Title:        &title,
					},
				)
				return err
			},
		},
		{
			name:   "DeleteTask",
			status: http.StatusNoContent,
			resp:   nil,
			call: func(c *p42.Client) error {
				return c.DeleteTask(
					context.Background(), &p42.DeleteTaskRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
						TaskID:       "task",
						Version:      1,
					},
				)
			},
		},
		{
			name:   "DeleteWorkstreamTask",
			status: http.StatusNoContent,
			resp:   nil,
			call: func(c *p42.Client) error {
				return c.DeleteWorkstreamTask(
					context.Background(), &p42.DeleteWorkstreamTaskRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
						WorkstreamID: "ws",
						TaskID:       "task",
						Version:      1,
					},
				)
			},
		},
		{
			name:   "ListTasks",
			status: http.StatusOK,
			resp:   p42.ListTasksResponse{},
			call: func(c *p42.Client) error {
				_, err := c.ListTasks(
					context.Background(), &p42.ListTasksRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
					},
				)
				return err
			},
		},
		{
			name:   "CreateTurn",
			status: http.StatusCreated,
			resp:   p42.Turn{},
			call: func(c *p42.Client) error {
				_, err := c.CreateTurn(
					context.Background(), &p42.CreateTurnRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
						TaskID:       "task",
						TurnIndex:    2,
						TaskVersion:  1,
						Prompt:       "",
					},
				)
				return err
			},
		},
		{
			name:   "GetTurn",
			status: http.StatusOK,
			resp:   p42.Turn{},
			call: func(c *p42.Client) error {
				_, err := c.GetTurn(
					context.Background(), &p42.GetTurnRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
						TaskID:       "task",
						TurnIndex:    1,
					},
				)
				return err
			},
		},
		{
			name:   "GetLastTurn",
			status: http.StatusOK,
			resp:   p42.Turn{},
			call: func(c *p42.Client) error {
				_, err := c.GetLastTurn(
					context.Background(), &p42.GetLastTurnRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
						TaskID:       "task",
					},
				)
				return err
			},
		},
		{
			name:   "UpdateTurn",
			status: http.StatusOK,
			resp:   p42.Turn{},
			call: func(c *p42.Client) error {
				status := "s"
				_, err := c.UpdateTurn(
					context.Background(), &p42.UpdateTurnRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
						TaskID:       "task",
						TurnIndex:    1,
						Version:      1,
						Status:       &status,
					},
				)
				return err
			},
		},
		{
			name:   "ListTurns",
			status: http.StatusOK,
			resp:   p42.ListTurnsResponse{},
			call: func(c *p42.Client) error {
				_, err := c.ListTurns(
					context.Background(), &p42.ListTurnsRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
						TaskID:       "task",
					},
				)
				return err
			},
		},
		{
			name:   "ListPolicies",
			status: http.StatusOK,
			resp:   p42.ListPoliciesResponse{},
			call: func(c *p42.Client) error {
				_, err := c.ListPolicies(
					context.Background(), &p42.ListPoliciesRequest{
						FeatureFlags: p42.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
						TenantID:     "t",
					},
				)
				return err
			},
		},
	}

	for _, tt := range cases {
		t.Run(
			tt.name, func(t *testing.T) {
				t.Parallel()
				srv := httptest.NewServer(
					http.HandlerFunc(
						func(w http.ResponseWriter, r *http.Request) {
							require.Equal(t, ffHeader, r.Header.Get("X-EventHorizon-FeatureFlags"))
							w.WriteHeader(tt.status)
							if tt.resp != nil {
								_ = json.NewEncoder(w).Encode(tt.resp)
							}
						},
					),
				)
				defer srv.Close()
				client := p42.NewClient(srv.URL)
				require.NoError(t, tt.call(client))
			},
		)
	}
}
