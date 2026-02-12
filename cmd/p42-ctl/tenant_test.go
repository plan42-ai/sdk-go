package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/plan42-ai/sdk-go/p42"
	"github.com/stretchr/testify/require"
)

func TestGetTenantEncryptionKeyOptionsRun(t *testing.T) {
	t.Parallel()
	tenantID := "tenant-123"
	version := 2
	createdAt := "2024-01-02T03:04:05Z"

	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, fmt.Sprintf("/v1/tenants/%s/encryption-keys/%d", tenantID, version), r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"TenantID":"%s","Version":%d,"CreatedAt":"%s"}`, tenantID, version, createdAt)
			},
		),
	)
	defer srv.Close()

	opts := GetTenantEncryptionKeyOptions{TenantID: tenantID, Version: version}
	shared := SharedOptions{Client: p42.NewClient(srv.URL)}

	require.NoError(t, opts.Run(context.Background(), &shared))
}
