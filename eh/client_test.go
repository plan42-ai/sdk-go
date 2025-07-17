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
	sigv4auth "github.com/debugging-sucks/sigv4util/server/sigv4auth"
	"github.com/stretchr/testify/require"
)

const (
	expectedSigCreateTenant = "UE9TVCAvIEhUVFAvMS4xDQpIb3N0OiBzdHMudXMtd2VzdC0yLmFtYXpvbmF3cy5jb20NClVzZXItQWdlbnQ6IEdvLWh0dHAtY2xpZW50LzEuMQ0KVHJhbnNmZXItRW5jb2Rpbmc6IGNodW5rZWQNCkFjY2VwdDogYXBwbGljYXRpb24vanNvbg0KQWNjZXB0LUVuY29kaW5nOiBpZGVudGl0eQ0KQXV0aG9yaXphdGlvbjogQVdTNC1ITUFDLVNIQTI1NiBDcmVkZW50aWFsPUFLSUQvMjAyNTAxMDEvdXMtd2VzdC0yL3N0cy9hd3M0X3JlcXVlc3QsIFNpZ25lZEhlYWRlcnM9YWNjZXB0O2FjY2VwdC1lbmNvZGluZztjb250ZW50LXR5cGU7aG9zdDt4LWFtei1kYXRlO3gtYW16LXNlY3VyaXR5LXRva2VuO3gtZXZlbnQtaG9yaXpvbi1yZXF1ZXN0LWhhc2gsIFNpZ25hdHVyZT1lZmI1MGZlODE4NzRjOWFkMjI2ODJiNjAzYjI2OTkxNDQ4ZTRjNTIzODJmMTM4ZWM2NzQ1YmIzOTE0YzNiM2E1DQpDb250ZW50LVR5cGU6IGFwcGxpY2F0aW9uL3gtd3d3LWZvcm0tdXJsZW5jb2RlZA0KWC1BbXotRGF0ZTogMjAyNTAxMDFUMDAwMDAwWg0KWC1BbXotU2VjdXJpdHktVG9rZW46IFRPS0VODQpYLUV2ZW50LUhvcml6b24tUmVxdWVzdC1IYXNoOiAyOTdmNzRkYmRkY2RmZGU3NjY4OTYyZGYwM2YxMGMwZjVmYzYwNzA3YmExNGM2ZjJhNDgwMzE0ZDAxMzg5ZjZmDQoNCjJkDQpBY3Rpb249R2V0Q2FsbGVySWRlbnRpdHkmVmVyc2lvbj0yMDExLTA2LTE1DQoNCjANCg0K"
	expectedSigGetTenant    = "UE9TVCAvIEhUVFAvMS4xDQpIb3N0OiBzdHMudXMtd2VzdC0yLmFtYXpvbmF3cy5jb20NClVzZXItQWdlbnQ6IEdvLWh0dHAtY2xpZW50LzEuMQ0KVHJhbnNmZXItRW5jb2Rpbmc6IGNodW5rZWQNCkFjY2VwdDogYXBwbGljYXRpb24vanNvbg0KQWNjZXB0LUVuY29kaW5nOiBpZGVudGl0eQ0KQXV0aG9yaXphdGlvbjogQVdTNC1ITUFDLVNIQTI1NiBDcmVkZW50aWFsPUFLSUQvMjAyNTAxMDEvdXMtd2VzdC0yL3N0cy9hd3M0X3JlcXVlc3QsIFNpZ25lZEhlYWRlcnM9YWNjZXB0O2FjY2VwdC1lbmNvZGluZztjb250ZW50LXR5cGU7aG9zdDt4LWFtei1kYXRlO3gtYW16LXNlY3VyaXR5LXRva2VuO3gtZXZlbnQtaG9yaXpvbi1yZXF1ZXN0LWhhc2gsIFNpZ25hdHVyZT02OGU5ZWZhZTJkNjc0ZTc3NTViZWRiMDViNTYyOWNkZGJiY2QwMGZjYWQzMzE2Mjg5OWQ3ZGM2NzZhZWMxMDg3DQpDb250ZW50LVR5cGU6IGFwcGxpY2F0aW9uL3gtd3d3LWZvcm0tdXJsZW5jb2RlZA0KWC1BbXotRGF0ZTogMjAyNTAxMDFUMDAwMDAwWg0KWC1BbXotU2VjdXJpdHktVG9rZW46IFRPS0VODQpYLUV2ZW50LUhvcml6b24tUmVxdWVzdC1IYXNoOiBiYzhmOWZlNDM3ZjEwZmMwZWQ0YmExOWRkZjYyNmEzN2Y4NmI0Y2Y3Mzg4MTZkOGI1YTQ3ZmRmNmNjNWFiMTFlDQoNCjJkDQpBY3Rpb249R2V0Q2FsbGVySWRlbnRpdHkmVmVyc2lvbj0yMDExLTA2LTE1DQoNCjANCg0K"

	tenantIDThatNeedsEscaping = "foo/../../bar"
	escapedTenantID           = "foo%2F..%2F..%2Fbar"

	tokenIDThatNeedsEscaping = "tok/../../id"       // #nosec G101: This is not a credential.
	escapedTokenID           = "tok%2F..%2F..%2Fid" // #nosec G101: This is not a credential.
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
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(eh.Error{ResponseCode: http.StatusBadRequest, Message: "bad", ErrorType: "BadRequest"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
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
}

func TestListPolicies(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/policies", r.URL.Path)
		require.Equal(t, "123", r.URL.Query().Get("maxResults"))
		require.Equal(t, "tok", r.URL.Query().Get("token"))

		w.WriteHeader(http.StatusOK)
		resp := eh.ListPoliciesResponse{Policies: []eh.Policy{{Name: "p"}}}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	maxResults := 123
	tok := "tok"
	resp, err := client.ListPolicies(context.Background(), &eh.ListPoliciesRequest{TenantID: "abc", MaxResults: &maxResults, Token: &tok})
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	require.Equal(t, "p", resp.Policies[0].Name)
}

func TestListPoliciesError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(eh.Error{ResponseCode: http.StatusBadRequest, Message: "bad", ErrorType: "BadRequest"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
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
	resp, err := client.GenerateWebUIToken(context.Background(), &eh.GenerateWebUITokenRequest{TenantID: "abc", TokenID: "tok"})
	require.NoError(t, err)
	require.Equal(t, "jwt", resp.JWT)
}

func TestGenerateWebUITokenError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(eh.Error{ResponseCode: http.StatusBadRequest, Message: "bad", ErrorType: "BadRequest"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GenerateWebUIToken(context.Background(), &eh.GenerateWebUITokenRequest{TenantID: "abc", TokenID: "tok"})
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
