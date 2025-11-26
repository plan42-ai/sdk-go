package eh

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// RunnerMessage represents a message retrieved from a runner queue.
type RunnerMessage struct {
	TenantID        string    `json:"TenantID"`
	RunnerID        string    `json:"RunnerID"`
	QueueID         string    `json:"QueueID"`
	MessageID       string    `json:"MessageID"`
	CallerID        string    `json:"CallerID"`
	CallerPublicKey string    `json:"CallerPublicKey"`
	CreatedAt       time.Time `json:"CreatedAt"`
	Payload         string    `json:"Payload"`
}

// GetMessagesBatchRequest contains the parameters for GetMessagesBatch.
type GetMessagesBatchRequest struct {
	FeatureFlags

	TenantID string
	RunnerID string
	QueueID  string
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *GetMessagesBatchRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "RunnerID":
		return r.RunnerID, true
	case "QueueID":
		return r.QueueID, true
	default:
		return nil, false
	}
}

// GetMessagesBatchResponse is the response payload for GetMessagesBatch.
type GetMessagesBatchResponse struct {
	Messages []RunnerMessage `json:"Messages"`
}

// GetMessagesBatch retrieves a batch of messages for a runner queue.
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
	if req.QueueID == "" {
		return nil, fmt.Errorf("queue id is required")
	}

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"runners",
		url.PathEscape(req.RunnerID),
		"queues",
		url.PathEscape(req.QueueID),
		"messages",
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	processFeatureFlags(httpReq, req.FeatureFlags)

	err = c.authenticate(DelegatedAuthInfo{}, httpReq)
	if err != nil {
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
	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
