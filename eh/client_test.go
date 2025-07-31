package eh_test

import (
	"context"
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
	"github.com/debugging-sucks/clock"
	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	"github.com/debugging-sucks/event-horizon-sdk-go/internal/util"
	sigv4auth "github.com/debugging-sucks/sigv4util/server/sigv4auth"
	"github.com/stretchr/testify/require"
)

const (
	expectedSigCreateTenant   = "UE9TVCAvIEhUVFAvMS4xDQpIb3N0OiBzdHMudXMtd2VzdC0yLmFtYXpvbmF3cy5jb20NClVzZXItQWdlbnQ6IEdvLWh0dHAtY2xpZW50LzEuMQ0KVHJhbnNmZXItRW5jb2Rpbmc6IGNodW5rZWQNCkFjY2VwdDogYXBwbGljYXRpb24vanNvbg0KQWNjZXB0LUVuY29kaW5nOiBpZGVudGl0eQ0KQXV0aG9yaXphdGlvbjogQVdTNC1ITUFDLVNIQTI1NiBDcmVkZW50aWFsPUFLSUQvMjAyNTAxMDEvdXMtd2VzdC0yL3N0cy9hd3M0X3JlcXVlc3QsIFNpZ25lZEhlYWRlcnM9YWNjZXB0O2FjY2VwdC1lbmNvZGluZztjb250ZW50LXR5cGU7aG9zdDt4LWFtei1kYXRlO3gtYW16LXNlY3VyaXR5LXRva2VuO3gtZXZlbnQtaG9yaXpvbi1yZXF1ZXN0LWhhc2gsIFNpZ25hdHVyZT1lZmI1MGZlODE4NzRjOWFkMjI2ODJiNjAzYjI2OTkxNDQ4ZTRjNTIzODJmMTM4ZWM2NzQ1YmIzOTE0YzNiM2E1DQpDb250ZW50LVR5cGU6IGFwcGxpY2F0aW9uL3gtd3d3LWZvcm0tdXJsZW5jb2RlZA0KWC1BbXotRGF0ZTogMjAyNTAxMDFUMDAwMDAwWg0KWC1BbXotU2VjdXJpdHktVG9rZW46IFRPS0VODQpYLUV2ZW50LUhvcml6b24tUmVxdWVzdC1IYXNoOiAyOTdmNzRkYmRkY2RmZGU3NjY4OTYyZGYwM2YxMGMwZjVmYzYwNzA3YmExNGM2ZjJhNDgwMzE0ZDAxMzg5ZjZmDQoNCjJkDQpBY3Rpb249R2V0Q2FsbGVySWRlbnRpdHkmVmVyc2lvbj0yMDExLTA2LTE1DQoNCjANCg0K"
	expectedSigGetTenant      = "UE9TVCAvIEhUVFAvMS4xDQpIb3N0OiBzdHMudXMtd2VzdC0yLmFtYXpvbmF3cy5jb20NClVzZXItQWdlbnQ6IEdvLWh0dHAtY2xpZW50LzEuMQ0KVHJhbnNmZXItRW5jb2Rpbmc6IGNodW5rZWQNCkFjY2VwdDogYXBwbGljYXRpb24vanNvbg0KQWNjZXB0LUVuY29kaW5nOiBpZGVudGl0eQ0KQXV0aG9yaXphdGlvbjogQVdTNC1ITUFDLVNIQTI1NiBDcmVkZW50aWFsPUFLSUQvMjAyNTAxMDEvdXMtd2VzdC0yL3N0cy9hd3M0X3JlcXVlc3QsIFNpZ25lZEhlYWRlcnM9YWNjZXB0O2FjY2VwdC1lbmNvZGluZztjb250ZW50LXR5cGU7aG9zdDt4LWFtei1kYXRlO3gtYW16LXNlY3VyaXR5LXRva2VuO3gtZXZlbnQtaG9yaXpvbi1yZXF1ZXN0LWhhc2gsIFNpZ25hdHVyZT02OGU5ZWZhZTJkNjc0ZTc3NTViZWRiMDViNTYyOWNkZGJiY2QwMGZjYWQzMzE2Mjg5OWQ3ZGM2NzZhZWMxMDg3DQpDb250ZW50LVR5cGU6IGFwcGxpY2F0aW9uL3gtd3d3LWZvcm0tdXJsZW5jb2RlZA0KWC1BbXotRGF0ZTogMjAyNTAxMDFUMDAwMDAwWg0KWC1BbXotU2VjdXJpdHktVG9rZW46IFRPS0VODQpYLUV2ZW50LUhvcml6b24tUmVxdWVzdC1IYXNoOiBiYzhmOWZlNDM3ZjEwZmMwZWQ0YmExOWRkZjYyNmEzN2Y4NmI0Y2Y3Mzg4MTZkOGI1YTQ3ZmRmNmNjNWFiMTFlDQoNCjJkDQpBY3Rpb249R2V0Q2FsbGVySWRlbnRpdHkmVmVyc2lvbj0yMDExLTA2LTE1DQoNCjANCg0K"
	expectedSigGetCurrentUser = "UE9TVCAvIEhUVFAvMS4xDQpIb3N0OiBzdHMudXMtd2VzdC0yLmFtYXpvbmF3cy5jb20NClVzZXItQWdlbnQ6IEdvLWh0dHAtY2xpZW50LzEuMQ0KVHJhbnNmZXItRW5jb2Rpbmc6IGNodW5rZWQNCkFjY2VwdDogYXBwbGljYXRpb24vanNvbg0KQWNjZXB0LUVuY29kaW5nOiBpZGVudGl0eQ0KQXV0aG9yaXphdGlvbjogQVdTNC1ITUFDLVNIQTI1NiBDcmVkZW50aWFsPUFLSUQvMjAyNTAxMDEvdXMtd2VzdC0yL3N0cy9hd3M0X3JlcXVlc3QsIFNpZ25lZEhlYWRlcnM9YWNjZXB0O2FjY2VwdC1lbmNvZGluZztjb250ZW50LXR5cGU7aG9zdDt4LWFtei1kYXRlO3gtYW16LXNlY3VyaXR5LXRva2VuO3gtZXZlbnQtaG9yaXpvbi1yZXF1ZXN0LWhhc2gsIFNpZ25hdHVyZT0yMmEwYmVhMjg1ZTU4NTE3NjExMGJjYjY1NzEwNjM3YzdmYTYwNzVjOTA2MGFhNWIxYTZjZjc2Y2UxMGNmZjM5DQpDb250ZW50LVR5cGU6IGFwcGxpY2F0aW9uL3gtd3d3LWZvcm0tdXJsZW5jb2RlZA0KWC1BbXotRGF0ZTogMjAyNTAxMDFUMDAwMDAwWg0KWC1BbXotU2VjdXJpdHktVG9rZW46IFRPS0VODQpYLUV2ZW50LUhvcml6b24tUmVxdWVzdC1IYXNoOiBjMGRkNGY0ODFhM2I2OTVlMTM2MThkZWZmZjEyMDA0OWNlNWZhM2YxM2UwYTEzZmQ4ZTcwYmI4YWU1MjhmMWJhDQoNCjJkDQpBY3Rpb249R2V0Q2FsbGVySWRlbnRpdHkmVmVyc2lvbj0yMDExLTA2LTE1DQoNCjANCg0K"

	tenantIDThatNeedsEscaping = "foo/../../bar"
	escapedTenantID           = "foo%2F..%2F..%2Fbar"

	tokenIDThatNeedsEscaping       = "tok/../../id"       // #nosec G101: This is not a credential.
	escapedTokenID                 = "tok%2F..%2F..%2Fid" // #nosec G101: This is not a credential.
	environmentIDThatNeedsEscaping = "env/../../id"
	escapedEnvironmentID           = "env%2F..%2F..%2Fid"
	taskIDThatNeedsEscaping        = "task/../../id"
	escapedTaskID                  = "task%2F..%2F..%2Fid"

	tokenID    = "tok"
	taskTitle  = "new"
	turnStatus = "Done"
)

func TestCreateTenant(t *testing.T) {
	t.Parallel()
	// Mock server returns successful response
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/v1/tenants/abc", r.URL.Path)

		var reqBody eh.CreateTenantRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.Equal(t, eh.TenantTypeUser, reqBody.Type)

		w.WriteHeader(http.StatusCreated)
		resp := eh.Tenant{TenantID: "abc", Type: eh.TenantTypeUser, Version: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	tenant, err := client.CreateTenant(context.Background(), &eh.CreateTenantRequest{TenantID: "abc", Type: eh.TenantTypeUser})
	require.NoError(t, err)
	require.Equal(t, "abc", tenant.TenantID)
}

func TestCreateTenantError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.CreateTenant(context.Background(), &eh.CreateTenantRequest{TenantID: "abc", Type: eh.TenantTypeUser})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestCreateTenantConflictError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(eh.ConflictError{
			ResponseCode: http.StatusConflict,
			Message:      "exists",
			ErrorType:    "Conflict",
			Current:      &eh.Tenant{TenantID: "abc"},
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.CreateTenant(context.Background(), &eh.CreateTenantRequest{TenantID: "abc", Type: eh.TenantTypeUser})
	var clientErr *eh.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.Equal(t, "exists", clientErr.Message)
	require.Equal(t, "Conflict", clientErr.ErrorType)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, eh.ObjectTypeTenant, clientErr.Current.ObjectType())
	tenant, ok := clientErr.Current.(*eh.Tenant)
	require.True(t, ok, "Expected Current to be of type *eh.Tenant")
	require.Equal(t, eh.Tenant{TenantID: "abc"}, *tenant)
}

func TestGetTenant(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc", r.URL.Path)

		w.WriteHeader(http.StatusOK)
		resp := eh.Tenant{TenantID: "abc", Type: eh.TenantTypeUser, Version: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	tenant, err := client.GetTenant(context.Background(), &eh.GetTenantRequest{TenantID: "abc"})
	require.NoError(t, err)
	require.Equal(t, "abc", tenant.TenantID)
}

func TestGetTenantError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(eh.Error{ResponseCode: http.StatusNotFound, Message: "nope", ErrorType: "NotFound"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetTenant(context.Background(), &eh.GetTenantRequest{TenantID: "abc"})
	require.Error(t, err)
}

func TestGetCurrentUser(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/current-user", r.URL.Path)

		w.WriteHeader(http.StatusOK)
		resp := eh.Tenant{TenantID: "abc", Type: eh.TenantTypeUser, Version: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	tenant, err := client.GetCurrentUser(context.Background(), &eh.GetCurrentUserRequest{})
	require.NoError(t, err)
	require.Equal(t, "abc", tenant.TenantID)
}

func TestGetCurrentUserError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(eh.Error{ResponseCode: http.StatusForbidden, Message: "nope", ErrorType: "Forbidden"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetCurrentUser(context.Background(), &eh.GetCurrentUserRequest{})
	require.Error(t, err)
}

func TestCreateTenantPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the path and verify proper escaping
		escapedPath := r.URL.EscapedPath()

		// Split the path and check the escaped tenant ID part
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, len(parts), 4, "path doesn't have correct # of parts: %s", escapedPath)

		// The last part should be the escaped tenant ID
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")

		w.WriteHeader(http.StatusCreated)
		resp := eh.Tenant{TenantID: tenantIDThatNeedsEscaping, Type: eh.TenantTypeUser, Version: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.CreateTenant(context.Background(), &eh.CreateTenantRequest{
		TenantID: tenantIDThatNeedsEscaping,
		Type:     eh.TenantTypeUser,
	})

	require.NoError(t, err)
}

func TestGetTenantPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the path and verify proper escaping
		escapedPath := r.URL.EscapedPath()

		// Split the path and check the escaped tenant ID part
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, len(parts), 4, "path doesn't have correct # of parts: %s", escapedPath)

		// The last part should be the escaped tenant ID
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")

		w.WriteHeader(http.StatusOK)
		resp := eh.Tenant{TenantID: tenantIDThatNeedsEscaping, Type: eh.TenantTypeUser, Version: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetTenant(context.Background(), &eh.GetTenantRequest{
		TenantID: tenantIDThatNeedsEscaping,
	})

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
		Credentials: aws.NewCredentialsCache(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID:     "AKID",
				SecretAccessKey: "SECRET",
				SessionToken:    "TOKEN",
			},
		}),
		Region: "us-west-2",
	}

	clk := clock.NewFakeClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/tenants/abc", func(w http.ResponseWriter, r *http.Request) {
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
		resp := eh.Tenant{TenantID: "abc", Type: eh.TenantTypeUser, Version: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/v1/current-user", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		verifySigv4(t, r, &cfg, expectedSigGetCurrentUser, clk)
		w.WriteHeader(http.StatusOK)
		resp := eh.Tenant{TenantID: "abc", Type: eh.TenantTypeUser, Version: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewUnstartedServer(mux)
	defer srv.Close()
	_ = srv.Listener.Close()
	var err error
	srv.Listener, err = net.Listen("tcp", "localhost:4242")
	require.NoError(t, err)
	srv.Start()

	client := eh.NewClient(srv.URL, eh.WithSigv4Auth(cfg, clk))
	_, err = client.CreateTenant(context.Background(), &eh.CreateTenantRequest{TenantID: "abc", Type: eh.TenantTypeUser})
	require.NoError(t, err)

	_, err = client.GetTenant(context.Background(), &eh.GetTenantRequest{TenantID: "abc"})
	require.NoError(t, err)

	_, err = client.GetCurrentUser(context.Background(), &eh.GetCurrentUserRequest{})
	require.NoError(t, err)
}

func TestListPolicies(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/policies", r.URL.Path)
		require.Equal(t, "123", r.URL.Query().Get("maxResults"))
		require.Equal(t, tokenID, r.URL.Query().Get("token"))

		w.WriteHeader(http.StatusOK)
		resp := eh.ListPoliciesResponse{Policies: []eh.Policy{{Name: "p"}}}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	maxResults := 123
	resp, err := client.ListPolicies(context.Background(), &eh.ListPoliciesRequest{TenantID: "abc", MaxResults: &maxResults, Token: util.Pointer(tokenID)})
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	require.Equal(t, "p", resp.Policies[0].Name)
}

func TestListPoliciesError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListPolicies(context.Background(), &eh.ListPoliciesRequest{TenantID: "abc"})
	require.Error(t, err)
}

func TestListPoliciesPathEscaping(t *testing.T) {
	t.Parallel()
	tenantID := tenantIDThatNeedsEscaping
	expectedEscaped := "foo%2F..%2F..%2Fbar"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, expectedEscaped, parts[3], "TenantID not properly escaped in URL path")

		w.WriteHeader(http.StatusOK)
		resp := eh.ListPoliciesResponse{}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.ListPolicies(context.Background(), &eh.ListPoliciesRequest{TenantID: tenantID})
	require.NoError(t, err)
}

func TestGenerateWebUIToken(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/v1/tenants/abc/ui-tokens/tok", r.URL.Path)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(eh.GenerateWebUITokenResponse{JWT: "jwt"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	resp, err := client.GenerateWebUIToken(context.Background(), &eh.GenerateWebUITokenRequest{TenantID: "abc", TokenID: tokenID})
	require.NoError(t, err)
	require.Equal(t, "jwt", resp.JWT)
}

func TestGenerateWebUITokenError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()
	_, err := client.GenerateWebUIToken(context.Background(), &eh.GenerateWebUITokenRequest{TenantID: "abc", TokenID: tokenID})
	require.Error(t, err)
}

func TestGenerateWebUITokenPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedTokenID, parts[5], "TokenID not properly escaped in URL path")

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(eh.GenerateWebUITokenResponse{JWT: "jwt"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GenerateWebUIToken(context.Background(), &eh.GenerateWebUITokenRequest{TenantID: tenantIDThatNeedsEscaping, TokenID: tokenIDThatNeedsEscaping})
	require.NoError(t, err)
}

func TestCreateEnvironment(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/v1/tenants/abc/environments/env", r.URL.Path)

		var reqBody eh.CreateEnvironmentRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.Equal(t, "env", reqBody.Name)

		w.WriteHeader(http.StatusCreated)
		resp := eh.Environment{TenantID: "abc", EnvironmentID: "env"}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	env, err := client.CreateEnvironment(context.Background(), &eh.CreateEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", Name: "env"})
	require.NoError(t, err)
	require.Equal(t, "env", env.EnvironmentID)
}

func TestCreateEnvironmentError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.CreateEnvironment(context.Background(), &eh.CreateEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", Name: "env"})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func serveEnvironmentConflict() (*httptest.Server, *eh.Client) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(eh.ConflictError{
			ResponseCode: http.StatusConflict,
			Message:      "exists",
			ErrorType:    "Conflict",
			Current:      &eh.Environment{EnvironmentID: "env"},
		})
	})

	srv := httptest.NewServer(handler)

	client := eh.NewClient(srv.URL)
	return srv, client
}

func serveBadRequest() (*httptest.Server, *eh.Client) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(eh.Error{ResponseCode: http.StatusBadRequest, Message: "bad", ErrorType: "BadRequest"})
	})

	srv := httptest.NewServer(handler)
	client := eh.NewClient(srv.URL)

	return srv, client
}

func TestCreateEnvironmentConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveEnvironmentConflict()
	defer srv.Close()

	_, err := client.CreateEnvironment(context.Background(), &eh.CreateEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", Name: "env"})
	verifyEnvironmentConflict(t, err)
}

func verifyEnvironmentConflict(t *testing.T, err error) {
	var clientErr *eh.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.Equal(t, "exists", clientErr.Message)
	require.Equal(t, "Conflict", clientErr.ErrorType)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, eh.ObjectTypeEnvironment, clientErr.Current.ObjectType())
	env, ok := clientErr.Current.(*eh.Environment)
	require.True(t, ok, "Expected Current to be of type *eh.Environment")
	require.Equal(t, eh.Environment{EnvironmentID: "env"}, *env)
}

func serveTaskConflict() (*httptest.Server, *eh.Client) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(eh.ConflictError{
			ResponseCode: http.StatusConflict,
			Message:      "exists",
			ErrorType:    "Conflict",
			Current:      &eh.Task{TaskID: "task"},
		})
	})

	srv := httptest.NewServer(handler)
	client := eh.NewClient(srv.URL)
	return srv, client
}

func verifyTaskConflict(t *testing.T, err error) {
	var clientErr *eh.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.Equal(t, "exists", clientErr.Message)
	require.Equal(t, "Conflict", clientErr.ErrorType)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, eh.ObjectTypeTask, clientErr.Current.ObjectType())
	task, ok := clientErr.Current.(*eh.Task)
	require.True(t, ok, "Expected Current to be of type *eh.Task")
	require.Equal(t, eh.Task{TaskID: "task"}, *task)
}

// nolint: dupl
func TestCreateEnvironmentPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedEnvironmentID, parts[5], "EnvironmentID not properly escaped in URL path")

		w.WriteHeader(http.StatusCreated)
		resp := eh.Environment{TenantID: tenantIDThatNeedsEscaping, EnvironmentID: environmentIDThatNeedsEscaping}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.CreateEnvironment(context.Background(), &eh.CreateEnvironmentRequest{TenantID: tenantIDThatNeedsEscaping, EnvironmentID: environmentIDThatNeedsEscaping, Name: "env"})
	require.NoError(t, err)
}

func TestGetEnvironment(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/environments/env", r.URL.Path)
		require.Empty(t, r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.Environment{TenantID: "abc", EnvironmentID: "env"}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	env, err := client.GetEnvironment(context.Background(), &eh.GetEnvironmentRequest{TenantID: "abc", EnvironmentID: "env"})
	require.NoError(t, err)
	require.Equal(t, "env", env.EnvironmentID)
}

func TestGetEnvironmentIncludeDeleted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/environments/env", r.URL.Path)
		require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.Environment{TenantID: "abc", EnvironmentID: "env"}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	includeDeleted := true
	_, err := client.GetEnvironment(context.Background(), &eh.GetEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", IncludeDeleted: &includeDeleted})
	require.NoError(t, err)
}

func TestGetEnvironmentError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(eh.Error{ResponseCode: http.StatusNotFound, Message: "nope", ErrorType: "NotFound"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetEnvironment(context.Background(), &eh.GetEnvironmentRequest{TenantID: "abc", EnvironmentID: "env"})
	require.Error(t, err)
}

// nolint: dupl
func TestGetEnvironmentPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedEnvironmentID, parts[5], "EnvironmentID not properly escaped in URL path")

		w.WriteHeader(http.StatusOK)
		resp := eh.Environment{TenantID: tenantIDThatNeedsEscaping, EnvironmentID: environmentIDThatNeedsEscaping}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetEnvironment(context.Background(), &eh.GetEnvironmentRequest{TenantID: tenantIDThatNeedsEscaping, EnvironmentID: environmentIDThatNeedsEscaping})
	require.NoError(t, err)
}

func TestListEnvironments(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/environments", r.URL.Path)
		require.Equal(t, "123", r.URL.Query().Get("maxResults"))
		require.Equal(t, tokenID, r.URL.Query().Get("token"))
		require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.ListEnvironmentsResponse{Environments: []eh.Environment{{EnvironmentID: "env"}}}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	maxResults := 123
	includeDeleted := true
	resp, err := client.ListEnvironments(context.Background(), &eh.ListEnvironmentsRequest{TenantID: "abc", MaxResults: &maxResults, Token: util.Pointer(tokenID), IncludeDeleted: &includeDeleted})
	require.NoError(t, err)
	require.Len(t, resp.Environments, 1)
	require.Equal(t, "env", resp.Environments[0].EnvironmentID)
}

func TestListEnvironmentsError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()
	_, err := client.ListEnvironments(context.Background(), &eh.ListEnvironmentsRequest{TenantID: "abc"})
	require.Error(t, err)
}

func TestListEnvironmentsPathEscaping(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")

		w.WriteHeader(http.StatusOK)
		resp := eh.ListEnvironmentsResponse{}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.ListEnvironments(context.Background(), &eh.ListEnvironmentsRequest{TenantID: tenantIDThatNeedsEscaping})
	require.NoError(t, err)
}

// nolint: dupl
func TestUpdateEnvironment(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPatch, r.Method)
		require.Equal(t, "/v1/tenants/abc/environments/env", r.URL.Path)
		require.Equal(t, "1", r.Header.Get("If-Match"))

		var reqBody eh.UpdateEnvironmentRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.NotNil(t, reqBody.Name)
		require.Equal(t, "env2", *reqBody.Name)

		w.WriteHeader(http.StatusOK)
		resp := eh.Environment{TenantID: "abc", EnvironmentID: "env"}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	name := "env2"
	env, err := client.UpdateEnvironment(context.Background(), &eh.UpdateEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", Version: 1, Name: &name})
	require.NoError(t, err)
	require.Equal(t, "env", env.EnvironmentID)
}

func TestUpdateEnvironmentError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()
	_, err := client.UpdateEnvironment(context.Background(), &eh.UpdateEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", Version: 1})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestUpdateEnvironmentConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveEnvironmentConflict()
	defer srv.Close()
	_, err := client.UpdateEnvironment(context.Background(), &eh.UpdateEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", Version: 1})
	verifyEnvironmentConflict(t, err)
}

func TestUpdateEnvironmentPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedEnvironmentID, parts[5], "EnvironmentID not properly escaped in URL path")

		w.WriteHeader(http.StatusOK)
		resp := eh.Environment{TenantID: tenantIDThatNeedsEscaping, EnvironmentID: environmentIDThatNeedsEscaping}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	name := "env"
	_, err := client.UpdateEnvironment(context.Background(), &eh.UpdateEnvironmentRequest{TenantID: tenantIDThatNeedsEscaping, EnvironmentID: environmentIDThatNeedsEscaping, Version: 1, Name: &name})
	require.NoError(t, err)
}

func TestDeleteEnvironment(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/v1/tenants/abc/environments/env", r.URL.Path)
		require.Equal(t, "1", r.Header.Get("If-Match"))

		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	err := client.DeleteEnvironment(context.Background(), &eh.DeleteEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", Version: 1})
	require.NoError(t, err)
}

func TestDeleteEnvironmentError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()
	err := client.DeleteEnvironment(context.Background(), &eh.DeleteEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", Version: 1})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestDeleteEnvironmentConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveEnvironmentConflict()
	defer srv.Close()
	err := client.DeleteEnvironment(context.Background(), &eh.DeleteEnvironmentRequest{TenantID: "abc", EnvironmentID: "env", Version: 1})
	verifyEnvironmentConflict(t, err)
}

func TestDeleteEnvironmentPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedEnvironmentID, parts[5], "EnvironmentID not properly escaped in URL path")

		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	err := client.DeleteEnvironment(context.Background(), &eh.DeleteEnvironmentRequest{TenantID: tenantIDThatNeedsEscaping, EnvironmentID: environmentIDThatNeedsEscaping, Version: 1})
	require.NoError(t, err)
}

func TestCreateTask(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/v1/tenants/abc/tasks/task", r.URL.Path)

		var reqBody eh.CreateTaskRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.Equal(t, "title", reqBody.Title)

		w.WriteHeader(http.StatusCreated)
		resp := eh.Task{TenantID: "abc", TaskID: "task"}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	model := eh.ModelTypeCodexMini
	task, err := client.CreateTask(context.Background(), &eh.CreateTaskRequest{
		TenantID:      "abc",
		TaskID:        "task",
		Title:         "title",
		EnvironmentID: "env",
		Prompt:        "do",
		AssignedToAI:  true,
		Model:         &model,
		Branches:      map[string]string{},
	})
	require.NoError(t, err)
	require.Equal(t, "task", task.TaskID)
}

func TestCreateTaskError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()
	model := eh.ModelTypeCodexMini
	_, err := client.CreateTask(context.Background(), &eh.CreateTaskRequest{
		TenantID:      "abc",
		TaskID:        "task",
		Title:         "title",
		EnvironmentID: "env",
		Prompt:        "do",
		AssignedToAI:  true,
		Model:         &model,
		Branches:      map[string]string{},
	})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestCreateTaskConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveTaskConflict()
	defer srv.Close()
	model := eh.ModelTypeCodexMini
	_, err := client.CreateTask(context.Background(), &eh.CreateTaskRequest{
		TenantID:      "abc",
		TaskID:        "task",
		Title:         "title",
		EnvironmentID: "env",
		Prompt:        "do",
		AssignedToAI:  true,
		Model:         &model,
		Branches:      map[string]string{},
	})
	verifyTaskConflict(t, err)
}

func TestCreateTaskPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedTaskID, parts[5], "TaskID not properly escaped in URL path")

		w.WriteHeader(http.StatusCreated)
		resp := eh.Task{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	model := eh.ModelTypeCodexMini
	_, err := client.CreateTask(context.Background(), &eh.CreateTaskRequest{
		TenantID:      tenantIDThatNeedsEscaping,
		TaskID:        taskIDThatNeedsEscaping,
		Title:         "title",
		EnvironmentID: "env",
		Prompt:        "do",
		AssignedToAI:  true,
		Model:         &model,
		Branches:      map[string]string{},
	})
	require.NoError(t, err)
}

func TestDeleteTask(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/v1/tenants/abc/tasks/task", r.URL.Path)
		require.Equal(t, "1", r.Header.Get("If-Match"))

		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	err := client.DeleteTask(context.Background(), &eh.DeleteTaskRequest{TenantID: "abc", TaskID: "task", Version: 1})
	require.NoError(t, err)
}

func TestDeleteTaskError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()
	err := client.DeleteTask(context.Background(), &eh.DeleteTaskRequest{TenantID: "abc", TaskID: "task", Version: 1})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestDeleteTaskConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveTaskConflict()
	defer srv.Close()
	err := client.DeleteTask(context.Background(), &eh.DeleteTaskRequest{TenantID: "abc", TaskID: "task", Version: 1})
	verifyTaskConflict(t, err)
}

func TestDeleteTaskPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedTaskID, parts[5], "TaskID not properly escaped in URL path")

		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	err := client.DeleteTask(context.Background(), &eh.DeleteTaskRequest{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping, Version: 1})
	require.NoError(t, err)
}

func TestGetTask(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/tasks/task", r.URL.Path)
		require.Empty(t, r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.Task{TenantID: "abc", TaskID: "task"}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	task, err := client.GetTask(context.Background(), &eh.GetTaskRequest{TenantID: "abc", TaskID: "task"})
	require.NoError(t, err)
	require.Equal(t, "task", task.TaskID)
}

func TestGetTaskIncludeDeleted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/tasks/task", r.URL.Path)
		require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.Task{TenantID: "abc", TaskID: "task"}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	includeDeleted := true
	_, err := client.GetTask(context.Background(), &eh.GetTaskRequest{TenantID: "abc", TaskID: "task", IncludeDeleted: &includeDeleted})
	require.NoError(t, err)
}

func TestGetTaskError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(eh.Error{ResponseCode: http.StatusNotFound, Message: "nope", ErrorType: "NotFound"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetTask(context.Background(), &eh.GetTaskRequest{TenantID: "abc", TaskID: "task"})
	require.Error(t, err)
}

// nolint: dupl
func TestGetTaskPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedTaskID, parts[5], "TaskID not properly escaped in URL path")

		w.WriteHeader(http.StatusOK)
		resp := eh.Task{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetTask(context.Background(), &eh.GetTaskRequest{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping})
	require.NoError(t, err)
}

// nolint: dupl
func TestUpdateTask(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPatch, r.Method)
		require.Equal(t, "/v1/tenants/abc/tasks/task", r.URL.Path)
		require.Equal(t, "1", r.Header.Get("If-Match"))

		var reqBody eh.UpdateTaskRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.NotNil(t, reqBody.Title)
		require.Equal(t, taskTitle, *reqBody.Title)

		w.WriteHeader(http.StatusOK)
		resp := eh.Task{TenantID: "abc", TaskID: "task"}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	task, err := client.UpdateTask(context.Background(), &eh.UpdateTaskRequest{TenantID: "abc", TaskID: "task", Version: 1, Title: util.Pointer(taskTitle)})
	require.NoError(t, err)
	require.Equal(t, "task", task.TaskID)
}

func TestUpdateTaskError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()
	_, err := client.UpdateTask(context.Background(), &eh.UpdateTaskRequest{TenantID: "abc", TaskID: "task", Version: 1, Title: util.Pointer(taskTitle)})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestUpdateTaskConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveTaskConflict()
	defer srv.Close()
	_, err := client.UpdateTask(context.Background(), &eh.UpdateTaskRequest{TenantID: "abc", TaskID: "task", Version: 1, Title: util.Pointer(taskTitle)})
	verifyTaskConflict(t, err)
}

// nolint: dupl
func TestUpdateTaskPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedTaskID, parts[5], "TaskID not properly escaped in URL path")

		w.WriteHeader(http.StatusOK)
		resp := eh.Task{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.UpdateTask(context.Background(), &eh.UpdateTaskRequest{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping, Version: 1})
	require.NoError(t, err)
}

func TestCreateTurn(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/v1/tenants/abc/tasks/task1/turns/2", r.URL.Path)
		require.Equal(t, "1", r.Header.Get("If-Match"))

		var reqBody eh.CreateTurnRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.Equal(t, "prompt", reqBody.Prompt)

		w.WriteHeader(http.StatusCreated)
		resp := eh.Turn{TenantID: "abc", TaskID: "task1", TurnIndex: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	turn, err := client.CreateTurn(context.Background(), &eh.CreateTurnRequest{TenantID: "abc", TaskID: "task1", TurnIndex: 2, Prompt: "prompt", TaskVersion: 1})
	require.NoError(t, err)
	require.Equal(t, 1, turn.TurnIndex)
}

func TestCreateTurnError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()
	_, err := client.CreateTurn(context.Background(), &eh.CreateTurnRequest{TenantID: "abc", TaskID: "task1", TurnIndex: 2, Prompt: "prompt", TaskVersion: 1})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func serveTurnConflict() (*httptest.Server, *eh.Client) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(eh.ConflictError{
			ResponseCode: http.StatusConflict,
			Message:      "exists",
			ErrorType:    "Conflict",
			Current:      &eh.Turn{TurnIndex: 1, TaskID: "task1"},
		})
	})

	srv := httptest.NewServer(handler)

	client := eh.NewClient(srv.URL)
	return srv, client
}

func verifyTurnConflict(t *testing.T, err error) {
	var clientErr *eh.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.Equal(t, "exists", clientErr.Message)
	require.Equal(t, "Conflict", clientErr.ErrorType)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, eh.ObjectTypeTurn, clientErr.Current.ObjectType())
	turn, ok := clientErr.Current.(*eh.Turn)
	require.True(t, ok, "Expected Current to be of type *eh.Turn")
	require.Equal(t, eh.Turn{TurnIndex: 1, TaskID: "task1"}, *turn)
}

func TestCreateTurnConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveTurnConflict()
	defer srv.Close()

	_, err := client.CreateTurn(context.Background(), &eh.CreateTurnRequest{TenantID: "abc", TaskID: "task1", TurnIndex: 2, Prompt: "prompt", TaskVersion: 1})
	verifyTurnConflict(t, err)
}

func TestCreateTurnPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedTaskID, parts[5], "TaskID not properly escaped in URL path")

		w.WriteHeader(http.StatusCreated)
		resp := eh.Turn{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping, TurnIndex: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.CreateTurn(context.Background(), &eh.CreateTurnRequest{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping, TurnIndex: 2, Prompt: "prompt", TaskVersion: 1})
	require.NoError(t, err)
}

func TestGetTurn(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/tasks/task/turns/1", r.URL.Path)
		require.Empty(t, r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.Turn{TenantID: "abc", TaskID: "task", TurnIndex: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	turn, err := client.GetTurn(context.Background(), &eh.GetTurnRequest{TenantID: "abc", TaskID: "task", TurnIndex: 1})
	require.NoError(t, err)
	require.Equal(t, 1, turn.TurnIndex)
}

func TestGetTurnIncludeDeleted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/tasks/task/turns/1", r.URL.Path)
		require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.Turn{TenantID: "abc", TaskID: "task", TurnIndex: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	includeDeleted := true
	_, err := client.GetTurn(context.Background(), &eh.GetTurnRequest{TenantID: "abc", TaskID: "task", TurnIndex: 1, IncludeDeleted: &includeDeleted})
	require.NoError(t, err)
}

func TestGetTurnError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(eh.Error{ResponseCode: http.StatusNotFound, Message: "nope", ErrorType: "NotFound"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetTurn(context.Background(), &eh.GetTurnRequest{TenantID: "abc", TaskID: "task", TurnIndex: 1})
	require.Error(t, err)
}

func TestGetTurnPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedTaskID, parts[5], "TaskID not properly escaped in URL path")

		w.WriteHeader(http.StatusOK)
		resp := eh.Turn{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping, TurnIndex: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetTurn(context.Background(), &eh.GetTurnRequest{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping, TurnIndex: 1})
	require.NoError(t, err)
}

func TestGetLastTurn(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/tasks/task/turns/last", r.URL.Path)
		require.Empty(t, r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.Turn{TenantID: "abc", TaskID: "task", TurnIndex: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	turn, err := client.GetLastTurn(context.Background(), &eh.GetLastTurnRequest{TenantID: "abc", TaskID: "task"})
	require.NoError(t, err)
	require.Equal(t, 1, turn.TurnIndex)
}

func TestGetLastTurnIncludeDeleted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/tasks/task/turns/last", r.URL.Path)
		require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.Turn{TenantID: "abc", TaskID: "task", TurnIndex: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	includeDeleted := true
	_, err := client.GetLastTurn(context.Background(), &eh.GetLastTurnRequest{TenantID: "abc", TaskID: "task", IncludeDeleted: &includeDeleted})
	require.NoError(t, err)
}

func TestGetLastTurnError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(eh.Error{ResponseCode: http.StatusNotFound, Message: "nope", ErrorType: "NotFound"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetLastTurn(context.Background(), &eh.GetLastTurnRequest{TenantID: "abc", TaskID: "task"})
	require.Error(t, err)
}

func TestGetLastTurnPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedTaskID, parts[5], "TaskID not properly escaped in URL path")

		w.WriteHeader(http.StatusOK)
		resp := eh.Turn{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping, TurnIndex: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetLastTurn(context.Background(), &eh.GetLastTurnRequest{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping})
	require.NoError(t, err)
}

func TestGetLastTurnLog(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/tasks/task/turns/1/logs/last", r.URL.Path)
		require.Empty(t, r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.LastTurnLog{Index: 2, Timestamp: time.Unix(0, 0), Message: "msg"}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	log, err := client.GetLastTurnLog(context.Background(), &eh.GetLastTurnLogRequest{
		TenantID:  "abc",
		TaskID:    "task",
		TurnIndex: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 2, log.Index)
}

func TestGetLastTurnLogIncludeDeleted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(eh.LastTurnLog{})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	includeDeleted := true
	_, err := client.GetLastTurnLog(context.Background(), &eh.GetLastTurnLogRequest{
		TenantID:       "abc",
		TaskID:         "task",
		TurnIndex:      1,
		IncludeDeleted: &includeDeleted,
	})
	require.NoError(t, err)
}

func TestGetLastTurnLogPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 10, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3])
		require.Equal(t, escapedTaskID, parts[5])
		require.Equal(t, "1", parts[7])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(eh.LastTurnLog{})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetLastTurnLog(context.Background(), &eh.GetLastTurnLogRequest{
		TenantID:  tenantIDThatNeedsEscaping,
		TaskID:    taskIDThatNeedsEscaping,
		TurnIndex: 1,
	})
	require.NoError(t, err)
}

func TestGetLastTurnLogError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.GetLastTurnLog(context.Background(), &eh.GetLastTurnLogRequest{
		TenantID:  "abc",
		TaskID:    "task",
		TurnIndex: 1,
	})
	require.Error(t, err)
}

func TestUploadTurnLogs(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/tenants/abc/tasks/task/turns/0/logs", r.URL.Path)
		require.Equal(t, "1", r.Header.Get("If-Match"))

		var reqBody eh.UploadTurnLogsRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.Equal(t, 0, reqBody.Index)
		require.Len(t, reqBody.Logs, 1)
		require.Equal(t, "msg", reqBody.Logs[0].Message)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(eh.UploadTurnLogsResponse{Version: 2})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	logs := []eh.TurnLog{{Timestamp: time.Unix(0, 0), Message: "msg"}}
	resp, err := client.UploadTurnLogs(context.Background(), &eh.UploadTurnLogsRequest{
		TenantID:  "abc",
		TaskID:    "task",
		TurnIndex: 0,
		Version:   1,
		Index:     0,
		Logs:      logs,
	})
	require.NoError(t, err)
	require.Equal(t, 2, resp.Version)
}

func TestUploadTurnLogsError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.UploadTurnLogs(context.Background(), &eh.UploadTurnLogsRequest{
		TenantID:  "abc",
		TaskID:    "task",
		TurnIndex: 0,
		Version:   1,
		Index:     0,
	})
	require.Error(t, err)
}

func TestUploadTurnLogsConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveTurnConflict()
	defer srv.Close()

	_, err := client.UploadTurnLogs(context.Background(), &eh.UploadTurnLogsRequest{
		TenantID:  "abc",
		TaskID:    "task",
		TurnIndex: 0,
		Version:   1,
		Index:     0,
	})
	var clientErr *eh.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.Equal(t, "exists", clientErr.Message)
	require.Equal(t, "Conflict", clientErr.ErrorType)
}

func TestUploadTurnLogsPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 9, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3])
		require.Equal(t, escapedTaskID, parts[5])
		require.Equal(t, "0", parts[7])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(eh.UploadTurnLogsResponse{Version: 1})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.UploadTurnLogs(context.Background(), &eh.UploadTurnLogsRequest{
		TenantID:  tenantIDThatNeedsEscaping,
		TaskID:    taskIDThatNeedsEscaping,
		TurnIndex: 0,
		Version:   1,
		Index:     0,
	})
	require.NoError(t, err)
}

func TestListTasks(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/tasks", r.URL.Path)
		require.Equal(t, "ws", r.URL.Query().Get("workstreamID"))
		require.Equal(t, "123", r.URL.Query().Get("maxResults"))
		require.Equal(t, tokenID, r.URL.Query().Get("token"))
		require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.ListTasksResponse{Tasks: []eh.Task{{TaskID: "task"}}}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	maxResults := 123
	includeDeleted := true
	ws := "ws"
	resp, err := client.ListTasks(context.Background(), &eh.ListTasksRequest{
		TenantID:       "abc",
		WorkstreamID:   &ws,
		MaxResults:     &maxResults,
		Token:          util.Pointer(tokenID),
		IncludeDeleted: &includeDeleted,
	})
	require.NoError(t, err)
	require.Len(t, resp.Tasks, 1)
	require.Equal(t, "task", resp.Tasks[0].TaskID)
}

func TestListTasksError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListTasks(context.Background(), &eh.ListTasksRequest{TenantID: "abc"})
	require.Error(t, err)
}

func TestListTasksPathEscaping(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3])

		w.WriteHeader(http.StatusOK)
		resp := eh.ListTasksResponse{}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.ListTasks(context.Background(), &eh.ListTasksRequest{TenantID: tenantIDThatNeedsEscaping})
	require.NoError(t, err)
}

func TestListTurns(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/tasks/task1/turns", r.URL.Path)
		require.Equal(t, "20", r.URL.Query().Get("maxResults"))
		require.Equal(t, "tok", r.URL.Query().Get("token"))
		require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.ListTurnsResponse{Turns: []eh.Turn{{TurnIndex: 1}}}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	maxResults := 20
	tok := "tok"
	includeDeleted := true
	resp, err := client.ListTurns(context.Background(), &eh.ListTurnsRequest{
		TenantID:       "abc",
		TaskID:         "task1",
		MaxResults:     &maxResults,
		Token:          &tok,
		IncludeDeleted: &includeDeleted,
	})
	require.NoError(t, err)
	require.Len(t, resp.Turns, 1)
	require.Equal(t, 1, resp.Turns[0].TurnIndex)
}

func TestListTurnsError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListTurns(context.Background(), &eh.ListTurnsRequest{TenantID: "abc", TaskID: "task"})
	require.Error(t, err)
}

func TestListTurnsPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 7, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3])
		require.Equal(t, escapedTaskID, parts[5])

		w.WriteHeader(http.StatusOK)
		resp := eh.ListTurnsResponse{}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.ListTurns(context.Background(), &eh.ListTurnsRequest{
		TenantID: tenantIDThatNeedsEscaping,
		TaskID:   taskIDThatNeedsEscaping,
	})
	require.NoError(t, err)
}

func TestUpdateTurn(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPatch, r.Method)
		require.Equal(t, "/v1/tenants/abc/tasks/task1/turns/1", r.URL.Path)
		require.Equal(t, "1", r.Header.Get("If-Match"))

		var reqBody eh.UpdateTurnRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.NotNil(t, reqBody.Status)
		require.Equal(t, turnStatus, *reqBody.Status)

		w.WriteHeader(http.StatusOK)
		resp := eh.Turn{TenantID: "abc", TaskID: "task1", TurnIndex: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	status := turnStatus
	turn, err := client.UpdateTurn(context.Background(), &eh.UpdateTurnRequest{
		TenantID:  "abc",
		TaskID:    "task1",
		TurnIndex: 1,
		Version:   1,
		Status:    &status,
	})
	require.NoError(t, err)
	require.Equal(t, 1, turn.TurnIndex)
}

func TestUpdateTurnError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.UpdateTurn(context.Background(), &eh.UpdateTurnRequest{
		TenantID:  "abc",
		TaskID:    "task1",
		TurnIndex: 1,
		Version:   1,
		Status:    util.Pointer(turnStatus),
	})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestUpdateTurnConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveTurnConflict()
	defer srv.Close()

	_, err := client.UpdateTurn(context.Background(), &eh.UpdateTurnRequest{
		TenantID:  "abc",
		TaskID:    "task1",
		TurnIndex: 1,
		Version:   1,
		Status:    util.Pointer(turnStatus),
	})
	verifyTurnConflict(t, err)
}

func TestUpdateTurnPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3])
		require.Equal(t, escapedTaskID, parts[5])
		require.Equal(t, "1", parts[7])

		w.WriteHeader(http.StatusOK)
		resp := eh.Turn{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping, TurnIndex: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.UpdateTurn(context.Background(), &eh.UpdateTurnRequest{
		TenantID:  tenantIDThatNeedsEscaping,
		TaskID:    taskIDThatNeedsEscaping,
		TurnIndex: 1,
		Version:   1,
		Status:    util.Pointer(turnStatus),
	})
	require.NoError(t, err)
}

func TestStreamTurnLogs(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/tasks/task/turns/0/logs", r.URL.Path)
		require.Equal(t, "text/event-stream", r.Header.Get("Accept"))
		require.Empty(t, r.Header.Get("Last-Event-ID"))

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {}\n\n"))
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	body, err := client.StreamTurnLogs(context.Background(), &eh.StreamTurnLogsRequest{
		TenantID:  "abc",
		TaskID:    "task",
		TurnIndex: 0,
	})
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, "data: {}\n\n", string(data))
	_ = body.Close()
}

func TestStreamTurnLogsIncludeDeleted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	includeDeleted := true
	body, err := client.StreamTurnLogs(context.Background(), &eh.StreamTurnLogsRequest{
		TenantID:       "abc",
		TaskID:         "task",
		TurnIndex:      0,
		IncludeDeleted: &includeDeleted,
	})
	require.NoError(t, err)
	_ = body.Close()
}

func TestStreamTurnLogsLastEventID(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "5", r.Header.Get("Last-Event-ID"))
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	id := 5
	body, err := client.StreamTurnLogs(context.Background(), &eh.StreamTurnLogsRequest{
		TenantID:    "abc",
		TaskID:      "task",
		TurnIndex:   0,
		LastEventID: &id,
	})
	require.NoError(t, err)
	_ = body.Close()
}

func TestStreamTurnLogsNoContent(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	body, err := client.StreamTurnLogs(context.Background(), &eh.StreamTurnLogsRequest{
		TenantID:  "abc",
		TaskID:    "task",
		TurnIndex: 0,
	})
	require.NoError(t, err)
	require.Nil(t, body)
}

func TestStreamTurnLogsError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.StreamTurnLogs(context.Background(), &eh.StreamTurnLogsRequest{
		TenantID:  "abc",
		TaskID:    "task",
		TurnIndex: 0,
	})
	require.Error(t, err)
}

func TestStreamTurnLogsPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 9, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3])
		require.Equal(t, escapedTaskID, parts[5])
		require.Equal(t, "0", parts[7])

		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	body, err := client.StreamTurnLogs(context.Background(), &eh.StreamTurnLogsRequest{
		TenantID:  tenantIDThatNeedsEscaping,
		TaskID:    taskIDThatNeedsEscaping,
		TurnIndex: 0,
	})
	require.NoError(t, err)
	_ = body.Close()
}
