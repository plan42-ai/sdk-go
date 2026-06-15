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

// AgentType identifies the kind of sub-agent to launch.
type AgentType string

const (
	AgentTypeCodeReview AgentType = "CodeReview"
	AgentTypeCustom     AgentType = "Custom"
	AgentTypeMain       AgentType = "Main"
)

// AgentStatus represents the runtime state of a sub-agent.
type AgentStatus string

const (
	AgentStatusRunning    AgentStatus = "Running"
	AgentStatusWaiting    AgentStatus = "Waiting"
	AgentStatusTerminated AgentStatus = "Terminated"
)

// SubAgent represents a sub-agent associated with a turn.
type SubAgent struct {
	AgentID        string         `json:"AgentID"`
	AgentType      AgentType      `json:"AgentType"`
	Prompt         *string        `json:"Prompt,omitempty"`
	AutoExit       bool           `json:"AutoExit"`
	Name           string         `json:"Name"`
	Model          ModelType      `json:"Model"`
	ReasoningLevel ReasoningLevel `json:"ReasoningLevel"`
	Status         AgentStatus    `json:"Status"`
	Version        int            `json:"Version"`
	CreatedAt      time.Time      `json:"CreatedAt"`
	UpdatedAt      time.Time      `json:"UpdatedAt"`
}

// ObjectType returns the object type for ConflictError handling.
func (SubAgent) ObjectType() ObjectType { return ObjectTypeSubAgent }

// ListSubAgentsRequest is the request payload for ListSubAgents.
type ListSubAgentsRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID   string  `json:"-"`
	TaskID     string  `json:"-"`
	TurnIndex  int     `json:"-"`
	MaxResults *int    `json:"-"`
	Token      *string `json:"-"`
}

// nolint: goconst
func (r *ListSubAgentsRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "TaskID":
		return r.TaskID, true
	case "TurnIndex":
		return r.TurnIndex, true
	case "MaxResults":
		return EvalNullable(r.MaxResults)
	case "Token":
		return EvalNullable(r.Token)
	default:
		return nil, false
	}
}

// CreateSubAgentRequest is the request payload for CreateSubAgent.
type CreateSubAgentRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID       string          `json:"-"`
	TaskID         string          `json:"-"`
	TurnIndex      int             `json:"-"`
	AgentID        string          `json:"-"`
	WorkstreamID   *string         `json:"WorkstreamID,omitempty"`
	AgentType      AgentType       `json:"AgentType"`
	Prompt         *string         `json:"Prompt,omitempty"`
	AutoExit       *bool           `json:"AutoExit,omitempty"`
	Name           string          `json:"Name"`
	Model          *ModelType      `json:"Model,omitempty"`
	ReasoningLevel *ReasoningLevel `json:"ReasoningLevel,omitempty"`
}

// nolint: goconst
func (r *CreateSubAgentRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "TaskID":
		return r.TaskID, true
	case "TurnIndex":
		return r.TurnIndex, true
	case "AgentID":
		return r.AgentID, true
	case "WorkstreamID":
		return EvalNullable(r.WorkstreamID)
	case "AgentType":
		return r.AgentType, true
	case "Prompt":
		return EvalNullable(r.Prompt)
	case "AutoExit":
		return EvalNullable(r.AutoExit)
	case "Name":
		return r.Name, true
	case "Model":
		return EvalNullable(r.Model)
	case "ReasoningLevel":
		return EvalNullable(r.ReasoningLevel)
	default:
		return nil, false
	}
}

// CreateSubAgent launches a sub-agent for a turn.
func (c *Client) CreateSubAgent(ctx context.Context, req *CreateSubAgentRequest) (*SubAgent, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.TaskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	if req.AgentID == "" {
		return nil, fmt.Errorf("agent id is required")
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
		"subagents",
		url.PathEscape(req.AgentID),
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

	var out SubAgent
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListSubAgents lists the agents associated with a turn.
func (c *Client) ListSubAgents(ctx context.Context, req *ListSubAgentsRequest) (*List[*SubAgent], error) {
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

	u := c.BaseURL.JoinPath(
		"v1",
		"tenants",
		url.PathEscape(req.TenantID),
		"tasks",
		url.PathEscape(req.TaskID),
		"turns",
		strconv.Itoa(req.TurnIndex),
		"subagents",
	)
	q := u.Query()
	if req.MaxResults != nil {
		q.Set("maxResults", strconv.Itoa(*req.MaxResults))
	}
	if req.Token != nil {
		q.Set("token", *req.Token)
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

	var out List[*SubAgent]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
