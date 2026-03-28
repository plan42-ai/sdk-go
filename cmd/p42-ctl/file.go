package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/plan42-ai/sdk-go/internal/util"
	"github.com/plan42-ai/sdk-go/p42"
)

type FileOptions struct {
	List   ListFileOptions   `cmd:"" help:"List files owned by a tenant."`
	Get    GetFileOptions    `cmd:"" help:"Fetch metadata for a tenant-owned file."`
	Upload UploadFileOptions `cmd:"" help:"Upload one or more files for a tenant."`
}

type ListFileOptions struct {
	TenantID string `help:"The id of the tenant to list files for." name:"tenant-id" short:"i" required:""`
}

func (o *ListFileOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &p42.ListFilesRequest{TenantID: o.TenantID}
	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return err
	}

	var token *string
	for {
		req.NextToken = token
		processDelegatedAuth(s, &req.DelegatedAuthInfo)

		resp, err := s.Client.ListFiles(ctx, req)
		if err != nil {
			return err
		}

		for _, file := range resp.Items {
			if err := printJSON(file); err != nil {
				return err
			}
		}

		if resp.NextToken == nil {
			break
		}
		token = resp.NextToken
	}

	return nil
}

type GetFileOptions struct {
	TenantID string `help:"The id of the tenant that owns the file to fetch." name:"tenant-id" short:"i" required:""`
	FileID   string `help:"The id of the file to fetch." name:"file-id" short:"f" required:""`
}

func (o *GetFileOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &p42.GetFileRequest{
		TenantID: o.TenantID,
		FileID:   o.FileID,
	}
	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	file, err := s.Client.GetFile(ctx, req)
	if err != nil {
		return err
	}

	return printJSON(file)
}

type UploadFileOptions struct {
	TenantID string   `help:"The id of the tenant to upload the file for." name:"tenant-id" short:"i" required:""`
	Name     *string  `help:"The name of the file to upload. Only valid with a single file or when reading from stdin." name:"name"`
	Files    []string `arg:"" optional:"" help:"Files to upload. If omitted, reads from stdin."`
}

type uploadCandidate struct {
	path string
	name string
	size int64
	data []byte
}

type uploadResult struct {
	name string
	id   string
	size int64
}

var newFileID = uuid.NewString

func (o *UploadFileOptions) Run(ctx context.Context, s *SharedOptions) error {
	if o.Name != nil && len(o.Files) > 1 {
		return fmt.Errorf("--name cannot be used when uploading multiple files")
	}
	if len(o.Files) == 0 && s.FeatureFlags != nil && *s.FeatureFlags == "-" {
		return fmt.Errorf("the --feature-flags option cannot be '-' when uploading from stdin")
	}

	var flags p42.FeatureFlags
	if err := loadFeatureFlags(s, &flags); err != nil {
		return err
	}

	candidates, err := collectUploadCandidates(o)
	if err != nil {
		return err
	}

	results := make([]uploadResult, 0, len(candidates))
	var failed bool

	for _, candidate := range candidates {
		fileID := newFileID()
		_, _ = fmt.Fprintf(os.Stderr, "creating file object for %s (%s)\n", candidate.name, formatFileSize(candidate.size))

		createReq := &p42.CreateFileRequest{
			TenantID: o.TenantID,
			FileID:   fileID,
			Name:     candidate.name,
			Size:     int(candidate.size),
		}
		createReq.FeatureFlags = flags
		processDelegatedAuth(s, &createReq.DelegatedAuthInfo)

		reader, err := openUploadCandidate(candidate)
		if err != nil {
			failed = true
			_, _ = fmt.Fprintf(os.Stderr, "failed to open %s: %v\n", candidate.name, err)
			continue
		}
		// NOTE: This can close "reader" twice. Once inside uploadToPresignedUrl (which calls "Do" on the http request),
		// and again when returning from Run(). That's ok. We ignore errors from the second call, and both *os.CreateFileResponse and
		// io.NopCloser are safe to call more than  once. The file reader will return an error the second time, but we
		// ignore that, so it's ok.
		defer util.Close(reader)

		created, err := s.Client.CreateFile(ctx, createReq)
		if err != nil {
			failed = true
			_, _ = fmt.Fprintf(os.Stderr, "failed to create file object for %s: %v\n", candidate.name, err)
			continue
		}

		_, _ = fmt.Fprintf(os.Stderr, "uploading %s to s3\n", candidate.name)
		if err := uploadToPresignedURL(ctx, http.DefaultClient, created.UploadURL, reader, candidate.size); err != nil {
			failed = true
			_, _ = fmt.Fprintf(os.Stderr, "failed to upload %s: %v\n", candidate.name, normalizeUploadError(err))
			continue
		}

		results = append(results, uploadResult{name: created.Name, id: created.FileID, size: candidate.size})
	}

	printUploadResults(results)
	if failed {
		return fmt.Errorf("one or more file uploads failed")
	}
	return nil
}

func collectUploadCandidates(o *UploadFileOptions) ([]uploadCandidate, error) {
	if len(o.Files) == 0 {
		name := "stdin"
		if o.Name != nil {
			name = *o.Name
		}

		data, err := readStdinForUpload()
		if err != nil {
			return nil, err
		}
		return []uploadCandidate{{
			path: "-",
			name: name,
			size: int64(len(data)),
			data: data,
		}}, nil
	}

	results := make([]uploadCandidate, 0, len(o.Files))
	for _, path := range o.Files {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			return nil, fmt.Errorf("%s is a directory", path)
		}
		if info.Size() < 1 || info.Size() > p42.MaxFileSize {
			return nil, fmt.Errorf("%s size must be between 1 and %d", path, p42.MaxFileSize)
		}

		name := filepath.Base(path)
		if len(o.Files) == 1 && o.Name != nil {
			name = *o.Name
		}

		pathCopy := path
		results = append(results, uploadCandidate{
			path: pathCopy,
			name: name,
			size: info.Size(),
		})
	}

	return results, nil
}

func openUploadCandidate(candidate uploadCandidate) (io.ReadCloser, error) {
	if candidate.path == "-" {
		return io.NopCloser(bytes.NewReader(candidate.data)), nil
	}

	return os.Open(candidate.path)
}

func readStdinForUpload() ([]byte, error) {
	reader := io.LimitReader(os.Stdin, p42.MaxFileSize+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(data) < 1 || len(data) > p42.MaxFileSize {
		return nil, fmt.Errorf("stdin size must be between 1 and %d", p42.MaxFileSize)
	}
	return data, nil
}

func uploadToPresignedURL(ctx context.Context, client *http.Client, uploadURL string, body io.Reader, size int64) error {
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, body)
	if err != nil {
		return err
	}
	req.ContentLength = size
	req.Header.Set("If-None-Match", "*")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if readErr != nil {
			return fmt.Errorf("s3 upload failed with status %d", resp.StatusCode)
		}
		message := strings.TrimSpace(string(bodyBytes))
		if message == "" {
			return fmt.Errorf("s3 upload failed with status %d", resp.StatusCode)
		}
		return fmt.Errorf("s3 upload failed with status %d: %s", resp.StatusCode, message)
	}

	return nil
}

func printUploadResults(results []uploadResult) {
	if len(results) == 0 {
		return
	}

	nameWidth := len(results[0].name)
	idWidth := len(results[0].id)
	for _, result := range results[1:] {
		if len(result.name) > nameWidth {
			nameWidth = len(result.name)
		}
		if len(result.id) > idWidth {
			idWidth = len(result.id)
		}
	}

	for _, result := range results {
		_, _ = fmt.Fprintf(os.Stdout, "%-*s  %-*s  %s\n", nameWidth, result.name, idWidth, result.id, formatFileSize(result.size))
	}
}

func formatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	}
	if size < 1024*1024 {
		value := math.Round((float64(size)/1024)*10) / 10
		return fmt.Sprintf("%.1fKB", value)
	}
	value := math.Round((float64(size)/(1024*1024))*100) / 100
	return fmt.Sprintf("%.2fMB", value)
}

func normalizeUploadError(err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr
	}
	return err
}
