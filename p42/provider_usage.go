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

type ProviderUsageEvent struct {
	TenantID                      string         `json:"TenantID"`
	WorkstreamID                  *string        `json:"WorkstreamID,omitempty"`
	TaskID                        string         `json:"TaskID"`
	TurnIndex                     int            `json:"TurnIndex"`
	IterationIndex                int            `json:"IterationIndex"`
	Provider                      string         `json:"Provider"`
	ProviderModelID               string         `json:"ProviderModelID"`
	ResponseID                    *string        `json:"ResponseID,omitempty"`
	PromptTokens                  int            `json:"PromptTokens"`
	CompletionTokens              int            `json:"CompletionTokens"`
	TotalTokens                   int            `json:"TotalTokens"`
	CachedReadInputTokens         int            `json:"CachedReadInputTokens"`
	CacheCreationInputTokensByTTL map[string]int `json:"CacheCreationInputTokensByTTL,omitempty"`
	ReasoningTokens               int            `json:"ReasoningTokens"`
	IngestionInputTokens          int            `json:"IngestionInputTokens"`
	RequestStartedAt              *time.Time     `json:"RequestStartedAt,omitempty"`
	ResponseCompletedAt           time.Time      `json:"ResponseCompletedAt"`
	CreatedAt                     time.Time      `json:"CreatedAt"`
	UpdatedAt                     time.Time      `json:"UpdatedAt"`
	Version                       int            `json:"Version"`
}

func (ProviderUsageEvent) ObjectType() ObjectType { return ObjectTypeProviderUsageEvent }

type WriteProviderUsageRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID                      string         `json:"-"`
	TaskID                        string         `json:"-"`
	TurnIndex                     int            `json:"-"`
	IterationIndex                int            `json:"-"`
	WorkstreamID                  *string        `json:"-"`
	Provider                      string         `json:"Provider"`
	ProviderModelID               string         `json:"ProviderModelID"`
	ResponseID                    *string        `json:"ResponseID,omitempty"`
	PromptTokens                  int            `json:"PromptTokens"`
	CompletionTokens              int            `json:"CompletionTokens"`
	TotalTokens                   int            `json:"TotalTokens"`
	CachedReadInputTokens         int            `json:"CachedReadInputTokens"`
	CacheCreationInputTokensByTTL map[string]int `json:"CacheCreationInputTokensByTTL,omitempty"`
	ReasoningTokens               int            `json:"ReasoningTokens"`
	IngestionInputTokens          int            `json:"IngestionInputTokens"`
	RequestStartedAt              *time.Time     `json:"RequestStartedAt,omitempty"`
	ResponseCompletedAt           time.Time      `json:"ResponseCompletedAt"`
}

// nolint: goconst
func (r *WriteProviderUsageRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "TaskID":
		return r.TaskID, true
	case "TurnIndex":
		return r.TurnIndex, true
	case "IterationIndex":
		return r.IterationIndex, true
	case "WorkstreamID":
		return EvalNullable(r.WorkstreamID)
	case "Provider":
		return r.Provider, true
	case "ProviderModelID":
		return r.ProviderModelID, true
	case "ResponseID":
		return EvalNullable(r.ResponseID)
	case "PromptTokens":
		return r.PromptTokens, true
	case "CompletionTokens":
		return r.CompletionTokens, true
	case "TotalTokens":
		return r.TotalTokens, true
	case "CachedReadInputTokens":
		return r.CachedReadInputTokens, true
	case "CacheCreationInputTokensByTTL":
		return r.CacheCreationInputTokensByTTL, true
	case "ReasoningTokens":
		return r.ReasoningTokens, true
	case "IngestionInputTokens":
		return r.IngestionInputTokens, true
	case "RequestStartedAt":
		return EvalNullable(r.RequestStartedAt)
	case "ResponseCompletedAt":
		return r.ResponseCompletedAt, true
	default:
		return nil, false
	}
}

func (c *Client) WriteProviderUsage(ctx context.Context, req *WriteProviderUsageRequest) (*ProviderUsageEvent, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req.TaskID == "" {
		return nil, fmt.Errorf("task id is required")
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
		"iterations",
		strconv.Itoa(req.IterationIndex),
		"provider-usage",
	)
	q := u.Query()
	if req.WorkstreamID != nil {
		q.Set("workstreamID", *req.WorkstreamID)
	}
	u.RawQuery = q.Encode()

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

	if resp.StatusCode != http.StatusCreated {
		return nil, decodeError(resp)
	}

	var out ProviderUsageEvent
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
