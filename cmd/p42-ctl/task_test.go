package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/plan42-ai/sdk-go/p42"
	"github.com/stretchr/testify/require"
)

func TestSearchTasksOptionsRun(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/v1/tasks/search", r.URL.Path)
				require.Equal(t, "12345", r.URL.Query().Get("pullRequestId"))
				require.Equal(t, "", r.URL.Query().Get("taskId"))
				require.Equal(t, "application/json", r.Header.Get("Content-Type"))

				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Len(t, body, 0)

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"Tasks": [], "NextToken": null}`))
			},
		),
	)
	defer srv.Close()

	opts := SearchTasksOptions{PullRequestID: 12345}
	shared := SharedOptions{Client: p42.NewClient(srv.URL)}

	require.NoError(t, opts.Run(context.Background(), &shared))
}

func TestSearchTasksOptionsRunWithTaskID(t *testing.T) {
	t.Parallel()
	taskID := uuid.NewString()
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/v1/tasks/search", r.URL.Path)
				require.Equal(t, "", r.URL.Query().Get("pullRequestId"))
				require.Equal(t, taskID, r.URL.Query().Get("taskId"))

				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Len(t, body, 0)

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"Tasks": [], "NextToken": null}`))
			},
		),
	)
	defer srv.Close()

	opts := SearchTasksOptions{TaskID: taskID}
	shared := SharedOptions{Client: p42.NewClient(srv.URL)}

	require.NoError(t, opts.Run(context.Background(), &shared))
}

func TestSearchTasksOptionsRunWithBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/v1/tasks/search", r.URL.Path)
				require.Equal(t, "42", r.URL.Query().Get("pullRequestId"))
				require.Equal(t, "", r.URL.Query().Get("taskId"))

				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Equal(t, map[string]any{"query": "value"}, body)

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"Tasks": [], "NextToken": null}`))
			},
		),
	)
	defer srv.Close()

	tmpFile, err := os.CreateTemp(t.TempDir(), "search-body-*.json")
	require.NoError(t, err)
	_, err = tmpFile.WriteString(`{"query":"value"}`)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	opts := SearchTasksOptions{
		PullRequestID: 42,
		JSON:          tmpFile.Name(),
	}
	shared := SharedOptions{Client: p42.NewClient(srv.URL)}

	require.NoError(t, opts.Run(context.Background(), &shared))
}

func TestSearchTasksOptionsRunWithoutCriteria(t *testing.T) {
	t.Parallel()

	opts := SearchTasksOptions{}
	shared := SharedOptions{Client: p42.NewClient("http://example.com")}

	require.Error(t, opts.Run(context.Background(), &shared))
}

func TestMoveTaskOptionsRun(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1/tenants/tenant/workstreams/source/tasks/task":
					require.Equal(t, http.MethodGet, r.Method)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(fmt.Sprintf(`{"TenantId":"tenant","TaskId":"task","Title":"","Prompt":"","Parallel":false,"RepoInfo":{},"State":"Pending","CreatedAt":"%s","UpdatedAt":"%s","Deleted":false,"Version":2}`, now, now)))
				case "/v1/tenants/tenant/workstreams/source/tasks/task/move":
					require.Equal(t, http.MethodPost, r.Method)
					require.Equal(t, "2", r.Header.Get("If-Match"))
					var body map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					require.Equal(t, map[string]any{"DestinationWorkstreamID": "dest"}, body)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(fmt.Sprintf(`{"Task":{"TenantId":"tenant","TaskId":"task","Title":"","Prompt":"","Parallel":false,"RepoInfo":{},"State":"Pending","CreatedAt":"%s","UpdatedAt":"%s","Deleted":false,"Version":3},"SourceWorkstream":{},"DestinationWorkstream":{}}`, now, now)))
				default:
					t.Fatalf("unexpected path %s", r.URL.Path)
				}
			},
		),
	)
	defer srv.Close()

	opts := MoveTaskOptions{
		TenantID:                "tenant",
		TaskID:                  "task",
		SourceWorkstreamID:      "source",
		DestinationWorkstreamID: "dest",
	}
	shared := SharedOptions{Client: p42.NewClient(srv.URL)}

	require.NoError(t, opts.Run(context.Background(), &shared))
}

func TestGetTaskGithubCredsOptionsRun(t *testing.T) {
	t.Parallel()
	tenantID := "tenant"
	taskID := "task"
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "/v1/tenants/"+tenantID+"/tasks/"+taskID+"/github-creds", r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"GithubToken":"token"}`))
			},
		),
	)
	defer srv.Close()

	opts := GetTaskGithubCredsOptions{TenantID: tenantID, TaskID: taskID}
	shared := SharedOptions{Client: p42.NewClient(srv.URL), ShowSecrets: true}

	require.NoError(t, opts.Run(context.Background(), &shared))
}

func TestGetTaskGithubCredsOptionsRunRequiresShowSecrets(t *testing.T) {
	t.Parallel()

	opts := GetTaskGithubCredsOptions{TenantID: "tenant", TaskID: "task"}
	shared := SharedOptions{Client: p42.NewClient("http://example.com")}

	require.EqualError(t, opts.Run(context.Background(), &shared), "you must specify `-s` when calling get-github-creds")
}
