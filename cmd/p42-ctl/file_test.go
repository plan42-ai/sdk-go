package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/plan42-ai/sdk-go/p42"
	"github.com/stretchr/testify/require"
)

func TestFormatFileSize(t *testing.T) {
	t.Parallel()

	require.Equal(t, "12B", formatFileSize(12))
	require.Equal(t, "120KB", formatFileSize(120*1024))
	require.Equal(t, "124.2KB", formatFileSize(127181))
	require.Equal(t, "1.23MB", formatFileSize(1289748))
}

func TestUploadFileOptionsRejectsNameWithMultipleFiles(t *testing.T) {
	t.Parallel()

	name := "override.txt"
	opts := UploadFileOptions{TenantID: "tenant", Name: &name, Files: []string{"a.txt", "b.txt"}}
	shared := SharedOptions{Client: p42.NewClient("http://example.com")}

	require.EqualError(t, opts.Run(context.Background(), &shared), "--name may only be used with a single file or standard input")
}

func TestUploadFileOptionsRunSingleFile(t *testing.T) {
	t.Parallel()

	var createdName string
	var uploadedBody []byte
	var uploadContentLength string

	uploadSrv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			var err error
			uploadedBody, err = io.ReadAll(r.Body)
			require.NoError(t, err)
			uploadContentLength = r.Header.Get("Content-Length")
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer uploadSrv.Close()

	apiSrv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			require.Regexp(t, `^/v1/tenants/tenant/files/[^/]+$`, r.URL.Path)

			var req p42.CreateFileRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			createdName = req.Name

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"TenantID":"tenant","FileID":"file-123","Name":%q,"UploadURL":%q,"CreatedAt":"2024-01-02T03:04:05Z"}`,
				req.Name,
				uploadSrv.URL+"/bucket-name/foo.txt?X-Amz-Signature=123",
			)
		}),
	)
	defer apiSrv.Close()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "foo.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello world"), 0o600))

	stdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = stdout }()

	opts := UploadFileOptions{TenantID: "tenant", Files: []string{filePath}}
	shared := SharedOptions{Client: p42.NewClient(apiSrv.URL)}

	require.NoError(t, opts.Run(context.Background(), &shared))
	require.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	require.NoError(t, err)

	require.Equal(t, "foo.txt", createdName)
	require.Equal(t, []byte("hello world"), uploadedBody)
	require.Equal(t, "11", uploadContentLength)
	require.Contains(t, string(output), "foo.txt")
	require.Contains(t, string(output), "file-123")
	require.Contains(t, string(output), "11B")
}

func TestPrintUploadedFileRows(t *testing.T) {
	t.Parallel()

	stdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = stdout }()

	printUploadedFileRows([]uploadedFileRow{{name: "foo.txt", fileID: "id-1", size: 12}, {name: "longer-name.txt", fileID: "id-2", size: 120 * 1024}})
	require.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	require.NoError(t, err)

	lines := bytes.Split(bytes.TrimSpace(output), []byte("\n"))
	require.Len(t, lines, 2)
	require.Contains(t, string(lines[0]), "foo.txt")
	require.Contains(t, string(lines[0]), "id-1")
	require.Contains(t, string(lines[0]), "12B")
	require.Contains(t, string(lines[1]), "longer-name.txt")
	require.Contains(t, string(lines[1]), "id-2")
	require.Contains(t, string(lines[1]), "120KB")
}
