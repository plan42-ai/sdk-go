package eh

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// RunnerMessage represents a message destined for a runner instance.
type RunnerMessage struct {
	CallerID        string    `json:"CallerID"`
	MessageID       string    `json:"MessageID"`
	MessageType     string    `json:"MessageType"`
	CreatedAt       time.Time `json:"CreatedAt"`
	CallerPublicKey string    `json:"CallerPublicKey"`
	Payload         string    `json:"Payload"`
}

// GetMessagesBatchRequest is the request payload for GetMessagesBatch.
type GetMessagesBatchRequest struct {
	FeatureFlags
	TenantID   string `json:"-"`
	RunnerID   string `json:"-"`
	InstanceID string `json:"-"`
}

// GetField retrieves the value of a field by name.
func (r *GetMessagesBatchRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "RunnerID":
		return r.RunnerID, true
	case "InstanceID":
		return r.InstanceID, true
	default:
		return nil, false
	}
}

// GetMessagesBatchResponse is the response from GetMessagesBatch.
type GetMessagesBatchResponse struct {
	Messages []RunnerMessage `json:"Messages"`
}

// GetMessagesBatch retrieves a batch of messages for a runner instance.
func (c *Client) GetMessagesBatch(ctx context.Context, req *GetMessagesBatchRequest) (*GetMessagesBatchResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.RunnerID == "" {
		return nil, fmt.Errorf("runner id is required")
	}
	if req.InstanceID == "" {
		return nil, fmt.Errorf("instance id is required")
	}

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"runners",
		url.PathEscape(req.RunnerID),
		"instances",
		url.PathEscape(req.InstanceID),
		"messages",
		"batch",
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	processFeatureFlags(httpReq, req.FeatureFlags)

	if err := c.authenticate(DelegatedAuthInfo{}, httpReq); err != nil {
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

	var out GetMessagesBatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
