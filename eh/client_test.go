package eh_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/debugging-sucks/clock"
	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	sigv4clientutil "github.com/debugging-sucks/sigv4util/client"
	sigv4auth "github.com/debugging-sucks/sigv4util/server/sigv4auth"
	"github.com/stretchr/testify/require"
)

var (
	expectedSigCreate = "UE9TVCAvIEhUVFAvMS4xDQpIb3N0OiBzdHMudXMtd2VzdC0yLmFtYXpvbmF3cy5jb20NClVzZXItQWdlbnQ6IEdvLWh0dHAtY2xpZW50LzEuMQ0KVHJhbnNmZXItRW5jb2Rpbmc6IGNodW5rZWQNCkFjY2VwdDogYXBwbGljYXRpb24vanNvbg0KQWNjZXB0LUVuY29kaW5nOiBpZGVudGl0eQ0KQXV0aG9yaXphdGlvbjogQVdTNC1ITUFDLVNIQTI1NiBDcmVkZW50aWFsPUFLSUQvMjAyNTAxMDEvdXMtd2VzdC0yL3N0cy9hd3M0X3JlcXVlc3QsIFNpZ25lZEhlYWRlcnM9YWNjZXB0O2FjY2VwdC1lbmNvZGluZztjb250ZW50LXR5cGU7aG9zdDt4LWFtei1kYXRlO3gtYW16LXNlY3VyaXR5LXRva2VuO3gtZXZlbnRob3Jpem9uLXJlcXVlc3QtaGFzaCwgU2lnbmF0dXJlPTU4MzhjMTMwNjgxMmQwZTFhNWQ0ZTQ0ZjlmM2FjZjE4OTBhYTFjZGQ0ZWU3MGFlYjFlNmRlMThmN2RiMzBjYjYNCkNvbnRlbnQtVHlwZTogYXBwbGljYXRpb24veC13d3ctZm9ybS11cmxlbmNvZGVkDQpYLUFtei1EYXRlOiAyMDI1MDEwMVQwMDAwMDBaDQpYLUFtei1TZWN1cml0eS1Ub2tlbjogVE9LRU4NClgtRXZlbnRob3Jpem9uLVJlcXVlc3QtSGFzaDogZGM4YmRlZTRhMGYzNWVkYzQzNjY5NmVkODVmOTdkNjRjMjU0ZDIyZmQzNmU5ZDhjYWFkY2VlZTliYzQ1M2UxZQ0KDQoyZA0KQWN0aW9uPUdldENhbGxlcklkZW50aXR5JlZlcnNpb249MjAxMS0wNi0xNQ0KDQowDQoNCg=="
	expectedSigGet    = "UE9TVCAvIEhUVFAvMS4xDQpIb3N0OiBzdHMudXMtd2VzdC0yLmFtYXpvbmF3cy5jb20NClVzZXItQWdlbnQ6IEdvLWh0dHAtY2xpZW50LzEuMQ0KVHJhbnNmZXItRW5jb2Rpbmc6IGNodW5rZWQNCkFjY2VwdDogYXBwbGljYXRpb24vanNvbg0KQWNjZXB0LUVuY29kaW5nOiBpZGVudGl0eQ0KQXV0aG9yaXphdGlvbjogQVdTNC1ITUFDLVNIQTI1NiBDcmVkZW50aWFsPUFLSUQvMjAyNTAxMDEvdXMtd2VzdC0yL3N0cy9hd3M0X3JlcXVlc3QsIFNpZ25lZEhlYWRlcnM9YWNjZXB0O2FjY2VwdC1lbmNvZGluZztjb250ZW50LXR5cGU7aG9zdDt4LWFtei1kYXRlO3gtYW16LXNlY3VyaXR5LXRva2VuO3gtZXZlbnRob3Jpem9uLXJlcXVlc3QtaGFzaCwgU2lnbmF0dXJlPTc0MTQ2YzllNjMwYjBmOTkwNTdjMWQyOTQ2MTVjNWE0OTQ0ZGRlNDExY2YxY2Q3MWEzYWRhYzRhODM4MDRlNjMNCkNvbnRlbnQtVHlwZTogYXBwbGljYXRpb24veC13d3ctZm9ybS11cmxlbmNvZGVkDQpYLUFtei1EYXRlOiAyMDI1MDEwMVQwMDAwMDBaDQpYLUFtei1TZWN1cml0eS1Ub2tlbjogVE9LRU4NClgtRXZlbnRob3Jpem9uLVJlcXVlc3QtSGFzaDogNDdkODc1MzkzZDM5YjVhODgxNjU2NWQzODNhYTZmNmQ0MDVhNGNjN2FiNjhiMTI0ZDI5ZmYzNjIxMmE5ZTFlNA0KDQoyZA0KQWN0aW9uPUdldENhbGxlcklkZW50aXR5JlZlcnNpb249MjAxMS0wNi0xNQ0KDQowDQoNCg=="

	tenantIDThatNeedsEscaping = "foo/../../bar"
	escapedTenantID           = "foo%2F..%2F..%2Fbar"
)

func TestCreateTenant(t *testing.T) {
	t.Parallel()
	// Mock server returns successful response
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
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

	if expected != "" {
		require.Equal(t, expected, parts[1])
	} else {
		t.Logf("sigv4: %s", parts[1])
	}

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
		case http.MethodPost:
			verifySigv4(t, r, &cfg, expectedSigCreate, clk)
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			verifySigv4(t, r, &cfg, expectedSigGet, clk)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		resp := eh.Tenant{TenantID: "abc", Type: eh.TenantTypeUser, Version: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Compute expected signatures using the server URL.
	body, _ := json.Marshal(eh.CreateTenantRequest{TenantID: "abc", Type: eh.TenantTypeUser})
	postReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/tenants/abc", bytes.NewReader(body))
	postReq.Header.Set("Content-Type", "application/json")
	postReq.Header.Set("Accept", "application/json")
	getReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/tenants/abc", nil)
	getReq.Header.Set("Accept", "application/json")
	_ = sigv4clientutil.AddAuthHeaders(context.Background(), postReq, &cfg, cfg.Region, clk)
	_ = sigv4clientutil.AddAuthHeaders(context.Background(), getReq, &cfg, cfg.Region, clk)
	expectedSigCreate = postReq.Header.Get("Authorization")[len("sts:GetCallerIdentity "):]
	expectedSigGet = getReq.Header.Get("Authorization")[len("sts:GetCallerIdentity "):]

	client := eh.NewClient(srv.URL, eh.WithSigv4Auth(cfg, clk))

	_, err := client.CreateTenant(context.Background(), &eh.CreateTenantRequest{TenantID: "abc", Type: eh.TenantTypeUser})
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
		resp := eh.ListPoliciesResponse{Policies: []eh.Policy{{Name: "p", SchemaVersion: "1.0"}}}
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
