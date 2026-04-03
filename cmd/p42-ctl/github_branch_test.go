package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/plan42-ai/sdk-go/p42"
	"github.com/stretchr/testify/require"
)

func TestListGithubBranchesOptionsRun(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "/v1/tenants/tenant/github-connections/conn/orgs/myorg/repos/myrepo/branches", r.URL.Path)

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"Items":["main","develop","feature-x"],"NextToken":null}`))
			},
		),
	)
	defer srv.Close()

	opts := ListGithubBranchesOptions{
		TenantID:     "tenant",
		ConnectionID: "conn",
		Org:          "myorg",
		Repo:         "myrepo",
	}
	shared := SharedOptions{Client: p42.NewClient(srv.URL), Stdout: io.Discard, Stderr: io.Discard}

	require.NoError(t, opts.Run(context.Background(), &shared))
}

func TestListGithubBranchesOptionsRunWithSearch(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "feat", r.URL.Query().Get("search"))

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"Items":["feature-x"],"NextToken":null}`))
			},
		),
	)
	defer srv.Close()

	search := "feat"
	opts := ListGithubBranchesOptions{
		TenantID:     "tenant",
		ConnectionID: "conn",
		Org:          "myorg",
		Repo:         "myrepo",
		Search:       &search,
	}
	shared := SharedOptions{Client: p42.NewClient(srv.URL), Stdout: io.Discard, Stderr: io.Discard}

	require.NoError(t, opts.Run(context.Background(), &shared))
}

func TestListGithubBranchesOptionsRunPagination(t *testing.T) {
	t.Parallel()
	calls := 0
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				if calls == 1 {
					_, _ = w.Write([]byte(`{"Items":["main"],"NextToken":"page2"}`))
				} else {
					require.Equal(t, "page2", r.URL.Query().Get("token"))
					_, _ = w.Write([]byte(`{"Items":["develop"],"NextToken":null}`))
				}
			},
		),
	)
	defer srv.Close()

	opts := ListGithubBranchesOptions{
		TenantID:     "tenant",
		ConnectionID: "conn",
		Org:          "myorg",
		Repo:         "myrepo",
	}
	shared := SharedOptions{Client: p42.NewClient(srv.URL), Stdout: io.Discard, Stderr: io.Discard}

	require.NoError(t, opts.Run(context.Background(), &shared))
	require.Equal(t, 2, calls)
}

func TestGetDefaultBranchesOptionsRun(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/v1/tenants/tenant/github-connections/conn/default-branches", r.URL.Path)

				var body struct {
					Repos []string `json:"Repos"`
				}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Equal(t, []string{"org/repo1", "org/repo2"}, body.Repos)

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"Items":[{"Repo":"org/repo1","DefaultBranch":"main"},{"Repo":"org/repo2","DefaultBranch":"develop"}]}`))
			},
		),
	)
	defer srv.Close()

	opts := GetDefaultBranchesOptions{
		TenantID:     "tenant",
		ConnectionID: "conn",
		Repos:        []string{"org/repo1", "org/repo2"},
	}
	shared := SharedOptions{Client: p42.NewClient(srv.URL), Stdout: io.Discard, Stderr: io.Discard}

	require.NoError(t, opts.Run(context.Background(), &shared))
}

func TestGetDefaultBranchesOptionsRunError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"Message":"internal error","ErrorType":"InternalServerError"}`))
			},
		),
	)
	defer srv.Close()

	opts := GetDefaultBranchesOptions{
		TenantID:     "tenant",
		ConnectionID: "conn",
		Repos:        []string{"org/repo1"},
	}
	shared := SharedOptions{Client: p42.NewClient(srv.URL), Stdout: io.Discard, Stderr: io.Discard}

	require.Error(t, opts.Run(context.Background(), &shared))
}
