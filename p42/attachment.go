package p42

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
)

// Attachment represents an image or file attached to a task or turn prompt.
type Attachment struct {
	// Name is the display name of the attachment (e.g. "screenshot.png").
	Name string `json:"Name"`

	// MimeType is the MIME type (e.g. "image/png", "image/jpeg").
	MimeType string `json:"MimeType"`

	// S3Key is the S3 object key for uploaded attachments.
	// Mutually exclusive with RepoPath.
	S3Key string `json:"S3Key,omitempty"`

	// RepoPath is the repo-relative path for checked-in files (e.g. "docs/arch.png").
	// Mutually exclusive with S3Key.
	RepoPath string `json:"RepoPath,omitempty"`
}

// UploadAttachmentRequest is the request for uploading an attachment.
type UploadAttachmentRequest struct {
	DelegatedAuthInfo
	TenantID    string
	TaskID      string
	FileName    string
	ContentType string
	Body        io.Reader
	Size        int64
}

// UploadAttachmentResponse is the response from uploading an attachment.
type UploadAttachmentResponse struct {
	Attachment Attachment `json:"Attachment"`
}

// UploadAttachment uploads a file attachment for a task.
func (c *Client) UploadAttachment(ctx context.Context, req *UploadAttachmentRequest) (*UploadAttachmentResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.TaskID == "" {
		return nil, fmt.Errorf("task id is required")
	}

	// Build multipart body.
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", req.FileName)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, req.Body); err != nil {
		return nil, fmt.Errorf("copy body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "tasks", url.PathEscape(req.TaskID), "attachments")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), &buf)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Accept", "application/json")

	if err := c.authenticate(req.DelegatedAuthInfo, httpReq); err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, decodeError(resp)
	}

	var out UploadAttachmentResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
