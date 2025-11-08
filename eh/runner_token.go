package eh

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// RunnerTokenMetadata represents metadata about a runner token.
type RunnerTokenMetadata struct {
	TokenID   string     `json:"TokenID"`
	CreatedAt time.Time  `json:"CreatedAt"`
	ExpiresAt time.Time  `json:"ExpiresAt"`
	RevokedAt *time.Time `json:"RevokedAt,omitempty"`
	TokenHash string     `json:"TokenHash"`
}

// ListRunnerTokensRequest represents the request payload for ListRunnerTokens.
type ListRunnerTokensRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID       string
	RunnerID       string
	MaxResults     *int
	NextPageToken  *string
	IncludeRevoked *bool
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *ListRunnerTokensRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "RunnerID":
		return r.RunnerID, true
	case "MaxResults":
		return evalNullable(r.MaxResults)
	case "NextPageToken":
		return evalNullable(r.NextPageToken)
	case "IncludeRevoked":
		return evalNullable(r.IncludeRevoked)
	default:
		return nil, false
	}
}

// ListRunnerTokensResponse represents the response from ListRunnerTokens.
type ListRunnerTokensResponse struct {
	NextPageToken *string               `json:"NextPageToken"`
	Items         []RunnerTokenMetadata `json:"Items"`
}

// ListRunnerTokens lists tokens for a runner.
// nolint: dupl
func (c *Client) ListRunnerTokens(ctx context.Context, req *ListRunnerTokensRequest) (*ListRunnerTokensResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.RunnerID == "" {
		return nil, fmt.Errorf("runner id is required")
	}

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"runners",
		url.PathEscape(req.RunnerID),
		"tokens",
	)
	q := u.Query()
	if req.MaxResults != nil {
		q.Set("maxResults", strconv.Itoa(*req.MaxResults))
	}
	if req.NextPageToken != nil {
		q.Set("nextPageToken", *req.NextPageToken)
	}
	if req.IncludeRevoked != nil {
		q.Set("includeRevoked", strconv.FormatBool(*req.IncludeRevoked))
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

	var out ListRunnerTokensResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
