package eh

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/debugging-sucks/ecies"
)

type WrappedSecret interface {
	EncryptionAlgorithm() string
}

// RunnerMessage represents a message retrieved from a runner queue.
type RunnerMessage struct {
	TenantID        string        `json:"TenantID"`
	RunnerID        string        `json:"RunnerID"`
	QueueID         string        `json:"QueueID"`
	MessageID       string        `json:"MessageID"`
	CallerID        string        `json:"CallerID"`
	CallerPublicKey string        `json:"CallerPublicKey"`
	CreatedAt       time.Time     `json:"CreatedAt"`
	Payload         WrappedSecret `json:"Payload"`
}

func (m *RunnerMessage) UnmarshalJSON(bytes []byte) error {
	type runnerMessage struct {
		TenantID        string          `json:"TenantID"`
		RunnerID        string          `json:"RunnerID"`
		QueueID         string          `json:"QueueID"`
		MessageID       string          `json:"MessageID"`
		CallerID        string          `json:"CallerID"`
		CallerPublicKey string          `json:"CallerPublicKey"`
		CreatedAt       time.Time       `json:"CreatedAt"`
		Payload         json.RawMessage `json:"Payload"`
	}
	var tmp runnerMessage
	err := json.Unmarshal(bytes, &tmp)
	if err != nil {
		return err
	}

	type payloadWrapper struct {
		EncryptionAlgorithm string `json:"EncryptionAlgorithm"`
	}
	var pw payloadWrapper
	err = json.Unmarshal(tmp.Payload, &pw)
	if err != nil {
		return err
	}
	var payload WrappedSecret
	switch pw.EncryptionAlgorithm {
	case ecies.EciesCofactorVariableIVX963SHA256AESGCM:
		payload = &ecies.WrappedSecret{}
	default:
		return fmt.Errorf("unknown encryption algorithm: %s", pw.EncryptionAlgorithm)
	}
	err = json.Unmarshal(tmp.Payload, payload)
	if err != nil {
		return err
	}
	*m = RunnerMessage{
		TenantID:        tmp.TenantID,
		RunnerID:        tmp.RunnerID,
		QueueID:         tmp.QueueID,
		MessageID:       tmp.MessageID,
		CallerID:        tmp.CallerID,
		CallerPublicKey: tmp.CallerPublicKey,
		CreatedAt:       tmp.CreatedAt,
		Payload:         payload,
	}
	return nil
}

func (RunnerMessage) ObjectType() ObjectType {
	return ObjectTypeRunnerMessage
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
	Messages []*RunnerMessage `json:"Messages"`
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
