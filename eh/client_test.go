package eh_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	"github.com/stretchr/testify/require"
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
	tenantID := "foo/../../bar"
	expectedEscaped := "foo%2F..%2F..%2Fbar" // The properly escaped version

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the path and verify proper escaping
		escapedPath := r.URL.EscapedPath()

		// Split the path and check the escaped tenant ID part
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, len(parts), 4, "path doesn't have correct # of parts: %s", escapedPath)

		// The last part should be the escaped tenant ID
		require.Equal(t, expectedEscaped, parts[3], "TenantID not properly escaped in URL path")

		w.WriteHeader(http.StatusCreated)
		resp := eh.Tenant{TenantID: tenantID, Type: eh.TenantTypeUser, Version: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.CreateTenant(context.Background(), &eh.CreateTenantRequest{
		TenantID: tenantID,
		Type:     eh.TenantTypeUser,
	})

	require.NoError(t, err)
}

func TestGetTenantPathEscaping(t *testing.T) {
	t.Parallel()
	tenantID := "foo/../../bar"
	expectedEscaped := "foo%2F..%2F..%2Fbar" // The properly escaped version

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the path and verify proper escaping
		escapedPath := r.URL.EscapedPath()

		// Split the path and check the escaped tenant ID part
		parts := strings.Split(escapedPath, "/")
		require.Equal(t, len(parts), 4, "path doesn't have correct # of parts: %s", escapedPath)

		// The last part should be the escaped tenant ID
		require.Equal(t, expectedEscaped, parts[3], "TenantID not properly escaped in URL path")

		w.WriteHeader(http.StatusOK)
		resp := eh.Tenant{TenantID: tenantID, Type: eh.TenantTypeUser, Version: 1}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := eh.NewClient(srv.URL)
	_, err := client.GetTenant(context.Background(), &eh.GetTenantRequest{
		TenantID: tenantID,
	})

	require.NoError(t, err)
}
