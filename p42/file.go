package p42

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// MaxFileSize is the maximum file size in bytes (30 MB).
const MaxFileSize = 31457280

// CreateFileRequest is the request for CreateFile.
type CreateFileRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID string `json:"-"`
	FileID   string `json:"-"`
	Name     string `json:"Name"`
	Size     int    `json:"Size"`
}

// GetField retrieves the value of a field by name.
// nolint:goconst
func (r *CreateFileRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "FileID":
		return r.FileID, true
	case "Name":
		return r.Name, true
	case "Size":
		return r.Size, true
	default:
		return nil, false
	}
}

// CreateFileResponse contains metadata about a file object and its upload URL.
type CreateFileResponse struct {
	TenantID  string    `json:"TenantID"`
	FileID    string    `json:"FileID"`
	Name      string    `json:"Name"`
	Size      int       `json:"Size"`
	UploadURL string    `json:"UploadURL"`
	CreatedAt time.Time `json:"CreatedAt"`
}

func (f File) ObjectType() ObjectType {
	return ObjectTypeFile
}

// CreateFile creates a new file object and returns a presigned upload URL.
// nolint: dupl
func (c *Client) CreateFile(ctx context.Context, req *CreateFileRequest) (*CreateFileResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.FileID == "" {
		return nil, fmt.Errorf("file id is required")
	}
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.Size < 1 || req.Size > MaxFileSize {
		return nil, fmt.Errorf("size must be between 1 and %d", MaxFileSize)
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "files", url.PathEscape(req.FileID))

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(req); err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), &buf)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	processFeatureFlags(httpReq, req.FeatureFlags)

	if err := c.authenticate(req.DelegatedAuthInfo, httpReq); err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, decodeError(resp)
	}

	var out CreateFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DownloadURL contains a presigned URL for downloading a file.
type DownloadURL struct {
	TenantID    string    `json:"TenantID"`
	FileID      string    `json:"FileID"`
	Name        string    `json:"Name"`
	CreatedAt   time.Time `json:"CreatedAt"`
	DownloadURL string    `json:"DownloadURL"`
}

// GetDownloadURLRequest is the request payload for GetDownloadURL.
type GetDownloadURLRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID string `json:"-"`
	FileID   string `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *GetDownloadURLRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "FileID":
		return r.FileID, true
	default:
		return nil, false
	}
}

// GetDownloadURL retrieves a presigned URL for downloading a file.
func (c *Client) GetDownloadURL(ctx context.Context, req *GetDownloadURLRequest) (*DownloadURL, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.FileID == "" {
		return nil, fmt.Errorf("file id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "files", url.PathEscape(req.FileID), "download-url")

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	processFeatureFlags(httpReq, req.FeatureFlags)

	if err := c.authenticate(req.DelegatedAuthInfo, httpReq); err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var out DownloadURL
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return &out, nil
}

// ListFilesRequest represents the request payload for ListFiles.
type ListFilesRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID   string
	MaxResults *int
	NextToken  *string
}

// GetField retrieves the value of a field by name.
// nolint:goconst
func (r *ListFilesRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "MaxResults":
		return EvalNullable(r.MaxResults)
	case "NextToken":
		return EvalNullable(r.NextToken)
	default:
		return nil, false
	}
}

// ListFiles lists tenant-owned files with pagination support.
func (c *Client) ListFiles(ctx context.Context, req *ListFilesRequest) (*List[*File], error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.MaxResults != nil && (*req.MaxResults < 1 || *req.MaxResults > 500) {
		return nil, fmt.Errorf("max results must be between 1 and 500")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "files")
	q := u.Query()
	if req.MaxResults != nil {
		q.Set("maxResults", strconv.Itoa(*req.MaxResults))
	}
	if req.NextToken != nil {
		q.Set("token", *req.NextToken)
	}
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	processFeatureFlags(httpReq, req.FeatureFlags)

	if err := c.authenticate(req.DelegatedAuthInfo, httpReq); err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var out List[*File]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetFileRequest represents the request payload for GetFile.
type GetFileRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID string `json:"-"`
	FileID   string `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint:goconst
func (r *GetFileRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "FileID":
		return r.FileID, true
	default:
		return nil, false
	}
}

// GetFile retrieves metadata for a specific file object.
// nolint: dupl
func (c *Client) GetFile(ctx context.Context, req *GetFileRequest) (*File, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.FileID == "" {
		return nil, fmt.Errorf("file id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "files", url.PathEscape(req.FileID))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	processFeatureFlags(httpReq, req.FeatureFlags)

	if err := c.authenticate(req.DelegatedAuthInfo, httpReq); err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var out File
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return &out, nil
}

// UpdateFileRequest represents the request payload for UpdateFile.
type UpdateFileRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID string `json:"-"`
	FileID   string `json:"-"`
	Version  int    `json:"-"`

	IsMalicious        *bool               `json:"IsMalicious,omitempty"`
	ModerationScanInfo *ModerationScanInfo `json:"ModerationScanInfo,omitempty"`
	ContentType        *string             `json:"ContentType,omitempty"`
}

func (r *UpdateFileRequest) GetVersion() int {
	return r.Version
}

func (r *UpdateFileRequest) IsEmptyUpdate() bool {
	return r.IsMalicious == nil && r.ModerationScanInfo == nil && r.ContentType == nil
}

// GetField retrieves the value of a field by name.
// nolint:goconst
func (r *UpdateFileRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "FileID":
		return r.FileID, true
	case "Version":
		return r.Version, true
	case "IsMalicious":
		return EvalNullable(r.IsMalicious)
	case "ModerationScanInfo":
		return EvalNullable(r.ModerationScanInfo)
	case "ContentType":
		return EvalNullable(r.ContentType)
	default:
		return nil, false
	}
}

// UpdateFile updates mutable metadata for a file object.
// nolint: dupl
func (c *Client) UpdateFile(ctx context.Context, req *UpdateFileRequest) (*File, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.FileID == "" {
		return nil, fmt.Errorf("file id is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "files", url.PathEscape(req.FileID))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("If-Match", strconv.Itoa(req.Version))
	processFeatureFlags(httpReq, req.FeatureFlags)

	if err := c.authenticate(req.DelegatedAuthInfo, httpReq); err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var out File
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return &out, nil
}

// DeleteFileRequest represents the request payload for DeleteFile.
type DeleteFileRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID string
	FileID   string
}

// GetField retrieves the value of a field by name.
// nolint:goconst
func (r *DeleteFileRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "FileID":
		return r.FileID, true
	default:
		return nil, false
	}
}

// DeleteFile hard deletes file contents while preserving metadata.
func (c *Client) DeleteFile(ctx context.Context, req *DeleteFileRequest) error {
	if req == nil {
		return fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return fmt.Errorf("tenant id is required")
	}
	if req.FileID == "" {
		return fmt.Errorf("file id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "files", url.PathEscape(req.FileID))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, u.String(), nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Accept", "application/json")
	processFeatureFlags(httpReq, req.FeatureFlags)

	if err := c.authenticate(req.DelegatedAuthInfo, httpReq); err != nil {
		return err
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return decodeError(resp)
	}

	return nil
}
