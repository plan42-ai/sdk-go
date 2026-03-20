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
	size *int64
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
		info, err := os.Stat(path)
		if err == nil {
			size := info.Size()
			sources = append(sources, uploadSource{name: name, path: path, size: &size})
			continue
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
	if source.size != nil {
		return uploadFileToPresignedPOST(ctx, client, uploadURL, source, *source.size)
	}

	reader, err := openUploadSource(source)
	if err != nil {
		return 0, err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return 0, err
	}
	return uploadBufferToPresignedPOST(ctx, client, uploadURL, source.name, data)
}

func uploadFileToPresignedPOST(ctx context.Context, client *awshttp.BuildableClient, uploadURL string, source uploadSource, size int64) (int64, error) {
	file, err := os.Open(source.path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var multipartBody bytes.Buffer
	writer := multipart.NewWriter(&multipartBody)
	part, err := writer.CreateFormFile("file", source.name)
	if err != nil {
		return 0, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return 0, err
	}
	if err := writer.Close(); err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &multipartBody)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, fmt.Errorf("upload request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return size, nil
}

func uploadBufferToPresignedPOST(ctx context.Context, client *awshttp.BuildableClient, uploadURL string, fileName string, data []byte) (int64, error) {
	var multipartBody bytes.Buffer
	writer := multipart.NewWriter(&multipartBody)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return 0, err
	}
	if _, err := part.Write(data); err != nil {
		return 0, err
	}
	if err := writer.Close(); err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &multipartBody)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, fmt.Errorf("upload request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return int64(len(data)), nil
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
