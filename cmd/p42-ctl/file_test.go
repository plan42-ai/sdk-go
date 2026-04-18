package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/plan42-ai/sdk-go/p42"
	"github.com/stretchr/testify/require"
)

const (
	fileMetadataPath    = "/v1/tenants/tenant/files/file-123"
	fileDownloadURLPath = "/v1/tenants/tenant/files/file-123/download-url"
)

func TestGetFileOptionsRun(t *testing.T) {
	t.Parallel()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, fileMetadataPath, r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"TenantID":"tenant","FileID":"file-123","Name":"test.txt","Size":5,"Version":2}`))
	}))
	defer apiServer.Close()

	var stdout bytes.Buffer
	opts := GetFileOptions{TenantID: "tenant", FileID: "file-123"}
	err := opts.Run(context.Background(), &SharedOptions{Client: p42.NewClient(apiServer.URL), Stdout: &stdout, Stderr: io.Discard})
	require.NoError(t, err)
	output := stdout.String()
	require.Contains(t, output, `"TenantID": "tenant"`)
	require.Contains(t, output, `"FileID": "file-123"`)
	require.Contains(t, output, `"Name": "test.txt"`)
	require.Contains(t, output, `"Size": 5`)
	require.Contains(t, output, `"Version": 2`)
}

func TestDownloadFileOptionsRunWritesToSpecifiedPath(t *testing.T) {
	t.Parallel()

	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		_, _ = w.Write([]byte("hello world"))
	}))
	defer downloadServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case fileMetadataPath:
			require.Equal(t, http.MethodGet, r.Method)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"TenantID":"tenant","FileID":"file-123","Name":"test.txt","Size":11,"Version":2}`))
		case fileDownloadURLPath:
			require.Equal(t, http.MethodGet, r.Method)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"TenantID":"tenant","FileID":"file-123","Name":"test.txt","DownloadURL":"` + downloadServer.URL + `"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "custom.txt")

	var stderr bytes.Buffer
	opts := DownloadFileOptions{TenantID: "tenant", FileID: "file-123", Output: &outputPath}
	err := opts.Run(context.Background(), &SharedOptions{
		Client:     p42.NewClient(apiServer.URL),
		Stdout:     io.Discard,
		Stderr:     &stderr,
		HTTPClient: downloadServer.Client(),
	})
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Equal(t, "hello world", string(data))
	require.Equal(t, "downloaded file file-123 to "+outputPath+"\n", stderr.String())
}

func TestDownloadFileOptionsRunUsesDefaultFileName(t *testing.T) {
	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		_, _ = w.Write([]byte("default name"))
	}))
	defer downloadServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case fileMetadataPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"TenantID":"tenant","FileID":"file-123","Name":"downloaded.txt","Size":12,"Version":2}`))
		case fileDownloadURLPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"TenantID":"tenant","FileID":"file-123","Name":"downloaded.txt","DownloadURL":"` + downloadServer.URL + `"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	workingDir := t.TempDir()
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(workingDir))
	t.Cleanup(func() { _ = os.Chdir(originalWD) })

	var stderr bytes.Buffer
	opts := DownloadFileOptions{TenantID: "tenant", FileID: "file-123"}
	err = opts.Run(context.Background(), &SharedOptions{
		Client:     p42.NewClient(apiServer.URL),
		Stdout:     io.Discard,
		Stderr:     &stderr,
		HTTPClient: downloadServer.Client(),
	})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(workingDir, "downloaded.txt"))
	require.NoError(t, err)
	require.Equal(t, "default name", string(data))
	require.Equal(t, "downloaded file file-123 to downloaded.txt\n", stderr.String())
}

func TestDownloadFileOptionsRunWritesToStdout(t *testing.T) {
	t.Parallel()

	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		_, _ = w.Write([]byte("streamed"))
	}))
	defer downloadServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case fileMetadataPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"TenantID":"tenant","FileID":"file-123","Name":"ignored.txt","Size":8,"Version":2}`))
		case fileDownloadURLPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"TenantID":"tenant","FileID":"file-123","Name":"ignored.txt","DownloadURL":"` + downloadServer.URL + `"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	var stdout, stderr bytes.Buffer
	stdoutFlag := "-"
	opts := DownloadFileOptions{TenantID: "tenant", FileID: "file-123", Output: &stdoutFlag}
	err := opts.Run(context.Background(), &SharedOptions{
		Client:     p42.NewClient(apiServer.URL),
		Stdout:     &stdout,
		Stderr:     &stderr,
		HTTPClient: downloadServer.Client(),
	})
	require.NoError(t, err)
	require.Equal(t, "streamed", stdout.String())
	require.Empty(t, stderr.String())
}

func TestDownloadFileOptionsRunTreatsMetadataDashAsFileName(t *testing.T) {
	t.Parallel()

	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		_, _ = w.Write([]byte("dash file"))
	}))
	defer downloadServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case fileMetadataPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"TenantID":"tenant","FileID":"file-123","Name":"-","Size":9,"Version":2}`))
		case fileDownloadURLPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"TenantID":"tenant","FileID":"file-123","Name":"-","DownloadURL":"` + downloadServer.URL + `"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	var stdout, stderr bytes.Buffer
	outputPath := filepath.Join(t.TempDir(), "-")
	opts := DownloadFileOptions{TenantID: "tenant", FileID: "file-123", Output: &outputPath}

	err := opts.Run(context.Background(), &SharedOptions{
		Client:     p42.NewClient(apiServer.URL),
		Stdout:     &stdout,
		Stderr:     &stderr,
		HTTPClient: downloadServer.Client(),
	})
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Equal(t, "dash file", string(data))
	require.Empty(t, stdout.String())
	require.Equal(t, "downloaded file file-123 to "+outputPath+"\n", stderr.String())
}

func TestDownloadFileOptionsRunRejectsPathTraversalDefaultName(t *testing.T) {
	t.Parallel()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case fileMetadataPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"TenantID":"tenant","FileID":"file-123","Name":"../secret.txt","Size":9,"Version":2}`))
		case fileDownloadURLPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"TenantID":"tenant","FileID":"file-123","Name":"../secret.txt","DownloadURL":"http://example.com/download"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	opts := DownloadFileOptions{TenantID: "tenant", FileID: "file-123"}
	err := opts.Run(context.Background(), &SharedOptions{
		Client:     p42.NewClient(apiServer.URL),
		Stdout:     io.Discard,
		Stderr:     io.Discard,
		HTTPClient: http.DefaultClient,
	})
	require.EqualError(t, err, `invalid default output file name "../secret.txt"`)
}

func TestListFileOptionsRun(t *testing.T) {
	t.Parallel()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/tenant/files", r.URL.Path)

		pageToken := r.URL.Query().Get("token")
		w.Header().Set("Content-Type", "application/json")
		if pageToken == "" {
			_, _ = w.Write([]byte(`{"Items":[{"TenantID":"tenant","FileID":"file-1","Name":"first.txt","Size":5,"Version":1}],"NextToken":"page-2"}`))
			return
		}

		require.Equal(t, "page-2", pageToken)
		_, _ = w.Write([]byte(`{"Items":[{"TenantID":"tenant","FileID":"file-2","Name":"second.txt","Size":7,"Version":2}]}`))
	}))
	defer apiServer.Close()

	var stdout bytes.Buffer
	opts := ListFileOptions{TenantID: "tenant"}
	err := opts.Run(context.Background(), &SharedOptions{Client: p42.NewClient(apiServer.URL), Stdout: &stdout, Stderr: io.Discard})
	require.NoError(t, err)
	output := stdout.String()
	require.Contains(t, output, `"FileID": "file-1"`)
	require.Contains(t, output, `"Name": "first.txt"`)
	require.Contains(t, output, `"FileID": "file-2"`)
	require.Contains(t, output, `"Name": "second.txt"`)
}

func TestFormatFileSize(t *testing.T) {
	t.Parallel()

	require.Equal(t, "12B", formatFileSize(12))
	require.Equal(t, "120.0KB", formatFileSize(122880))
	require.Equal(t, "10.20MB", formatFileSize(10695475))
}

func TestUploadFileOptionsRunRejectsNameWithMultipleFiles(t *testing.T) {
	t.Parallel()

	name := "custom"
	opts := UploadFileOptions{TenantID: "tenant", Name: &name, Files: []string{"a", "b"}}
	err := opts.Run(context.Background(), &SharedOptions{
		Client:     p42.NewClient("http://example.com"),
		Stdout:     io.Discard,
		Stderr:     io.Discard,
		HTTPClient: http.DefaultClient,
	})
	require.EqualError(t, err, "--name cannot be used when uploading multiple files")
}

func TestCollectUploadCandidatesUsesStdinDefaultName(t *testing.T) {
	originalStdin := os.Stdin
	t.Cleanup(func() { os.Stdin = originalStdin })

	r, w, err := os.Pipe()
	require.NoError(t, err)
	_, err = w.Write([]byte("hello"))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	os.Stdin = r

	candidates, err := collectUploadCandidates(&UploadFileOptions{TenantID: "tenant"})
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, "-", candidates[0].path)
	require.Equal(t, "stdin", candidates[0].name)
	require.EqualValues(t, 5, candidates[0].size)
	require.Equal(t, []byte("hello"), candidates[0].data)
}

func TestUploadFileOptionsRunUploadsFile(t *testing.T) {
	t.Parallel()

	var uploadedBody []byte
	var uploadHeaders http.Header
	uploadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "*", r.Header.Get("If-None-Match"))
		uploadHeaders = r.Header.Clone()
		var err error
		uploadedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer uploadServer.Close()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.True(t, strings.HasPrefix(r.URL.Path, "/v1/tenants/tenant/files/"), "unexpected path: %s", r.URL.Path)
		fileID := strings.TrimPrefix(r.URL.Path, "/v1/tenants/tenant/files/")
		require.NotEmpty(t, fileID)

		var req map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, "test.txt", req["Name"])
		require.Equal(t, float64(5), req["Size"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		// #nosec: G705: XSS via taint analysis
		// This is a unit test, the input is controlled. This patter is not ok in production code, but it's ok here in the
		// test.
		_, _ = w.Write([]byte(`{"TenantID":"tenant","FileID":"` + fileID + `","Name":"test.txt","Size":5,"UploadURL":"` + uploadServer.URL + `/upload?sig=abc","CreatedAt":"` + now + `"}`))
	}))
	defer apiServer.Close()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello"), 0o600))

	var stdout, stderr bytes.Buffer
	opts := UploadFileOptions{TenantID: "tenant", Files: []string{filePath}}
	err := opts.Run(context.Background(), &SharedOptions{
		Client:     p42.NewClient(apiServer.URL),
		Stdout:     &stdout,
		Stderr:     &stderr,
		HTTPClient: http.DefaultClient,
	})
	require.NoError(t, err)
	require.Equal(t, []byte("hello"), uploadedBody)
	require.Equal(t, "*", uploadHeaders.Get("If-None-Match"))
	require.Equal(t, "5", uploadHeaders.Get("Content-Length"))
	require.Contains(t, stderr.String(), "creating file object for test.txt")
	require.Contains(t, stderr.String(), "uploading test.txt to s3")
}

func TestUploadToPresignedURLReturnsErrorForFailureStatus(t *testing.T) {
	t.Parallel()

	uploadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusPreconditionFailed)
		_, _ = w.Write([]byte("already exists"))
	}))
	defer uploadServer.Close()

	err := uploadToPresignedURL(context.Background(), http.DefaultClient, uploadServer.URL, http.NoBody, 0)
	require.EqualError(t, err, "s3 upload failed with status 412: already exists")
}

func TestDownloadFromPresignedURLReturnsErrorForFailureStatus(t *testing.T) {
	t.Parallel()

	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("denied"))
	}))
	defer downloadServer.Close()

	err := downloadFromPresignedURL(context.Background(), downloadServer.Client(), downloadServer.URL, io.Discard)
	require.EqualError(t, err, "s3 download failed with status 403: denied")
}

func TestDownloadToPathPreservesExistingFileOnFailure(t *testing.T) {
	t.Parallel()

	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("denied"))
	}))
	defer downloadServer.Close()

	outputPath := filepath.Join(t.TempDir(), "existing.txt")
	require.NoError(t, os.WriteFile(outputPath, []byte("keep me"), 0o600))

	err := downloadToPath(context.Background(), downloadServer.Client(), downloadServer.URL, outputPath)
	require.EqualError(t, err, "s3 download failed with status 403: denied")

	data, readErr := os.ReadFile(outputPath)
	require.NoError(t, readErr)
	require.Equal(t, "keep me", string(data))
}

func TestUploadFileOptionsRunRejectsFeatureFlagsFromStdinWhenUploadingFromStdin(t *testing.T) {
	t.Parallel()

	featureFlags := "-"
	opts := UploadFileOptions{TenantID: "tenant"}
	err := opts.Run(context.Background(), &SharedOptions{
		Client:       p42.NewClient("http://example.com"),
		FeatureFlags: &featureFlags,
		Stdout:       io.Discard,
		Stderr:       io.Discard,
		HTTPClient:   http.DefaultClient,
	})
	require.EqualError(t, err, "the --feature-flags option cannot be '-' when uploading from stdin")
}

func TestPrintUploadResults(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	printUploadResults(&stdout, []uploadResult{{name: "foo.txt", id: "id-1", size: 12}, {name: "longer.txt", id: "id-2", size: 2048}})
	output := stdout.String()
	require.Contains(t, output, "foo.txt")
	require.Contains(t, output, "id-1")
	require.Contains(t, output, "12B")
	require.Contains(t, output, "2.0KB")
}
