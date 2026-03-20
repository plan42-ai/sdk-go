package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/google/uuid"
	"github.com/plan42-ai/sdk-go/p42"
)

const maxUploadFileSizeBytes = 30 * 1024 * 1024

type FileOptions struct {
	Upload UploadFileOptions `cmd:"" help:"Upload one or more files for a tenant."`
}

type UploadFileOptions struct {
	TenantID string   `help:"The tenant to upload the file for." name:"tenant-id" short:"i" required:""`
	Name     *string  `help:"The name of the file to upload. Only valid with a single file or standard input." name:"name"`
	Files    []string `arg:"" optional:"" name:"files" help:"The files to upload. If omitted, content is read from standard input."`
}

type uploadSource struct {
	name string
	path string
}

type uploadedFile struct {
	name   string
	fileID string
	size   int64
}

func (o *UploadFileOptions) Run(ctx context.Context, s *SharedOptions) error {
	if len(o.Files) > 1 && o.Name != nil {
		return fmt.Errorf("--name may only be used with a single file or standard input")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	sources := o.loadSources()

	var featureFlags p42.FeatureFlags
	if err := loadFeatureFlags(s, &featureFlags); err != nil {
		return err
	}

	delegatedAuth := p42.DelegatedAuthInfo{}
	processDelegatedAuth(s, &delegatedAuth)

	client := newUploadHTTPClient()
	files := make([]uploadedFile, 0, len(sources))

	for _, source := range sources {
		logger.InfoContext(ctx, "creating file object", "tenant_id", o.TenantID, "name", source.name)

		createReq := &p42.CreateFileRequest{
			FeatureFlags:      featureFlags,
			DelegatedAuthInfo: delegatedAuth,
			TenantID:          o.TenantID,
			FileID:            uuid.NewString(),
			Name:              source.name,
		}

		fileObj, err := s.Client.CreateFile(ctx, createReq)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to create file object for %s: %v\n", source.name, err)
			continue
		}

		logger.InfoContext(
			ctx,
			"uploading file to s3",
			"tenant_id", o.TenantID,
			"name", source.name,
			"file_id", fileObj.FileID,
			"bucket", fileObj.Bucket,
			"key", fileObj.Key,
		)

		size, err := uploadFileToPresignedURL(ctx, client, fileObj.UploadURL, source)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to upload %s: %v\n", source.name, err)
			continue
		}

		files = append(files, uploadedFile{name: source.name, fileID: fileObj.FileID, size: size})
	}

	printUploadedFiles(files)
	return nil
}

func (o *UploadFileOptions) loadSources() []uploadSource {
	if len(o.Files) == 0 {
		name := "stdin"
		if o.Name != nil {
			name = *o.Name
		}
		return []uploadSource{{name: name, path: "-"}}
	}

	sources := make([]uploadSource, 0, len(o.Files))
	for _, path := range o.Files {
		name := filepath.Base(path)
		if len(o.Files) == 1 && o.Name != nil {
			name = *o.Name
		}
		sources = append(sources, uploadSource{name: name, path: path})
	}

	return sources
}

func openUploadSource(source uploadSource) (io.ReadCloser, error) {
	if source.path == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	file, err := os.Open(source.path)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func newUploadHTTPClient() *awshttp.BuildableClient {
	return awshttp.NewBuildableClient()
}

func uploadFileToPresignedURL(ctx context.Context, client *awshttp.BuildableClient, uploadURL string, source uploadSource) (int64, error) {
	reader, err := openUploadSource(source)
	if err != nil {
		return 0, err
	}
	defer reader.Close()

	countingReader := &countingReadCloser{ReadCloser: reader}

	method := http.MethodPost
	contentType := "application/octet-stream"
	body := io.Reader(countingReader)

	if strings.Contains(uploadURL, "X-Amz-Algorithm=") || strings.Contains(uploadURL, "X-Amz-Signature=") {
		method = http.MethodPut
	} else {
		var multipartBody multipartBodyBuffer
		writer := multipart.NewWriter(&multipartBody)
		part, err := writer.CreateFormFile("file", source.name)
		if err != nil {
			return 0, err
		}
		if _, err := io.Copy(part, countingReader); err != nil {
			return 0, err
		}
		if err := writer.Close(); err != nil {
			return 0, err
		}
		body = &multipartBody
		contentType = writer.FormDataContentType()
	}

	req, err := http.NewRequestWithContext(ctx, method, uploadURL, body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, fmt.Errorf("upload request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if countingReader.size > maxUploadFileSizeBytes {
		return 0, fmt.Errorf("file %s exceeds the 30MB upload limit", source.name)
	}
	return countingReader.size, nil
}

func printUploadedFiles(files []uploadedFile) {
	if len(files) == 0 {
		return
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, file := range files {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", file.name, file.fileID, formatFileSize(file.size))
	}
	_ = tw.Flush()
}

type countingReadCloser struct {
	io.ReadCloser
	size int64
}

func (r *countingReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	r.size += int64(n)
	return n, err
}

type multipartBodyBuffer struct {
	bytes.Buffer
}

func formatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	}
	if size < 1024*1024 {
		kb := math.Round((float64(size)/1024)*10) / 10
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", kb), "0"), ".") + "KB"
	}
	mb := math.Round((float64(size)/(1024*1024))*100) / 100
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", mb), "0"), ".") + "MB"
}
