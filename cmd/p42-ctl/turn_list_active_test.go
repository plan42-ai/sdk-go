package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/plan42-ai/sdk-go/internal/util"
	"github.com/plan42-ai/sdk-go/p42"
	"github.com/stretchr/testify/require"
)

func TestListActiveTurnsOptionsRun(t *testing.T) {
	t.Parallel()

	fromTime := time.Date(2024, 7, 10, 10, 0, 0, 0, time.UTC)
	toTime := time.Date(2024, 7, 10, 12, 0, 0, 0, time.UTC)

	callCount := 0
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				callCount++
				require.Equal(t, "/v1/turns", r.URL.Path)
				require.Equal(t, "3", r.URL.Query().Get("partition"))
				require.Equal(t, fromTime.UTC().Format(time.RFC3339Nano), r.URL.Query().Get("minUpdatedAt"))
				require.Equal(t, toTime.UTC().Format(time.RFC3339Nano), r.URL.Query().Get("maxUpdatedAt"))

				if callCount == 1 {
					require.Equal(t, "", r.URL.Query().Get("token"))
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"Items": [], "NextToken": "next"}`))
					return
				}

				require.Equal(t, "next", r.URL.Query().Get("token"))
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"Items": []}`))
			},
		),
	)
	defer srv.Close()

	opts := ListActiveTurnsOptions{
		Partition: 3,
		From:      util.Pointer(fromTime.Format(time.RFC3339)),
		To:        toTime.Format(time.RFC3339),
	}
	shared := SharedOptions{Client: p42.NewClient(srv.URL), Stdout: io.Discard, Stderr: io.Discard}

	require.NoError(t, opts.Run(context.Background(), &shared))
	require.Equal(t, 2, callCount)
}
