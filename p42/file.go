package p42

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// DeleteFileRequest contains parameters for DeleteFile.
type DeleteFileRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID string `json:"-"`
	FileID   string `json:"-"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
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

// DeleteFile hard deletes file content without removing the file metadata entry.
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
