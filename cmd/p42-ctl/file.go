package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
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
	Verbose  bool     `help:"Enable verbose logging to stderr while creating file objects and uploading to S3." short:"v"`
	Files    []string `arg:"" optional:"" name:"files" help:"The files to upload. If omitted, content is read from standard input."`
}

type uploadSource struct {
	name string
	data []byte
	size int64
}

type uploadedFileRow struct {
	name   string
	fileID string
	size   int64
}

func (o *UploadFileOptions) Run(ctx context.Context, s *SharedOptions) error {
	if len(o.Files) > 1 && o.Name != nil {
		return fmt.Errorf("--name may only be used with a single file or standard input")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	sources, err := o.loadSources()
	if err != nil {
		return err
	}

	client := newUploadHTTPClient()
	rows := make([]uploadedFileRow, 0, len(sources))

	for _, source := range sources {
		if o.Verbose {
			logger.InfoContext(ctx, "creating file object", "tenant_id", o.TenantID, "name", source.name, "size", source.size)
		}

		createReq := &p42.CreateFileRequest{
			TenantID: o.TenantID,
			FileID:   uuid.NewString(),
			Name:     source.name,
		}
		if err := loadFeatureFlags(s, &createReq.FeatureFlags); err != nil {
			return err
		}
		processDelegatedAuth(s, &createReq.DelegatedAuthInfo)

		fileObj, err := s.Client.CreateFile(ctx, createReq)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to create file object for %s: %v\n", source.name, err)
			continue
		}

		if o.Verbose {
			logger.InfoContext(ctx, "uploading file to s3", "tenant_id", o.TenantID, "name", source.name, "file_id", fileObj.FileID)
		}

		if err := uploadFileToPresignedURL(ctx, client, fileObj.UploadURL, source); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to upload %s: %v\n", source.name, err)
			continue
		}

		rows = append(rows, uploadedFileRow{name: source.name, fileID: fileObj.FileID, size: source.size})
	}

	printUploadedFileRows(rows)
	return nil
}

func (o *UploadFileOptions) loadSources() ([]uploadSource, error) {
	if len(o.Files) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		name := "stdin"
		if o.Name != nil {
			name = *o.Name
		}
		if err := validateUploadSize(int64(len(data)), name); err != nil {
			return nil, err
		}
		return []uploadSource{{name: name, data: data, size: int64(len(data))}}, nil
	}

	sources := make([]uploadSource, 0, len(o.Files))
	for _, path := range o.Files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(path)
		if len(o.Files) == 1 && o.Name != nil {
			name = *o.Name
		}
		size := int64(len(data))
		if err := validateUploadSize(size, name); err != nil {
			return nil, err
		}
		sources = append(sources, uploadSource{name: name, data: data, size: size})
	}

	return sources, nil
}

func validateUploadSize(size int64, name string) error {
	if size > maxUploadFileSizeBytes {
		return fmt.Errorf("file %s exceeds the 30MB upload limit", name)
	}
	return nil
}

func newUploadHTTPClient() *awshttp.BuildableClient {
	return awshttp.NewBuildableClient()
}

func uploadFileToPresignedURL(ctx context.Context, client *awshttp.BuildableClient, uploadURL string, source uploadSource) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(source.data))
	if err != nil {
		return err
	}
	req.ContentLength = source.size

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("upload request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func printUploadedFileRows(rows []uploadedFileRow) {
	if len(rows) == 0 {
		return
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, row := range rows {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", row.name, row.fileID, formatFileSize(row.size))
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
