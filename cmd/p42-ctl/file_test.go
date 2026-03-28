package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/plan42-ai/sdk-go/p42"
	"github.com/stretchr/testify/require"
)

func TestGetFileOptionsRun(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/tenants/tenant/files/file-123", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"TenantID":"tenant","FileID":"file-123","Name":"test.txt","Size":5,"Version":2}`))
	}))
	defer apiServer.Close()

	originalStdout := os.Stdout
	stdoutReader, stdoutWriter, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = stdoutWriter
	t.Cleanup(func() { os.Stdout = originalStdout })

	opts := GetFileOptions{TenantID: "tenant", FileID: "file-123"}
	err = opts.Run(context.Background(), &SharedOptions{Client: p42.NewClient(apiServer.URL)})
	require.NoError(t, stdoutWriter.Close())
	stdoutBytes, readErr := io.ReadAll(stdoutReader)
	require.NoError(t, readErr)

	require.NoError(t, err)
	output := string(stdoutBytes)
	require.Contains(t, output, `"TenantID": "tenant"`)
	require.Contains(t, output, `"FileID": "file-123"`)
	require.Contains(t, output, `"Name": "test.txt"`)
	require.Contains(t, output, `"Size": 5`)
	require.Contains(t, output, `"Version": 2`)
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
	err := opts.Run(context.Background(), &SharedOptions{Client: p42.NewClient("http://example.com")})
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
		require.Equal(t, "/v1/tenants/tenant/files/file-123", r.URL.Path)

		var req map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, "test.txt", req["Name"])
		require.Equal(t, float64(5), req["Size"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"TenantID":"tenant","FileID":"file-123","Name":"test.txt","Size":5,"UploadURL":"` + uploadServer.URL + `/upload?sig=abc","CreatedAt":"` + now + `"}`))
	}))
	defer apiServer.Close()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello"), 0o600))

	originalNewFileID := newFileID
	newFileID = func() string { return "file-123" }
	t.Cleanup(func() { newFileID = originalNewFileID })

	originalStderr := os.Stderr
	stderrReader, stderrWriter, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = stderrWriter
	t.Cleanup(func() { os.Stderr = originalStderr })

	opts := UploadFileOptions{TenantID: "tenant", Files: []string{filePath}}
	err = opts.Run(context.Background(), &SharedOptions{Client: p42.NewClient(apiServer.URL)})
	require.NoError(t, stderrWriter.Close())
	stderrBytes, readErr := io.ReadAll(stderrReader)
	require.NoError(t, readErr)

	require.NoError(t, err)
	require.Equal(t, []byte("hello"), uploadedBody)
	require.Equal(t, "*", uploadHeaders.Get("If-None-Match"))
	require.Equal(t, "5", uploadHeaders.Get("Content-Length"))
	require.Contains(t, string(stderrBytes), "creating file object for test.txt")
	require.Contains(t, string(stderrBytes), "uploading test.txt to s3")
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

func TestUploadFileOptionsRunRejectsFeatureFlagsFromStdinWhenUploadingFromStdin(t *testing.T) {
	t.Parallel()

	featureFlags := "-"
	opts := UploadFileOptions{TenantID: "tenant"}
	err := opts.Run(context.Background(), &SharedOptions{Client: p42.NewClient("http://example.com"), FeatureFlags: &featureFlags})
	require.EqualError(t, err, "the --feature-flags option cannot be '-' when uploading from stdin")
}

func TestPrintUploadResults(t *testing.T) {
	originalStdout := os.Stdout
	stdoutReader, stdoutWriter, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = stdoutWriter
	t.Cleanup(func() { os.Stdout = originalStdout })

	printUploadResults([]uploadResult{{name: "foo.txt", id: "id-1", size: 12}, {name: "longer.txt", id: "id-2", size: 2048}})
	require.NoError(t, stdoutWriter.Close())

	stdoutBytes, err := io.ReadAll(stdoutReader)
	require.NoError(t, err)
	output := string(stdoutBytes)
	require.Contains(t, output, "foo.txt")
	require.Contains(t, output, "id-1")
	require.Contains(t, output, "12B")
	require.Contains(t, output, "2.0KB")
}
