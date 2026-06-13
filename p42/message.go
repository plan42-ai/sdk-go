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

	"github.com/plan42-ai/ecies"
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

// MessageFromType identifies the type of sender for turn messages.
type MessageFromType string

const (
	MessageFromTypeAgent  MessageFromType = "Agent"
	MessageFromTypeTenant MessageFromType = "Tenant"
)

// SentMessage represents a message sent between agents or from a tenant to the main agent.
type SentMessage struct {
	MessageID    string    `json:"MessageID"`
	FromTenantID *string   `json:"FromTenantID,omitempty"`
	FromAgentID  *string   `json:"FromAgentID,omitempty"`
	To           []string  `json:"To"`
	Message      string    `json:"Message"`
	CreatedAt    time.Time `json:"CreatedAt"`
}

// SendMessageRequest contains the parameters for SendMessage.
type SendMessageRequest struct {
	FeatureFlags
	DelegatedAuthInfo

	TenantID  string          `json:"-"`
	TaskID    string          `json:"-"`
	TurnIndex int             `json:"-"`
	MessageID string          `json:"-"`
	From      string          `json:"From"`
	FromType  MessageFromType `json:"FromType"`
	To        []string        `json:"To"`
	Message   string          `json:"Message"`
}

// GetField retrieves the value of a field by name.
// nolint: goconst
func (r *SendMessageRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "TaskID":
		return r.TaskID, true
	case "TurnIndex":
		return r.TurnIndex, true
	case "MessageID":
		return r.MessageID, true
	case "From":
		return r.From, true
	case "FromType":
		return r.FromType, true
	case "To":
		return r.To, true
	case "Message":
		return r.Message, true
	default:
		return nil, false
	}
}

// SendMessage sends a message to one or more agents participating in a turn.
func (c *Client) SendMessage(ctx context.Context, req *SendMessageRequest) (*SentMessage, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.TaskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	if req.TurnIndex < 0 {
		return nil, fmt.Errorf("turn index is required")
	}
	if req.MessageID == "" {
		return nil, fmt.Errorf("message id is required")
	}
	if req.From == "" {
		return nil, fmt.Errorf("from is required")
	}
	if req.FromType == "" {
		return nil, fmt.Errorf("from type is required")
	}
	if len(req.To) == 0 {
		return nil, fmt.Errorf("to is required")
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"tasks",
		url.PathEscape(req.TaskID),
		"turns",
		strconv.Itoa(req.TurnIndex),
		"messages",
		url.PathEscape(req.MessageID),
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
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

	var out SentMessage
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMessagesBatchRequest contains the parameters for GetMessagesBatch.
type GetMessagesBatchRequest struct {
	FeatureFlags

	TenantID       string
	RunnerID       string
	QueueID        string
	MaxWaitSeconds *int
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
	case "MaxWaitSeconds":
		return EvalNullable(r.MaxWaitSeconds)
	default:
		return nil, false
	}
}

// GetMessagesBatchResponse is the response payload for GetMessagesBatch.
type GetMessagesBatchResponse struct {
	Messages []*RunnerMessage `json:"Messages"`
}

// GetMessagesBatch retrieves a batch of messages for a runner queue.
func (c *Client) GetMessagesBatch(ctx context.Context, req *GetMessagesBatchRequest) (
	*GetMessagesBatchResponse,
	error,
) {
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

	if req.MaxWaitSeconds != nil {
		q := u.Query()
		q.Set("maxWaitSeconds", fmt.Sprintf("%d", *req.MaxWaitSeconds))
		u.RawQuery = q.Encode()
	}

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
