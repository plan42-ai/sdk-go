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

	tokenID         = "tok"
	taskTitle       = "new"
	turnStatus      = "Done"
	githubUserLogin = "octocat"
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

func TestListTenants(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants", r.URL.Path)
		require.Equal(t, "123", r.URL.Query().Get("maxResults"))
		require.Equal(t, tokenID, r.URL.Query().Get("token"))

		w.WriteHeader(http.StatusOK)
		resp := eh.List[*eh.Tenant]{Items: []*eh.Tenant{{TenantID: "abc"}}}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	maxResults := 123
	resp, err := client.ListTenants(context.Background(), &eh.ListTenantsRequest{MaxResults: &maxResults, Token: util.Pointer(tokenID)})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	require.Equal(t, "abc", resp.Items[0].TenantID)
}

func TestListTenantsError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListTenants(context.Background(), &eh.ListTenantsRequest{})
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

func serveRunnerConflict() (*httptest.Server, *eh.Client) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(eh.ConflictError{
			ResponseCode: http.StatusConflict,
			Message:      "exists",
			ErrorType:    "Conflict",
			Current:      &eh.Runner{RunnerID: "runner1"},
		})
	})

	srv := httptest.NewServer(handler)

	client := eh.NewClient(srv.URL)
	return srv, client
}

func verifyRunnerConflict(t *testing.T, err error) {
	var clientErr *eh.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.Equal(t, "exists", clientErr.Message)
	require.Equal(t, "Conflict", clientErr.ErrorType)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, eh.ObjectTypeRunner, clientErr.Current.ObjectType())
	runner, ok := clientErr.Current.(*eh.Runner)
	require.True(t, ok, "Expected Current to be of type *eh.Runner")
	require.Equal(t, "runner1", runner.RunnerID)
}

func serveTaskConflict() (*httptest.Server, *eh.Client) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(eh.ConflictError{
			ResponseCode: http.StatusConflict,
			Message:      "exists",
			ErrorType:    "Conflict",
			Current: &eh.WorkstreamTaskConflict{
				Task:       &eh.Task{TaskID: "task"},
				Workstream: &eh.Workstream{WorkstreamID: "ws"},
			},
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
	require.Equal(t, eh.ObjectTypeWorkstreamTaskConflict, clientErr.Current.ObjectType())
	conflict, ok := clientErr.Current.(*eh.WorkstreamTaskConflict)
	require.True(t, ok, "Expected Current to be of type *eh.WorkstreamTaskConflict")
	require.NotNil(t, conflict.Task)
	require.Equal(t, "task", conflict.Task.TaskID)
	require.NotNil(t, conflict.Workstream)
	require.Equal(t, "ws", conflict.Workstream.WorkstreamID)
}

func serveGithubOrgConflict() (*httptest.Server, *eh.Client) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(eh.ConflictError{
			ResponseCode: http.StatusConflict,
			Message:      "exists",
			ErrorType:    "Conflict",
			Current:      &eh.GithubOrg{OrgID: "org"},
		})
	})

	srv := httptest.NewServer(handler)
	client := eh.NewClient(srv.URL)
	return srv, client
}

func verifyGithubOrgConflict(t *testing.T, err error) {
	var clientErr *eh.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.Equal(t, "exists", clientErr.Message)
	require.Equal(t, "Conflict", clientErr.ErrorType)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, eh.ObjectTypeGithubOrg, clientErr.Current.ObjectType())
	org, ok := clientErr.Current.(*eh.GithubOrg)
	require.True(t, ok, "Expected Current to be of type *eh.GithubOrg")
	require.Equal(t, eh.GithubOrg{OrgID: "org"}, *org)
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

func TestCreateRunner(t *testing.T) {
	t.Parallel()

	description := "runner-desc"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/v1/tenants/abc/runners/runner1", r.URL.Path)

		var reqBody eh.CreateRunnerRequest
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
		resp := eh.Runner{
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
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	runner, err := client.CreateRunner(context.Background(), &eh.CreateRunnerRequest{
		TenantID:      "abc",
		RunnerID:      "runner1",
		Name:          "runner-name",
		Description:   &description,
		IsCloud:       true,
		RunsTasks:     true,
		ProxiesGithub: true,
	})
	require.NoError(t, err)
	require.Equal(t, "runner1", runner.RunnerID)
	require.NotNil(t, runner.Description)
	require.Equal(t, description, *runner.Description)
}

func TestCreateRunnerError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.CreateRunner(context.Background(), &eh.CreateRunnerRequest{TenantID: "abc", RunnerID: "runner1", Name: "runner-name"})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestCreateRunnerConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveRunnerConflict()
	defer srv.Close()

	_, err := client.CreateRunner(context.Background(), &eh.CreateRunnerRequest{TenantID: "abc", RunnerID: "runner1", Name: "runner-name"})
	verifyRunnerConflict(t, err)
}

func TestCreateRunnerPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedRunnerID, parts[5], "RunnerID not properly escaped in URL path")

		w.WriteHeader(http.StatusCreated)
		now := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
		resp := eh.Runner{
			TenantID:  tenantIDThatNeedsEscaping,
			RunnerID:  runnerIDThatNeedsEscaping,
			Name:      "runner",
			CreatedAt: now,
			UpdatedAt: now,
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.CreateRunner(context.Background(), &eh.CreateRunnerRequest{TenantID: tenantIDThatNeedsEscaping, RunnerID: runnerIDThatNeedsEscaping, Name: "runner"})
	require.NoError(t, err)
}

func TestDeleteRunner(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/v1/tenants/abc/runners/runner1", r.URL.Path)
		require.Equal(t, "1", r.Header.Get("If-Match"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))

		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	err := client.DeleteRunner(context.Background(), &eh.DeleteRunnerRequest{
		TenantID: "abc",
		RunnerID: "runner1",
		Version:  1,
	})
	require.NoError(t, err)
}

func TestDeleteRunnerError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	err := client.DeleteRunner(context.Background(), &eh.DeleteRunnerRequest{
		TenantID: "abc",
		RunnerID: "runner1",
		Version:  1,
	})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestDeleteRunnerConflictError(t *testing.T) {
	t.Parallel()

	srv, client := serveRunnerConflict()
	defer srv.Close()

	err := client.DeleteRunner(context.Background(), &eh.DeleteRunnerRequest{
		TenantID: "abc",
		RunnerID: "runner1",
		Version:  1,
	})
	verifyRunnerConflict(t, err)
}

func TestDeleteRunnerPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedRunnerID, parts[5], "RunnerID not properly escaped in URL path")

		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	err := client.DeleteRunner(context.Background(), &eh.DeleteRunnerRequest{
		TenantID: tenantIDThatNeedsEscaping,
		RunnerID: runnerIDThatNeedsEscaping,
		Version:  1,
	})
	require.NoError(t, err)
}

func TestGetRunner(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/runners/runner", r.URL.Path)

		w.WriteHeader(http.StatusOK)
		resp := eh.Runner{TenantID: "abc", RunnerID: "runner"}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	runner, err := client.GetRunner(context.Background(), &eh.GetRunnerRequest{TenantID: "abc", RunnerID: "runner"})
	require.NoError(t, err)
	require.Equal(t, "runner", runner.RunnerID)
}

func TestGetRunnerError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(eh.Error{ResponseCode: http.StatusNotFound, Message: "nope", ErrorType: "NotFound"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetRunner(context.Background(), &eh.GetRunnerRequest{TenantID: "abc", RunnerID: "runner"})
	require.Error(t, err)
}

// nolint:dupl
func TestGetRunnerPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedRunnerID, parts[5], "RunnerID not properly escaped in URL path")

		w.WriteHeader(http.StatusOK)
		resp := eh.Runner{TenantID: tenantIDThatNeedsEscaping, RunnerID: runnerIDThatNeedsEscaping}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetRunner(context.Background(), &eh.GetRunnerRequest{TenantID: tenantIDThatNeedsEscaping, RunnerID: runnerIDThatNeedsEscaping})
	require.NoError(t, err)
}

func TestUpdateRunner(t *testing.T) {
	t.Parallel()

	name := "runner-updated"
	description := "runner-desc"
	isCloud := true
	runsTasks := false
	proxiesGithub := true

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPatch, r.Method)
		require.Equal(t, "/v1/tenants/abc/runners/runner1", r.URL.Path)
		require.Equal(t, "2", r.Header.Get("If-Match"))

		var reqBody eh.UpdateRunnerRequest
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
		resp := eh.Runner{
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
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	runner, err := client.UpdateRunner(context.Background(), &eh.UpdateRunnerRequest{
		TenantID:      "abc",
		RunnerID:      "runner1",
		Version:       2,
		Name:          &name,
		Description:   &description,
		IsCloud:       &isCloud,
		RunsTasks:     &runsTasks,
		ProxiesGithub: &proxiesGithub,
	})
	require.NoError(t, err)
	require.Equal(t, "runner1", runner.RunnerID)
	require.NotNil(t, runner.Description)
	require.Equal(t, description, *runner.Description)
}

func TestUpdateRunnerError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.UpdateRunner(context.Background(), &eh.UpdateRunnerRequest{TenantID: "abc", RunnerID: "runner1", Version: 1})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestUpdateRunnerConflictError(t *testing.T) {
	t.Parallel()

	srv, client := serveRunnerConflict()
	defer srv.Close()

	_, err := client.UpdateRunner(context.Background(), &eh.UpdateRunnerRequest{TenantID: "abc", RunnerID: "runner1", Version: 1})
	verifyRunnerConflict(t, err)
}

func TestUpdateRunnerPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedRunnerID, parts[5], "RunnerID not properly escaped in URL path")
		require.Equal(t, "1", r.Header.Get("If-Match"))

		w.WriteHeader(http.StatusOK)
		now := time.Date(2024, time.January, 3, 0, 0, 0, 0, time.UTC)
		resp := eh.Runner{
			TenantID:  tenantIDThatNeedsEscaping,
			RunnerID:  runnerIDThatNeedsEscaping,
			Name:      "runner",
			CreatedAt: now,
			UpdatedAt: now,
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	isCloud := true
	_, err := client.UpdateRunner(context.Background(), &eh.UpdateRunnerRequest{
		TenantID: tenantIDThatNeedsEscaping,
		RunnerID: runnerIDThatNeedsEscaping,
		Version:  1,
		IsCloud:  &isCloud,
	})
	require.NoError(t, err)
}

// nolint: dupl
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

// nolint: dupl
func TestListEnvironments(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/environments", r.URL.Path)
		require.Equal(t, "123", r.URL.Query().Get("maxResults"))
		require.Equal(t, tokenID, r.URL.Query().Get("token"))
		require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.List[eh.Environment]{Items: []eh.Environment{{EnvironmentID: "env"}}}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	maxResults := 123
	includeDeleted := true
	resp, err := client.ListEnvironments(context.Background(), &eh.ListEnvironmentsRequest{TenantID: "abc", MaxResults: &maxResults, Token: util.Pointer(tokenID), IncludeDeleted: &includeDeleted})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	require.Equal(t, "env", resp.Items[0].EnvironmentID)
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
		resp := eh.List[eh.Environment]{}
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
		EnvironmentID: util.Pointer("env"),
		Prompt:        "do",
		Model:         &model,
		RepoInfo:      map[string]*eh.RepoInfo{},
	})
	require.NoError(t, err)
	require.Equal(t, "task", task.TaskID)
}

func TestCreateWorkstreamTask(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/v1/tenants/abc/workstreams/ws/tasks/task", r.URL.Path)

		var reqBody eh.CreateWorkstreamTaskRequest
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
		require.Equal(t, eh.ModelTypeCodexMini, *reqBody.Model)
		require.Nil(t, reqBody.AssignedToTenantID)
		require.True(t, reqBody.AssignedToAI)
		require.NotNil(t, reqBody.State)
		require.Equal(t, eh.TaskStatePending, *reqBody.State)
		require.Contains(t, reqBody.RepoInfo, "repo")
		repo := reqBody.RepoInfo["repo"]
		require.NotNil(t, repo)
		require.Equal(t, "feature", repo.FeatureBranch)
		require.Equal(t, "main", repo.TargetBranch)

		w.WriteHeader(http.StatusCreated)
		resp := eh.Task{TenantID: "abc", TaskID: "task"}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	model := eh.ModelTypeCodexMini
	parallel := true
	state := eh.TaskStatePending
	task, err := client.CreateWorkstreamTask(context.Background(), &eh.CreateWorkstreamTaskRequest{
		TenantID:      "abc",
		WorkstreamID:  "ws",
		TaskID:        "task",
		Title:         "title",
		EnvironmentID: util.Pointer("env"),
		Prompt:        util.Pointer("do"),
		Parallel:      &parallel,
		Model:         &model,
		AssignedToAI:  true,
		RepoInfo: map[string]*eh.RepoInfo{
			"repo": {
				FeatureBranch: "feature",
				TargetBranch:  "main",
			},
		},
		State: &state,
	})
	require.NoError(t, err)
	require.Equal(t, "task", task.TaskID)
}

func TestCreateWorkstreamTaskError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.CreateWorkstreamTask(context.Background(), &eh.CreateWorkstreamTaskRequest{
		TenantID:     "abc",
		WorkstreamID: "ws",
		TaskID:       "task",
		Title:        "title",
		AssignedToAI: true,
	})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestCreateWorkstreamTaskConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveTaskConflict()
	defer srv.Close()

	_, err := client.CreateWorkstreamTask(context.Background(), &eh.CreateWorkstreamTaskRequest{
		TenantID:     "abc",
		WorkstreamID: "ws",
		TaskID:       "task",
		Title:        "title",
		AssignedToAI: true,
	})
	verifyTaskConflict(t, err)
}

func TestCreateWorkstreamTaskPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedWorkstreamID, parts[5], "WorkstreamID not properly escaped in URL path")
		require.Equal(t, escapedTaskID, parts[7], "TaskID not properly escaped in URL path")

		w.WriteHeader(http.StatusCreated)
		resp := eh.Task{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	parallel := true
	_, err := client.CreateWorkstreamTask(context.Background(), &eh.CreateWorkstreamTaskRequest{
		TenantID:     tenantIDThatNeedsEscaping,
		WorkstreamID: workstreamIDThatNeedsEscaping,
		TaskID:       taskIDThatNeedsEscaping,
		Title:        "title",
		Parallel:     &parallel,
		AssignedToAI: true,
	})
	require.NoError(t, err)
}

// nolint:dupl
func TestUpdateWorkstreamTask(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPatch, r.Method)
		require.Equal(t, "/v1/tenants/abc/workstreams/ws/tasks/task", r.URL.Path)
		require.Equal(t, "3", r.Header.Get("If-Match"))

		var reqBody eh.UpdateWorkstreamTaskRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.NotNil(t, reqBody.Title)
		require.Equal(t, taskTitle, *reqBody.Title)
		require.NotNil(t, reqBody.EnvironmentID)
		require.NotNil(t, *reqBody.EnvironmentID)
		require.Equal(t, "env", **reqBody.EnvironmentID)
		require.NotNil(t, reqBody.Prompt)
		require.Equal(t, "prompt", *reqBody.Prompt)
		require.NotNil(t, reqBody.Parallel)
		require.True(t, *reqBody.Parallel)
		require.NotNil(t, reqBody.Model)
		require.Equal(t, eh.ModelTypeCodexMini, *reqBody.Model)
		require.NotNil(t, reqBody.AssignedToTenantID)
		require.NotNil(t, *reqBody.AssignedToTenantID)
		require.Equal(t, "tenant-b", **reqBody.AssignedToTenantID)
		require.NotNil(t, reqBody.AssignedToAI)
		require.False(t, *reqBody.AssignedToAI)
		require.NotNil(t, reqBody.RepoInfo)
		require.Contains(t, *reqBody.RepoInfo, "repo")
		repo := (*reqBody.RepoInfo)["repo"]
		require.NotNil(t, repo)
		require.Equal(t, "feature", repo.FeatureBranch)
		require.Equal(t, "main", repo.TargetBranch)
		require.NotNil(t, reqBody.State)
		require.Equal(t, eh.TaskStateExecuting, *reqBody.State)
		require.NotNil(t, reqBody.BeforeTaskID)
		require.Equal(t, "before", *reqBody.BeforeTaskID)
		require.Nil(t, reqBody.AfterTaskID)
		require.NotNil(t, reqBody.Deleted)
		require.True(t, *reqBody.Deleted)

		w.WriteHeader(http.StatusOK)
		resp := eh.Task{TenantID: "abc", TaskID: "task"}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	title := util.Pointer(taskTitle)
	env := util.Pointer("env")
	prompt := util.Pointer("prompt")
	parallel := true
	model := eh.ModelTypeCodexMini
	assignedTenant := util.Pointer("tenant-b")
	assignedToAI := false
	repoInfo := map[string]*eh.RepoInfo{
		"repo": {
			FeatureBranch: "feature",
			TargetBranch:  "main",
		},
	}
	state := eh.TaskStateExecuting
	before := util.Pointer("before")
	deleted := true

	task, err := client.UpdateWorkstreamTask(context.Background(), &eh.UpdateWorkstreamTaskRequest{
		TenantID:           "abc",
		WorkstreamID:       "ws",
		TaskID:             "task",
		Version:            3,
		Title:              title,
		EnvironmentID:      &env,
		Prompt:             prompt,
		Parallel:           &parallel,
		Model:              &model,
		AssignedToTenantID: &assignedTenant,
		AssignedToAI:       util.Pointer(assignedToAI),
		RepoInfo:           &repoInfo,
		State:              &state,
		BeforeTaskID:       before,
		Deleted:            &deleted,
	})
	require.NoError(t, err)
	require.Equal(t, "task", task.TaskID)
}

func TestUpdateWorkstreamTaskError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.UpdateWorkstreamTask(context.Background(), &eh.UpdateWorkstreamTaskRequest{
		TenantID:     "abc",
		WorkstreamID: "ws",
		TaskID:       "task",
		Version:      1,
		Title:        util.Pointer(taskTitle),
	})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestUpdateWorkstreamTaskConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveTaskConflict()
	defer srv.Close()

	_, err := client.UpdateWorkstreamTask(context.Background(), &eh.UpdateWorkstreamTaskRequest{
		TenantID:     "abc",
		WorkstreamID: "ws",
		TaskID:       "task",
		Version:      1,
		Title:        util.Pointer(taskTitle),
	})
	verifyTaskConflict(t, err)
}

// nolint:dupl
func TestUpdateWorkstreamTaskPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedWorkstreamID, parts[5], "WorkstreamID not properly escaped in URL path")
		require.Equal(t, escapedTaskID, parts[7], "TaskID not properly escaped in URL path")

		w.WriteHeader(http.StatusOK)
		resp := eh.Task{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.UpdateWorkstreamTask(context.Background(), &eh.UpdateWorkstreamTaskRequest{
		TenantID:     tenantIDThatNeedsEscaping,
		WorkstreamID: workstreamIDThatNeedsEscaping,
		TaskID:       taskIDThatNeedsEscaping,
		Version:      1,
		Title:        util.Pointer("title"),
	})
	require.NoError(t, err)
}

func TestDeleteWorkstreamTask(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/v1/tenants/abc/workstreams/ws/tasks/task", r.URL.Path)
		require.Equal(t, "1", r.Header.Get("If-Match"))

		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	err := client.DeleteWorkstreamTask(context.Background(), &eh.DeleteWorkstreamTaskRequest{
		TenantID:     "abc",
		WorkstreamID: "ws",
		TaskID:       "task",
		Version:      1,
	})
	require.NoError(t, err)
}

func TestDeleteWorkstreamTaskError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	err := client.DeleteWorkstreamTask(context.Background(), &eh.DeleteWorkstreamTaskRequest{
		TenantID:     "abc",
		WorkstreamID: "ws",
		TaskID:       "task",
		Version:      1,
	})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
	require.Equal(t, "BadRequest", clientErr.ErrorType)
}

func TestDeleteWorkstreamTaskConflictError(t *testing.T) {
	t.Parallel()

	srv, client := serveTaskConflict()
	defer srv.Close()

	err := client.DeleteWorkstreamTask(context.Background(), &eh.DeleteWorkstreamTaskRequest{
		TenantID:     "abc",
		WorkstreamID: "ws",
		TaskID:       "task",
		Version:      1,
	})
	verifyTaskConflict(t, err)
}

func TestDeleteWorkstreamTaskPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedWorkstreamID, parts[5], "WorkstreamID not properly escaped in URL path")
		require.Equal(t, escapedTaskID, parts[7], "TaskID not properly escaped in URL path")

		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	err := client.DeleteWorkstreamTask(context.Background(), &eh.DeleteWorkstreamTaskRequest{
		TenantID:     tenantIDThatNeedsEscaping,
		WorkstreamID: workstreamIDThatNeedsEscaping,
		TaskID:       taskIDThatNeedsEscaping,
		Version:      1,
	})
	require.NoError(t, err)
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
		EnvironmentID: util.Pointer("env"),
		Prompt:        "do",
		Model:         &model,
		RepoInfo:      map[string]*eh.RepoInfo{},
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
		EnvironmentID: util.Pointer("env"),
		Prompt:        "do",
		Model:         &model,
		RepoInfo:      map[string]*eh.RepoInfo{},
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
		EnvironmentID: util.Pointer("env"),
		Prompt:        "do",
		Model:         &model,
		RepoInfo:      map[string]*eh.RepoInfo{},
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

// nolint: dupl
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

func TestGetWorkstreamTask(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/workstreams/ws/tasks/task", r.URL.Path)
		require.Empty(t, r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.Task{TenantID: "abc", TaskID: "task", WorkstreamID: util.Pointer("ws")}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetWorkstreamTask(context.Background(), &eh.GetWorkstreamTaskRequest{TenantID: "abc", WorkstreamID: "ws", TaskID: "task"})
	require.NoError(t, err)
}

func TestGetWorkstreamTaskIncludeDeleted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/workstreams/ws/tasks/task", r.URL.Path)
		require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.Task{TenantID: "abc", TaskID: "task", WorkstreamID: util.Pointer("ws")}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	includeDeleted := true
	_, err := client.GetWorkstreamTask(
		context.Background(),
		&eh.GetWorkstreamTaskRequest{TenantID: "abc", WorkstreamID: "ws", TaskID: "task", IncludeDeleted: &includeDeleted},
	)
	require.NoError(t, err)
}

func TestGetWorkstreamTaskError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(eh.Error{ResponseCode: http.StatusNotFound, Message: "nope", ErrorType: "NotFound"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetWorkstreamTask(context.Background(), &eh.GetWorkstreamTaskRequest{TenantID: "abc", WorkstreamID: "ws", TaskID: "task"})
	require.Error(t, err)
}

func TestGetWorkstreamTaskPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 8, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedWorkstreamID, parts[5], "WorkstreamID not properly escaped in URL path")
		require.Equal(t, escapedTaskID, parts[7], "TaskID not properly escaped in URL path")

		w.WriteHeader(http.StatusOK)
		resp := eh.Task{TenantID: tenantIDThatNeedsEscaping, TaskID: taskIDThatNeedsEscaping, WorkstreamID: util.Pointer(workstreamIDThatNeedsEscaping)}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetWorkstreamTask(
		context.Background(),
		&eh.GetWorkstreamTaskRequest{
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

// nolint: dupl
func TestListTasks(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/tasks", r.URL.Path)
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
	resp, err := client.ListTasks(context.Background(), &eh.ListTasksRequest{
		TenantID:       "abc",
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

func TestListWorkstreamTasks(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/workstreams/ws/tasks", r.URL.Path)
		require.Equal(t, "100", r.URL.Query().Get("maxResults"))
		require.Equal(t, tokenID, r.URL.Query().Get("token"))
		require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.List[eh.Task]{Items: []eh.Task{{TaskID: "task"}}}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	maxResults := 100
	includeDeleted := true
	resp, err := client.ListWorkstreamTasks(context.Background(), &eh.ListWorkstreamTasksRequest{
		TenantID:       "abc",
		WorkstreamID:   "ws",
		MaxResults:     &maxResults,
		Token:          util.Pointer(tokenID),
		IncludeDeleted: &includeDeleted,
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	require.Equal(t, "task", resp.Items[0].TaskID)
}

func TestListWorkstreamTasksError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListWorkstreamTasks(context.Background(), &eh.ListWorkstreamTasksRequest{TenantID: "abc", WorkstreamID: "ws"})
	require.Error(t, err)
}

func TestListWorkstreamTasksPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 7, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")
		require.Equal(t, escapedWorkstreamID, parts[5], "WorkstreamID not properly escaped in URL path")

		w.WriteHeader(http.StatusOK)
		resp := eh.List[eh.Task]{}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.ListWorkstreamTasks(context.Background(), &eh.ListWorkstreamTasksRequest{
		TenantID:     tenantIDThatNeedsEscaping,
		WorkstreamID: workstreamIDThatNeedsEscaping,
	})
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

func TestCreateGithubConnection(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	tokenExpiry := now.Add(time.Hour)
	stateExpiry := now.Add(2 * time.Hour)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/v1/tenants/abc/github-connections/conn", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))

		var reqBody eh.CreateGithubConnectionRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.False(t, reqBody.Private)
		require.Nil(t, reqBody.RunnerID)
		require.NotNil(t, reqBody.GithubUserLogin)
		require.Equal(t, githubUserLogin, *reqBody.GithubUserLogin)
		require.NotNil(t, reqBody.GithubUserID)
		require.Equal(t, 123, *reqBody.GithubUserID)

		w.WriteHeader(http.StatusCreated)
		resp := eh.GithubConnection{
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
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	login := githubUserLogin
	userID := 123

	resp, err := client.CreateGithubConnection(context.Background(), &eh.CreateGithubConnectionRequest{
		TenantID:        "abc",
		ConnectionID:    "conn",
		GithubUserLogin: &login,
		GithubUserID:    &userID,
	})
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
	_, err := client.CreateGithubConnection(context.Background(), &eh.CreateGithubConnectionRequest{
		TenantID:        "abc",
		ConnectionID:    "conn",
		GithubUserLogin: &login,
		GithubUserID:    &userID,
	})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
}

func TestCreateGithubConnectionConflictError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(eh.ConflictError{
			ResponseCode: http.StatusConflict,
			Message:      "exists",
			ErrorType:    "Conflict",
			Current:      &eh.GithubConnection{ConnectionID: "conn"},
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	login := githubUserLogin
	userID := 123

	_, err := client.CreateGithubConnection(context.Background(), &eh.CreateGithubConnectionRequest{
		TenantID:        "abc",
		ConnectionID:    "conn",
		GithubUserLogin: &login,
		GithubUserID:    &userID,
	})
	var clientErr *eh.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, eh.ObjectTypeGithubConnection, clientErr.Current.ObjectType())
	connection, ok := clientErr.Current.(*eh.GithubConnection)
	require.True(t, ok, "Expected Current to be of type *eh.GithubConnection")
	require.Equal(t, "conn", connection.ConnectionID)
}

func TestCreateGithubConnectionPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3])
		require.Equal(t, escapedGithubConnectionID, parts[5])

		w.WriteHeader(http.StatusCreated)
		now := time.Now().UTC()
		resp := eh.GithubConnection{
			TenantID:     tenantIDThatNeedsEscaping,
			ConnectionID: githubConnectionIDThatNeedsEscaping,
			CreatedAt:    now,
			UpdatedAt:    now,
			Version:      1,
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.CreateGithubConnection(context.Background(), &eh.CreateGithubConnectionRequest{
		TenantID:        tenantIDThatNeedsEscaping,
		ConnectionID:    githubConnectionIDThatNeedsEscaping,
		GithubUserLogin: util.Pointer(githubUserLogin),
		GithubUserID:    util.Pointer(123),
	})
	require.NoError(t, err)
}

func TestCreateGithubConnectionPrivateValidation(t *testing.T) {
	t.Parallel()

	client := eh.NewClient("https://api.example.com")

	_, err := client.CreateGithubConnection(context.Background(), &eh.CreateGithubConnectionRequest{
		TenantID:     "abc",
		ConnectionID: "conn",
		Private:      true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "runner id is required when private is true")

	_, err = client.CreateGithubConnection(context.Background(), &eh.CreateGithubConnectionRequest{
		TenantID:        "abc",
		ConnectionID:    "conn",
		Private:         true,
		RunnerID:        util.Pointer("runner"),
		GithubUserLogin: util.Pointer(githubUserLogin),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "github user login must be nil when private is true")

	_, err = client.CreateGithubConnection(context.Background(), &eh.CreateGithubConnectionRequest{
		TenantID:     "abc",
		ConnectionID: "conn",
		Private:      true,
		RunnerID:     util.Pointer("runner"),
		GithubUserID: util.Pointer(123),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "github user id must be nil when private is true")
}

func TestListGithubConnections(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/github-connections", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Accept"))
		require.Equal(t, "10", r.URL.Query().Get("maxResults"))
		require.Equal(t, tokenID, r.URL.Query().Get("token"))

		w.WriteHeader(http.StatusOK)
		resp := eh.List[eh.GithubConnection]{
			Items: []eh.GithubConnection{{ConnectionID: "conn-1"}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	maxResults := 10
	resp, err := client.ListGithubConnections(context.Background(), &eh.ListGithubConnectionsRequest{
		TenantID:   "abc",
		MaxResults: &maxResults,
		Token:      util.Pointer(tokenID),
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	require.Equal(t, "conn-1", resp.Items[0].ConnectionID)
}

func TestListGithubConnectionsError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListGithubConnections(context.Background(), &eh.ListGithubConnectionsRequest{TenantID: "abc"})
	require.Error(t, err)
}

func TestListGithubConnectionsPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3])
		require.Equal(t, "github-connections", parts[4])

		w.WriteHeader(http.StatusOK)
		resp := eh.List[eh.GithubConnection]{Items: []eh.GithubConnection{}}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.ListGithubConnections(context.Background(), &eh.ListGithubConnectionsRequest{
		TenantID: tenantIDThatNeedsEscaping,
	})
	require.NoError(t, err)
}

func TestAddGithubOrg(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/v1/github/orgs/abc", r.URL.Path)

		var reqBody eh.AddGithubOrgRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.Equal(t, "MyOrg", reqBody.OrgName)
		require.Equal(t, 123, reqBody.ExternalOrgID)
		require.Equal(t, 456, reqBody.InstallationID)

		w.WriteHeader(http.StatusCreated)
		resp := eh.GithubOrg{OrgID: "abc", OrgName: "MyOrg", ExternalOrgID: 123, InstallationID: 456, Version: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	org, err := client.AddGithubOrg(context.Background(), &eh.AddGithubOrgRequest{
		OrgID:          "abc",
		OrgName:        "MyOrg",
		ExternalOrgID:  123,
		InstallationID: 456,
	})
	require.NoError(t, err)
	require.Equal(t, "abc", org.OrgID)
}

func TestAddGithubOrgError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.AddGithubOrg(context.Background(), &eh.AddGithubOrgRequest{
		OrgID:          "abc",
		OrgName:        "MyOrg",
		ExternalOrgID:  123,
		InstallationID: 456,
	})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func TestAddGithubOrgPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedGithubOrgID, parts[4])

		w.WriteHeader(http.StatusCreated)
		resp := eh.GithubOrg{OrgID: githubOrgIDThatNeedsEscaping, OrgName: "name", ExternalOrgID: 1, InstallationID: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.AddGithubOrg(context.Background(), &eh.AddGithubOrgRequest{
		OrgID:          githubOrgIDThatNeedsEscaping,
		OrgName:        "name",
		ExternalOrgID:  1,
		InstallationID: 1,
	})
	require.NoError(t, err)
}

// nolint: dupl
func TestListGithubOrgs(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/github/orgs", r.URL.Path)
		require.Equal(t, "123", r.URL.Query().Get("maxResults"))
		require.Equal(t, tokenID, r.URL.Query().Get("token"))
		require.Empty(t, r.URL.Query().Get("name"))
		require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.ListGithubOrgsResponse{Orgs: []eh.GithubOrg{{OrgID: "org"}}}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	maxResults := 123
	includeDeleted := true
	resp, err := client.ListGithubOrgs(context.Background(), &eh.ListGithubOrgsRequest{MaxResults: &maxResults, Token: util.Pointer(tokenID), IncludeDeleted: &includeDeleted})
	require.NoError(t, err)
	require.Len(t, resp.Orgs, 1)
	require.Equal(t, "org", resp.Orgs[0].OrgID)
}

func TestListGithubOrgsWithName(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/github/orgs", r.URL.Path)
		require.Equal(t, "the-name", r.URL.Query().Get("name"))
		require.Empty(t, r.URL.Query().Get("maxResults"))
		require.Empty(t, r.URL.Query().Get("token"))
		require.Empty(t, r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.ListGithubOrgsResponse{Orgs: []eh.GithubOrg{{OrgID: "org"}}}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	name := "the-name"
	resp, err := client.ListGithubOrgs(context.Background(), &eh.ListGithubOrgsRequest{Name: &name})
	require.NoError(t, err)
	require.Len(t, resp.Orgs, 1)
	require.Equal(t, "org", resp.Orgs[0].OrgID)
}

func TestListGithubOrgsError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListGithubOrgs(context.Background(), &eh.ListGithubOrgsRequest{})
	require.Error(t, err)
}

func TestGetGithubOrg(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/github/orgs/abc", r.URL.Path)
		require.Empty(t, r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.GithubOrg{OrgID: "abc", OrgName: "name", ExternalOrgID: 1, InstallationID: 2}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	org, err := client.GetGithubOrg(context.Background(), &eh.GetGithubOrgRequest{OrgID: "abc"})
	require.NoError(t, err)
	require.Equal(t, "abc", org.OrgID)
}

func TestGetGithubOrgIncludeDeleted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/github/orgs/abc", r.URL.Path)
		require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.GithubOrg{OrgID: "abc"}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	includeDeleted := true
	_, err := client.GetGithubOrg(context.Background(), &eh.GetGithubOrgRequest{OrgID: "abc", IncludeDeleted: &includeDeleted})
	require.NoError(t, err)
}

func TestGetGithubOrgError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.GetGithubOrg(context.Background(), &eh.GetGithubOrgRequest{OrgID: "abc"})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

// nolint: dupl
func TestGetGithubOrgPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedGithubOrgID, parts[4])

		w.WriteHeader(http.StatusOK)
		resp := eh.GithubOrg{OrgID: githubOrgIDThatNeedsEscaping}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetGithubOrg(context.Background(), &eh.GetGithubOrgRequest{OrgID: githubOrgIDThatNeedsEscaping})
	require.NoError(t, err)
}

func TestUpdateGithubOrg(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPatch, r.Method)
		require.Equal(t, "/v1/github/orgs/abc", r.URL.Path)
		require.Equal(t, "1", r.Header.Get("If-Match"))

		var reqBody eh.UpdateGithubOrgRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.NotNil(t, reqBody.OrgName)
		require.Equal(t, "MyOrg", *reqBody.OrgName)

		w.WriteHeader(http.StatusOK)
		resp := eh.GithubOrg{OrgID: "abc", OrgName: "MyOrg", ExternalOrgID: 123, InstallationID: 456, Version: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	name := "MyOrg"
	org, err := client.UpdateGithubOrg(context.Background(), &eh.UpdateGithubOrgRequest{
		OrgID:   "abc",
		Version: 1,
		OrgName: &name,
	})
	require.NoError(t, err)
	require.Equal(t, "abc", org.OrgID)
}

func TestUpdateGithubOrgError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.UpdateGithubOrg(context.Background(), &eh.UpdateGithubOrgRequest{OrgID: "abc", Version: 1})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func TestUpdateGithubOrgConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveGithubOrgConflict()
	defer srv.Close()

	_, err := client.UpdateGithubOrg(context.Background(), &eh.UpdateGithubOrgRequest{OrgID: "abc", Version: 1})
	verifyGithubOrgConflict(t, err)
}

// nolint: dupl
func TestUpdateGithubOrgPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedGithubOrgID, parts[4])

		w.WriteHeader(http.StatusOK)
		resp := eh.GithubOrg{OrgID: githubOrgIDThatNeedsEscaping}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.UpdateGithubOrg(context.Background(), &eh.UpdateGithubOrgRequest{
		OrgID:   githubOrgIDThatNeedsEscaping,
		Version: 1,
	})
	require.NoError(t, err)
}

func TestDeleteGithubOrg(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/v1/github/orgs/abc", r.URL.Path)
		require.Equal(t, "1", r.Header.Get("If-Match"))

		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	err := client.DeleteGithubOrg(context.Background(), &eh.DeleteGithubOrgRequest{OrgID: "abc", Version: 1})
	require.NoError(t, err)
}

func TestDeleteGithubOrgError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	err := client.DeleteGithubOrg(context.Background(), &eh.DeleteGithubOrgRequest{OrgID: "abc", Version: 1})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func TestDeleteGithubOrgConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveGithubOrgConflict()
	defer srv.Close()

	err := client.DeleteGithubOrg(context.Background(), &eh.DeleteGithubOrgRequest{OrgID: "abc", Version: 1})
	verifyGithubOrgConflict(t, err)
}

func TestDeleteGithubOrgPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedGithubOrgID, parts[4])

		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	err := client.DeleteGithubOrg(context.Background(), &eh.DeleteGithubOrgRequest{OrgID: githubOrgIDThatNeedsEscaping, Version: 1})
	require.NoError(t, err)
}

func TestCreateFeatureFlag(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/v1/featureflags/flag", r.URL.Path)

		var reqBody eh.CreateFeatureFlagRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.Equal(t, "desc", reqBody.Description)
		require.Equal(t, 0.5, reqBody.DefaultPct)

		w.WriteHeader(http.StatusCreated)
		resp := eh.FeatureFlag{Name: "flag"}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	flag, err := client.CreateFeatureFlag(context.Background(), &eh.CreateFeatureFlagRequest{FlagName: "flag", Description: "desc", DefaultPct: 0.5})
	require.NoError(t, err)
	require.Equal(t, "flag", flag.Name)
}

func TestCreateFeatureFlagError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.CreateFeatureFlag(context.Background(), &eh.CreateFeatureFlagRequest{FlagName: "flag", Description: "d", DefaultPct: 0.1})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func serveFeatureFlagConflict() (*httptest.Server, *eh.Client) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(eh.ConflictError{
			ResponseCode: http.StatusConflict,
			Message:      "exists",
			ErrorType:    "Conflict",
			Current:      &eh.FeatureFlag{Name: "flag"},
		})
	})

	srv := httptest.NewServer(handler)
	client := eh.NewClient(srv.URL)
	return srv, client
}

func verifyFeatureFlagConflict(t *testing.T, err error) {
	var clientErr *eh.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.Equal(t, "exists", clientErr.Message)
	require.Equal(t, "Conflict", clientErr.ErrorType)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, eh.ObjectTypeFeatureFlag, clientErr.Current.ObjectType())
	ff, ok := clientErr.Current.(*eh.FeatureFlag)
	require.True(t, ok, "Expected Current to be of type *eh.FeatureFlag")
	require.Equal(t, eh.FeatureFlag{Name: "flag"}, *ff)
}

func TestCreateFeatureFlagConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveFeatureFlagConflict()
	defer srv.Close()

	_, err := client.CreateFeatureFlag(context.Background(), &eh.CreateFeatureFlagRequest{FlagName: "flag", Description: "desc", DefaultPct: 0.5})
	verifyFeatureFlagConflict(t, err)
}

func TestCreateFeatureFlagPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 4, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedFeatureFlagName, parts[3])

		w.WriteHeader(http.StatusCreated)
		resp := eh.FeatureFlag{Name: featureFlagNameThatNeedsEscaping}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.CreateFeatureFlag(context.Background(), &eh.CreateFeatureFlagRequest{FlagName: featureFlagNameThatNeedsEscaping, Description: "desc", DefaultPct: 0.5})
	require.NoError(t, err)
}

func TestCreateFeatureFlagOverride(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/v1/tenants/abc/featureFlagOverrides/flag", r.URL.Path)

		var reqBody eh.CreateFeatureFlagOverrideRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.True(t, reqBody.Enabled)

		w.WriteHeader(http.StatusCreated)
		resp := eh.FeatureFlagOverride{FlagName: "flag", TenantID: "abc", Enabled: true}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	resp, err := client.CreateFeatureFlagOverride(context.Background(), &eh.CreateFeatureFlagOverrideRequest{TenantID: "abc", FlagName: "flag", Enabled: true})
	require.NoError(t, err)
	require.Equal(t, "flag", resp.FlagName)
	require.True(t, resp.Enabled)
}

func TestCreateFeatureFlagOverrideError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.CreateFeatureFlagOverride(context.Background(), &eh.CreateFeatureFlagOverrideRequest{TenantID: "abc", FlagName: "flag", Enabled: true})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func serveFeatureFlagOverrideConflict() (*httptest.Server, *eh.Client) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(eh.ConflictError{
			ResponseCode: http.StatusConflict,
			Message:      "exists",
			ErrorType:    "Conflict",
			Current:      &eh.FeatureFlagOverride{FlagName: "flag", TenantID: "abc", Enabled: true},
		})
	})

	srv := httptest.NewServer(handler)
	client := eh.NewClient(srv.URL)
	return srv, client
}

func verifyFeatureFlagOverrideConflict(t *testing.T, err error) {
	var clientErr *eh.ConflictError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusConflict, clientErr.ResponseCode)
	require.Equal(t, "exists", clientErr.Message)
	require.Equal(t, "Conflict", clientErr.ErrorType)
	require.NotNil(t, clientErr.Current)
	require.Equal(t, eh.ObjectTypeFeatureFlagOverride, clientErr.Current.ObjectType())
	ovr, ok := clientErr.Current.(*eh.FeatureFlagOverride)
	require.True(t, ok, "Expected Current to be of type *eh.FeatureFlagOverride")
	require.Equal(t, eh.FeatureFlagOverride{FlagName: "flag", TenantID: "abc", Enabled: true}, *ovr)
}

func TestCreateFeatureFlagOverrideConflictError(t *testing.T) {
	t.Parallel()

	srv, client := serveFeatureFlagOverrideConflict()
	defer srv.Close()

	_, err := client.CreateFeatureFlagOverride(context.Background(), &eh.CreateFeatureFlagOverrideRequest{TenantID: "abc", FlagName: "flag", Enabled: true})
	verifyFeatureFlagOverrideConflict(t, err)
}

// nolint: dupl
func TestGetFeatureFlagPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3])
		require.Equal(t, escapedFeatureFlagName, parts[5])

		w.WriteHeader(http.StatusCreated)
		resp := eh.FeatureFlagOverride{FlagName: featureFlagNameThatNeedsEscaping, TenantID: tenantIDThatNeedsEscaping, Enabled: true}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.CreateFeatureFlagOverride(context.Background(), &eh.CreateFeatureFlagOverrideRequest{TenantID: tenantIDThatNeedsEscaping, FlagName: featureFlagNameThatNeedsEscaping, Enabled: true})
	require.NoError(t, err)
}

func TestGetFeatureFlagOverride(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/featureFlagOverrides/flag", r.URL.Path)
		require.Empty(t, r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.FeatureFlagOverride{FlagName: "flag", TenantID: "abc", Enabled: true}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	resp, err := client.GetFeatureFlagOverride(context.Background(), &eh.GetFeatureFlagOverrideRequest{TenantID: "abc", FlagName: "flag"})
	require.NoError(t, err)
	require.Equal(t, "flag", resp.FlagName)
	require.True(t, resp.Enabled)
}

func TestGetFeatureFlagOverrideIncludeDeleted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/featureFlagOverrides/flag", r.URL.Path)
		require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.FeatureFlagOverride{FlagName: "flag", TenantID: "abc", Enabled: true}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	includeDeleted := true
	_, err := client.GetFeatureFlagOverride(context.Background(), &eh.GetFeatureFlagOverrideRequest{TenantID: "abc", FlagName: "flag", IncludeDeleted: &includeDeleted})
	require.NoError(t, err)
}

func TestGetFeatureFlagOverrideError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.GetFeatureFlagOverride(context.Background(), &eh.GetFeatureFlagOverrideRequest{TenantID: "abc", FlagName: "flag"})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func TestGetFeatureFlagOverridePathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3])
		require.Equal(t, escapedFeatureFlagName, parts[5])

		w.WriteHeader(http.StatusOK)
		resp := eh.FeatureFlagOverride{FlagName: featureFlagNameThatNeedsEscaping, TenantID: tenantIDThatNeedsEscaping, Enabled: true}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetFeatureFlagOverride(context.Background(), &eh.GetFeatureFlagOverrideRequest{TenantID: tenantIDThatNeedsEscaping, FlagName: featureFlagNameThatNeedsEscaping})
	require.NoError(t, err)
}

func TestUpdateFeatureFlagOverride(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPatch, r.Method)
		require.Equal(t, "/v1/tenants/abc/featureFlagOverrides/flag", r.URL.Path)
		require.Equal(t, "1", r.Header.Get("If-Match"))

		var reqBody eh.UpdateFeatureFlagOverrideRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.NotNil(t, reqBody.Enabled)
		require.True(t, *reqBody.Enabled)

		w.WriteHeader(http.StatusOK)
		resp := eh.FeatureFlagOverride{FlagName: "flag", TenantID: "abc", Enabled: true}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	enabled := true
	resp, err := client.UpdateFeatureFlagOverride(context.Background(), &eh.UpdateFeatureFlagOverrideRequest{
		TenantID: "abc",
		FlagName: "flag",
		Version:  1,
		Enabled:  &enabled,
	})
	require.NoError(t, err)
	require.Equal(t, "flag", resp.FlagName)
	require.True(t, resp.Enabled)
}

func TestUpdateFeatureFlagOverrideError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	enabled := true
	_, err := client.UpdateFeatureFlagOverride(context.Background(), &eh.UpdateFeatureFlagOverrideRequest{
		TenantID: "abc", FlagName: "flag", Version: 1, Enabled: &enabled,
	})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func TestUpdateFeatureFlagOverrideConflictError(t *testing.T) {
	t.Parallel()
	srv, client := serveFeatureFlagOverrideConflict()
	defer srv.Close()

	enabled := true
	_, err := client.UpdateFeatureFlagOverride(context.Background(), &eh.UpdateFeatureFlagOverrideRequest{
		TenantID: "abc", FlagName: "flag", Version: 1, Enabled: &enabled,
	})
	verifyFeatureFlagOverrideConflict(t, err)
}

// nolint: dupl
func TestUpdateFeatureFlagOverridePathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3])
		require.Equal(t, escapedFeatureFlagName, parts[5])

		w.WriteHeader(http.StatusOK)
		resp := eh.FeatureFlagOverride{FlagName: featureFlagNameThatNeedsEscaping, TenantID: tenantIDThatNeedsEscaping}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	enabled := true
	_, err := client.UpdateFeatureFlagOverride(context.Background(), &eh.UpdateFeatureFlagOverrideRequest{
		TenantID: tenantIDThatNeedsEscaping,
		FlagName: featureFlagNameThatNeedsEscaping,
		Version:  1,
		Enabled:  &enabled,
	})
	require.NoError(t, err)
}

// nolint: dupl
func TestListFeatureFlags(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/featureflags", r.URL.Path)
		require.Equal(t, "123", r.URL.Query().Get("maxResults"))
		require.Equal(t, tokenID, r.URL.Query().Get("token"))
		require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.ListFeatureFlagsResponse{FeatureFlags: []eh.FeatureFlag{{Name: "flag"}}}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	maxResults := 123
	includeDeleted := true
	resp, err := client.ListFeatureFlags(context.Background(), &eh.ListFeatureFlagsRequest{MaxResults: &maxResults, Token: util.Pointer(tokenID), IncludeDeleted: &includeDeleted})
	require.NoError(t, err)
	require.Len(t, resp.FeatureFlags, 1)
	require.Equal(t, "flag", resp.FeatureFlags[0].Name)
}

func TestListFeatureFlagsError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListFeatureFlags(context.Background(), &eh.ListFeatureFlagsRequest{})
	require.Error(t, err)
}

func TestUpdateFeatureFlag(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPatch, r.Method)
		require.Equal(t, "/v1/featureflags/flag", r.URL.Path)
		require.Equal(t, "1", r.Header.Get("If-Match"))

		var reqBody eh.UpdateFeatureFlagRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.NotNil(t, reqBody.Description)
		require.Equal(t, "new", *reqBody.Description)

		w.WriteHeader(http.StatusOK)
		resp := eh.FeatureFlag{Name: "flag", Description: "new", Version: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	desc := "new"
	flag, err := client.UpdateFeatureFlag(context.Background(), &eh.UpdateFeatureFlagRequest{
		FlagName:    "flag",
		Version:     1,
		Description: &desc,
	})
	require.NoError(t, err)
	require.Equal(t, "flag", flag.Name)
}

func TestUpdateFeatureFlagError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.UpdateFeatureFlag(context.Background(), &eh.UpdateFeatureFlagRequest{FlagName: "flag", Version: 1})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func TestUpdateFeatureFlagConflictError(t *testing.T) {
	t.Parallel()

	srv, client := serveFeatureFlagConflict()
	defer srv.Close()

	_, err := client.UpdateFeatureFlag(context.Background(), &eh.UpdateFeatureFlagRequest{FlagName: "flag", Version: 1})
	verifyFeatureFlagConflict(t, err)
}

// nolint: dupl
func TestUpdateFeatureFlagPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 4, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedFeatureFlagName, parts[3])

		w.WriteHeader(http.StatusOK)
		resp := eh.FeatureFlag{Name: featureFlagNameThatNeedsEscaping}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.UpdateFeatureFlag(context.Background(), &eh.UpdateFeatureFlagRequest{
		FlagName: featureFlagNameThatNeedsEscaping,
		Version:  1,
	})
	require.NoError(t, err)
}

func TestGetTenantFeatureFlags(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/featureflags", r.URL.Path)

		w.WriteHeader(http.StatusOK)
		resp := eh.GetTenantFeatureFlagsResponse{FeatureFlags: map[string]bool{"foo": true}}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	resp, err := client.GetTenantFeatureFlags(context.Background(), &eh.GetTenantFeatureFlagsRequest{TenantID: "abc"})
	require.NoError(t, err)
	require.True(t, resp.FeatureFlags["foo"])
}

func TestGetTenantFeatureFlagsError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.GetTenantFeatureFlags(context.Background(), &eh.GetTenantFeatureFlagsRequest{TenantID: "abc"})
	require.Error(t, err)
}

func TestGetTenantFeatureFlagsPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3], "TenantID not properly escaped in URL path")

		w.WriteHeader(http.StatusOK)
		resp := eh.GetTenantFeatureFlagsResponse{}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetTenantFeatureFlags(context.Background(), &eh.GetTenantFeatureFlagsRequest{TenantID: tenantIDThatNeedsEscaping})
	require.NoError(t, err)
}

func TestDeleteFeatureFlag(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/v1/featureflags/flag", r.URL.Path)
		require.Equal(t, "1", r.Header.Get("If-Match"))

		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	err := client.DeleteFeatureFlag(context.Background(), &eh.DeleteFeatureFlagRequest{FlagName: "flag", Version: 1})
	require.NoError(t, err)
}

func TestDeleteFeatureFlagError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	err := client.DeleteFeatureFlag(context.Background(), &eh.DeleteFeatureFlagRequest{FlagName: "flag", Version: 1})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func TestDeleteFeatureFlagConflictError(t *testing.T) {
	t.Parallel()

	srv, client := serveFeatureFlagConflict()
	defer srv.Close()

	err := client.DeleteFeatureFlag(context.Background(), &eh.DeleteFeatureFlagRequest{FlagName: "flag", Version: 1})
	verifyFeatureFlagConflict(t, err)
}

func TestDeleteFeatureFlagPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 4, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedFeatureFlagName, parts[3])

		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	err := client.DeleteFeatureFlag(context.Background(), &eh.DeleteFeatureFlagRequest{FlagName: featureFlagNameThatNeedsEscaping, Version: 1})
	require.NoError(t, err)
}
func TestDeleteFeatureFlagOverride(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/v1/tenants/abc/featureFlagOverrides/flag", r.URL.Path)
		require.Equal(t, "1", r.Header.Get("If-Match"))

		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	err := client.DeleteFeatureFlagOverride(context.Background(), &eh.DeleteFeatureFlagOverrideRequest{TenantID: "abc", FlagName: "flag", Version: 1})
	require.NoError(t, err)
}

func TestDeleteFeatureFlagOverrideError(t *testing.T) {
	t.Parallel()

	srv, client := serveBadRequest()
	defer srv.Close()

	err := client.DeleteFeatureFlagOverride(context.Background(), &eh.DeleteFeatureFlagOverrideRequest{TenantID: "abc", FlagName: "flag", Version: 1})
	var clientErr *eh.Error
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.ResponseCode)
	require.Equal(t, "bad", clientErr.Message)
}

func TestDeleteFeatureFlagOverrideConflictError(t *testing.T) {
	t.Parallel()

	srv, client := serveFeatureFlagOverrideConflict()
	defer srv.Close()

	err := client.DeleteFeatureFlagOverride(context.Background(), &eh.DeleteFeatureFlagOverrideRequest{TenantID: "abc", FlagName: "flag", Version: 1})
	verifyFeatureFlagOverrideConflict(t, err)
}

func TestDeleteFeatureFlagOverridePathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 6, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3])
		require.Equal(t, escapedFeatureFlagName, parts[5])

		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	err := client.DeleteFeatureFlagOverride(context.Background(), &eh.DeleteFeatureFlagOverrideRequest{TenantID: tenantIDThatNeedsEscaping, FlagName: featureFlagNameThatNeedsEscaping, Version: 1})
	require.NoError(t, err)
}
func TestListFeatureFlagOverrides(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/abc/featureFlagOverrides", r.URL.Path)
		require.Equal(t, "123", r.URL.Query().Get("maxResults"))
		require.Equal(t, tokenID, r.URL.Query().Get("token"))
		require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))

		w.WriteHeader(http.StatusOK)
		resp := eh.ListFeatureFlagOverridesResponse{FeatureFlagOverrides: []eh.FeatureFlagOverride{{FlagName: "flag", TenantID: "abc"}}}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	maxResults := 123
	includeDeleted := true
	resp, err := client.ListFeatureFlagOverrides(context.Background(), &eh.ListFeatureFlagOverridesRequest{
		TenantID:       "abc",
		MaxResults:     &maxResults,
		Token:          util.Pointer(tokenID),
		IncludeDeleted: &includeDeleted,
	})
	require.NoError(t, err)
	require.Len(t, resp.FeatureFlagOverrides, 1)
	require.Equal(t, "flag", resp.FeatureFlagOverrides[0].FlagName)
}

func TestListFeatureFlagOverridesPathEscaping(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := r.URL.EscapedPath()
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, 5, len(parts), "path doesn't have correct # of parts: %s", escapedPath)
		require.Equal(t, escapedTenantID, parts[3])

		w.WriteHeader(http.StatusOK)
		resp := eh.ListFeatureFlagOverridesResponse{}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.ListFeatureFlagOverrides(context.Background(), &eh.ListFeatureFlagOverridesRequest{TenantID: tenantIDThatNeedsEscaping})
	require.NoError(t, err)
}

func TestListFeatureFlagOverridesError(t *testing.T) {
	t.Parallel()
	srv, client := serveBadRequest()
	defer srv.Close()

	_, err := client.ListFeatureFlagOverrides(context.Background(), &eh.ListFeatureFlagOverridesRequest{TenantID: "abc"})
	require.Error(t, err)
}

func TestFeatureFlagsHeader(t *testing.T) {
	t.Parallel()
	ffHeader := `{"ff":true}`

	cases := []struct {
		name   string
		status int
		resp   any
		call   func(*eh.Client) error
	}{
		{
			name:   "CreateTenant",
			status: http.StatusCreated,
			resp:   eh.Tenant{},
			call: func(c *eh.Client) error {
				_, err := c.CreateTenant(context.Background(), &eh.CreateTenantRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
					Type:         eh.TenantTypeUser,
				})
				return err
			},
		},
		{
			name:   "GetTenant",
			status: http.StatusOK,
			resp:   eh.Tenant{},
			call: func(c *eh.Client) error {
				_, err := c.GetTenant(context.Background(), &eh.GetTenantRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
				})
				return err
			},
		},
		{
			name:   "GetCurrentUser",
			status: http.StatusOK,
			resp:   eh.Tenant{},
			call: func(c *eh.Client) error {
				_, err := c.GetCurrentUser(context.Background(), &eh.GetCurrentUserRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
				})
				return err
			},
		},
		{
			name:   "GetTenantFeatureFlags",
			status: http.StatusOK,
			resp:   eh.GetTenantFeatureFlagsResponse{},
			call: func(c *eh.Client) error {
				_, err := c.GetTenantFeatureFlags(context.Background(), &eh.GetTenantFeatureFlagsRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
				})
				return err
			},
		},
		{
			name:   "GenerateWebUIToken",
			status: http.StatusCreated,
			resp:   eh.GenerateWebUITokenResponse{JWT: "x"},
			call: func(c *eh.Client) error {
				_, err := c.GenerateWebUIToken(context.Background(), &eh.GenerateWebUITokenRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
					TokenID:      "id",
				})
				return err
			},
		},
		{
			name:   "CreateEnvironment",
			status: http.StatusCreated,
			resp:   eh.Environment{},
			call: func(c *eh.Client) error {
				_, err := c.CreateEnvironment(context.Background(), &eh.CreateEnvironmentRequest{
					FeatureFlags:  eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:      "t",
					EnvironmentID: "e",
				})
				return err
			},
		},
		{
			name:   "GetRunner",
			status: http.StatusOK,
			resp:   eh.Runner{},
			call: func(c *eh.Client) error {
				_, err := c.GetRunner(context.Background(), &eh.GetRunnerRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
					RunnerID:     "r",
				})
				return err
			},
		},
		{
			name:   "GetEnvironment",
			status: http.StatusOK,
			resp:   eh.Environment{},
			call: func(c *eh.Client) error {
				_, err := c.GetEnvironment(context.Background(), &eh.GetEnvironmentRequest{
					FeatureFlags:  eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:      "t",
					EnvironmentID: "e",
				})
				return err
			},
		},
		{
			name:   "UpdateEnvironment",
			status: http.StatusOK,
			resp:   eh.Environment{},
			call: func(c *eh.Client) error {
				_, err := c.UpdateEnvironment(context.Background(), &eh.UpdateEnvironmentRequest{
					FeatureFlags:  eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:      "t",
					EnvironmentID: "e",
					Version:       1,
				})
				return err
			},
		},
		{
			name:   "ListEnvironments",
			status: http.StatusOK,
			resp:   eh.List[eh.Environment]{},
			call: func(c *eh.Client) error {
				_, err := c.ListEnvironments(context.Background(), &eh.ListEnvironmentsRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
				})
				return err
			},
		},
		{
			name:   "DeleteEnvironment",
			status: http.StatusNoContent,
			resp:   nil,
			call: func(c *eh.Client) error {
				return c.DeleteEnvironment(context.Background(), &eh.DeleteEnvironmentRequest{
					FeatureFlags:  eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:      "t",
					EnvironmentID: "e",
					Version:       1,
				})
			},
		},
		{
			name:   "UploadTurnLogs",
			status: http.StatusOK,
			resp:   eh.UploadTurnLogsResponse{},
			call: func(c *eh.Client) error {
				_, err := c.UploadTurnLogs(context.Background(), &eh.UploadTurnLogsRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
					TaskID:       "task",
					TurnIndex:    1,
					Version:      1,
					Index:        0,
					Logs:         []eh.TurnLog{},
				})
				return err
			},
		},
		{
			name:   "GetLastTurnLog",
			status: http.StatusOK,
			resp:   eh.LastTurnLog{},
			call: func(c *eh.Client) error {
				_, err := c.GetLastTurnLog(context.Background(), &eh.GetLastTurnLogRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
					TaskID:       "task",
					TurnIndex:    1,
				})
				return err
			},
		},
		{
			name:   "StreamTurnLogs",
			status: http.StatusNoContent,
			resp:   nil,
			call: func(c *eh.Client) error {
				body, err := c.StreamTurnLogs(context.Background(), &eh.StreamTurnLogsRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
					TaskID:       "task",
					TurnIndex:    1,
				})
				if body != nil {
					_ = body.Close()
				}
				return err
			},
		},
		{
			name:   "GetTask",
			status: http.StatusOK,
			resp:   eh.Task{},
			call: func(c *eh.Client) error {
				_, err := c.GetTask(context.Background(), &eh.GetTaskRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
					TaskID:       "task",
				})
				return err
			},
		},
		{
			name:   "CreateTask",
			status: http.StatusCreated,
			resp:   eh.Task{},
			call: func(c *eh.Client) error {
				_, err := c.CreateTask(context.Background(), &eh.CreateTaskRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
					TaskID:       "task",
					RepoInfo:     map[string]*eh.RepoInfo{},
				})
				return err
			},
		},
		{
			name:   "UpdateTask",
			status: http.StatusOK,
			resp:   eh.Task{},
			call: func(c *eh.Client) error {
				title := ""
				_, err := c.UpdateTask(context.Background(), &eh.UpdateTaskRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
					TaskID:       "task",
					Version:      1,
					Title:        &title,
				})
				return err
			},
		},
		{
			name:   "DeleteTask",
			status: http.StatusNoContent,
			resp:   nil,
			call: func(c *eh.Client) error {
				return c.DeleteTask(context.Background(), &eh.DeleteTaskRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
					TaskID:       "task",
					Version:      1,
				})
			},
		},
		{
			name:   "DeleteWorkstreamTask",
			status: http.StatusNoContent,
			resp:   nil,
			call: func(c *eh.Client) error {
				return c.DeleteWorkstreamTask(context.Background(), &eh.DeleteWorkstreamTaskRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
					WorkstreamID: "ws",
					TaskID:       "task",
					Version:      1,
				})
			},
		},
		{
			name:   "ListTasks",
			status: http.StatusOK,
			resp:   eh.ListTasksResponse{},
			call: func(c *eh.Client) error {
				_, err := c.ListTasks(context.Background(), &eh.ListTasksRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
				})
				return err
			},
		},
		{
			name:   "CreateTurn",
			status: http.StatusCreated,
			resp:   eh.Turn{},
			call: func(c *eh.Client) error {
				_, err := c.CreateTurn(context.Background(), &eh.CreateTurnRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
					TaskID:       "task",
					TurnIndex:    2,
					TaskVersion:  1,
					Prompt:       "",
				})
				return err
			},
		},
		{
			name:   "GetTurn",
			status: http.StatusOK,
			resp:   eh.Turn{},
			call: func(c *eh.Client) error {
				_, err := c.GetTurn(context.Background(), &eh.GetTurnRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
					TaskID:       "task",
					TurnIndex:    1,
				})
				return err
			},
		},
		{
			name:   "GetLastTurn",
			status: http.StatusOK,
			resp:   eh.Turn{},
			call: func(c *eh.Client) error {
				_, err := c.GetLastTurn(context.Background(), &eh.GetLastTurnRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
					TaskID:       "task",
				})
				return err
			},
		},
		{
			name:   "UpdateTurn",
			status: http.StatusOK,
			resp:   eh.Turn{},
			call: func(c *eh.Client) error {
				status := "s"
				_, err := c.UpdateTurn(context.Background(), &eh.UpdateTurnRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
					TaskID:       "task",
					TurnIndex:    1,
					Version:      1,
					Status:       &status,
				})
				return err
			},
		},
		{
			name:   "ListTurns",
			status: http.StatusOK,
			resp:   eh.ListTurnsResponse{},
			call: func(c *eh.Client) error {
				_, err := c.ListTurns(context.Background(), &eh.ListTurnsRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
					TaskID:       "task",
				})
				return err
			},
		},
		{
			name:   "ListPolicies",
			status: http.StatusOK,
			resp:   eh.ListPoliciesResponse{},
			call: func(c *eh.Client) error {
				_, err := c.ListPolicies(context.Background(), &eh.ListPoliciesRequest{
					FeatureFlags: eh.FeatureFlags{FeatureFlags: map[string]bool{"ff": true}},
					TenantID:     "t",
				})
				return err
			},
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, ffHeader, r.Header.Get("X-EventHorizon-FeatureFlags"))
				w.WriteHeader(tt.status)
				if tt.resp != nil {
					_ = json.NewEncoder(w).Encode(tt.resp)
				}
			}))
			defer srv.Close()
			client := eh.NewClient(srv.URL)
			require.NoError(t, tt.call(client))
		})
	}
}
