package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/plan42-ai/sdk-go/p42"
	"github.com/stretchr/testify/require"
)

func TestGetTaskCmdRun(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/tenant-1/tasks/task-1", r.URL.Path)
		require.Equal(t, "false", r.URL.Query().Get("includeDeleted"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"TenantId":"tenant-1","TaskId":"task-1","Title":"t","Prompt":"p","Parallel":false,"RepoInfo":{},"State":"Pending","CreatedAt":"%s","UpdatedAt":"%s","Deleted":false,"Version":1}`, now, now)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	opts := GetTaskCmdOptions{TenantID: "tenant-1", TaskID: "task-1"}
	shared := SharedOptions{Client: p42.NewClient(srv.URL), Stdout: &buf, Stderr: io.Discard}

	require.NoError(t, opts.Run(context.Background(), &shared))

	var task p42.Task
	require.NoError(t, json.Unmarshal(buf.Bytes(), &task))
	require.Equal(t, "tenant-1", task.TenantID)
	require.Equal(t, "task-1", task.TaskID)
}

func TestGetTaskCmdRunIncludeDeleted(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/tenant-1/tasks/task-1", r.URL.Path)
		require.Equal(t, "true", r.URL.Query().Get("includeDeleted"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"TenantId":"tenant-1","TaskId":"task-1","Title":"t","Prompt":"p","Parallel":false,"RepoInfo":{},"State":"Pending","CreatedAt":"%s","UpdatedAt":"%s","Deleted":true,"Version":1}`, now, now)
	}))
	defer srv.Close()

	opts := GetTaskCmdOptions{TenantID: "tenant-1", TaskID: "task-1", IncludeDeleted: true}
	shared := SharedOptions{Client: p42.NewClient(srv.URL), Stdout: io.Discard, Stderr: io.Discard}

	require.NoError(t, opts.Run(context.Background(), &shared))
}
