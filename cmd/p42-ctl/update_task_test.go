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

func TestUpdateTaskCmdRun(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPatch, r.Method)
		require.Equal(t, "/v1/tenants/tenant-1/tasks/task-1", r.URL.Path)
		require.Equal(t, "3", r.Header.Get("If-Match"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "new title", body["Title"])
		require.Equal(t, "new prompt", body["Prompt"])

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"TenantId":"tenant-1","TaskId":"task-1","Title":"new title","Prompt":"new prompt","Parallel":false,"RepoInfo":{},"State":"Pending","CreatedAt":"%s","UpdatedAt":"%s","Deleted":false,"Version":4}`, now, now)
	}))
	defer srv.Close()

	title := "new title"
	prompt := "new prompt"
	var buf bytes.Buffer
	opts := UpdateTaskCmdOptions{
		TenantID: "tenant-1",
		TaskID:   "task-1",
		Version:  3,
		Title:    &title,
		Prompt:   &prompt,
	}
	shared := SharedOptions{Client: p42.NewClient(srv.URL), Stdout: &buf, Stderr: io.Discard}

	require.NoError(t, opts.Run(context.Background(), &shared))

	var task p42.Task
	require.NoError(t, json.Unmarshal(buf.Bytes(), &task))
	require.Equal(t, "tenant-1", task.TenantID)
	require.Equal(t, "task-1", task.TaskID)
	require.Equal(t, "new title", task.Title)
	require.Equal(t, "new prompt", task.Prompt)
	require.Equal(t, 4, task.Version)
}

func TestUpdateTaskCmdRunWithModel(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPatch, r.Method)

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "Claude 4.5 Opus", body["Model"])

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"TenantId":"t","TaskId":"tk","Title":"","Prompt":"","Parallel":false,"Model":"Claude 4.5 Opus","RepoInfo":{},"State":"Pending","CreatedAt":"%s","UpdatedAt":"%s","Deleted":false,"Version":2}`, now, now)
	}))
	defer srv.Close()

	model := "Claude 4.5 Opus"
	opts := UpdateTaskCmdOptions{
		TenantID: "t",
		TaskID:   "tk",
		Version:  1,
		Model:    &model,
	}
	shared := SharedOptions{Client: p42.NewClient(srv.URL), Stdout: io.Discard, Stderr: io.Discard}

	require.NoError(t, opts.Run(context.Background(), &shared))
}

func TestUpdateTaskCmdRunWithRepoInfo(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPatch, r.Method)

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		ri, ok := body["RepoInfo"].(map[string]any)
		require.True(t, ok)
		repo, ok := ri["org/repo"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "fb", repo["FeatureBranch"])
		require.Equal(t, "main", repo["TargetBranch"])

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"TenantId":"t","TaskId":"tk","Title":"","Prompt":"","Parallel":false,"RepoInfo":{},"State":"Pending","CreatedAt":"%s","UpdatedAt":"%s","Deleted":false,"Version":2}`, now, now)
	}))
	defer srv.Close()

	repoJSON := `{"org/repo":{"FeatureBranch":"fb","TargetBranch":"main"}}`
	opts := UpdateTaskCmdOptions{
		TenantID:     "t",
		TaskID:       "tk",
		Version:      1,
		RepoInfoJSON: &repoJSON,
	}
	shared := SharedOptions{Client: p42.NewClient(srv.URL), Stdout: io.Discard, Stderr: io.Discard}

	require.NoError(t, opts.Run(context.Background(), &shared))
}

func TestUpdateTaskCmdRunBadRepoInfoJSON(t *testing.T) {
	t.Parallel()

	badJSON := `{not valid`
	opts := UpdateTaskCmdOptions{
		TenantID:     "t",
		TaskID:       "tk",
		Version:      1,
		RepoInfoJSON: &badJSON,
	}
	shared := SharedOptions{Client: p42.NewClient("http://example.com"), Stdout: io.Discard, Stderr: io.Discard}

	err := opts.Run(context.Background(), &shared)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid --repo-info JSON")
}
