package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

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
