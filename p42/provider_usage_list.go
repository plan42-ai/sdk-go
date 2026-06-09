package p42

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ListProviderUsageEventsRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID     string
	MaxResults   *int
	Token        *string
	WorkstreamID *string
	TaskID       *string
	Provider     *string
	Model        *string
	StartTime    *time.Time
	EndTime      *time.Time
}

// nolint: goconst
func (r *ListProviderUsageEventsRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "MaxResults":
		return EvalNullable(r.MaxResults)
	case "Token":
		return EvalNullable(r.Token)
	case "WorkstreamID":
		return EvalNullable(r.WorkstreamID)
	case "TaskID":
		return EvalNullable(r.TaskID)
	case "Provider":
		return EvalNullable(r.Provider)
	case "Model":
		return EvalNullable(r.Model)
	case "StartTime":
		return EvalNullable(r.StartTime)
	case "EndTime":
		return EvalNullable(r.EndTime)
	default:
		return nil, false
	}
}

type ProviderUsageSummaryGroupBy string

const (
	ProviderUsageSummaryGroupByHour       ProviderUsageSummaryGroupBy = "hour"
	ProviderUsageSummaryGroupByDay        ProviderUsageSummaryGroupBy = "day"
	ProviderUsageSummaryGroupByMonth      ProviderUsageSummaryGroupBy = "month"
	ProviderUsageSummaryGroupByProvider   ProviderUsageSummaryGroupBy = "provider"
	ProviderUsageSummaryGroupByModel      ProviderUsageSummaryGroupBy = "model"
	ProviderUsageSummaryGroupByTask       ProviderUsageSummaryGroupBy = "task"
	ProviderUsageSummaryGroupByWorkstream ProviderUsageSummaryGroupBy = "workstream"
)

type ProviderUsageSummary struct {
	Provider        *string    `json:"Provider,omitempty"`
	ProviderModelID *string    `json:"ProviderModelID,omitempty"`
	TaskID          *string    `json:"TaskID,omitempty"`
	WorkstreamID    *string    `json:"WorkstreamID,omitempty"`
	BucketStartTime *time.Time `json:"BucketStartTime,omitempty"`
	// BucketGranularity is set when a time dimension (`hour`, `day`, or
	// `month`) is in GroupBy. It tells the caller which time grain
	// BucketStartTime represents — without it day vs. hour vs. month
	// buckets are indistinguishable from a bare timestamp.
	BucketGranularity             *string        `json:"BucketGranularity,omitempty"`
	PromptTokens                  int            `json:"PromptTokens"`
	CompletionTokens              int            `json:"CompletionTokens"`
	CachedReadInputTokens         int            `json:"CachedReadInputTokens"`
	CacheCreationInputTokensByTTL map[string]int `json:"CacheCreationInputTokensByTTL,omitempty"`
	ReasoningTokens               int            `json:"ReasoningTokens"`
	EventCount                    int            `json:"EventCount"`
}

type GetProviderUsageSummaryRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID     string
	GroupBy      []ProviderUsageSummaryGroupBy
	WorkstreamID *string
	TaskID       *string
	Provider     *string
	Model        *string
	StartTime    *time.Time
	EndTime      *time.Time
}

// nolint: goconst
func (r *GetProviderUsageSummaryRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "GroupBy":
		return r.GroupBy, true
	case "WorkstreamID":
		return EvalNullable(r.WorkstreamID)
	case "TaskID":
		return EvalNullable(r.TaskID)
	case "Provider":
		return EvalNullable(r.Provider)
	case "Model":
		return EvalNullable(r.Model)
	case "StartTime":
		return EvalNullable(r.StartTime)
	case "EndTime":
		return EvalNullable(r.EndTime)
	default:
		return nil, false
	}
}

type GetProviderUsageSummaryResponse struct {
	TenantID  string                        `json:"TenantID"`
	StartTime *time.Time                    `json:"StartTime,omitempty"`
	EndTime   *time.Time                    `json:"EndTime,omitempty"`
	GroupBy   []ProviderUsageSummaryGroupBy `json:"GroupBy"`
	Items     []ProviderUsageSummary        `json:"Items"`
}

// ListProviderUsageEvents lists raw provider usage event records for a tenant,
// optionally filtered by workstream, task, provider, model, and a
// [StartTime, EndTime) window, with pagination via MaxResults/Token.
func (c *Client) ListProviderUsageEvents(
	ctx context.Context,
	req *ListProviderUsageEventsRequest,
) (*List[*ProviderUsageEvent], error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "provider-usage-events")
	q := u.Query()
	if req.MaxResults != nil {
		q.Set("maxResults", strconv.Itoa(*req.MaxResults))
	}
	if req.Token != nil {
		q.Set("token", *req.Token)
	}
	if req.WorkstreamID != nil {
		q.Set("workstreamID", *req.WorkstreamID)
	}
	if req.TaskID != nil {
		q.Set("taskID", *req.TaskID)
	}
	if req.Provider != nil {
		q.Set("provider", *req.Provider)
	}
	if req.Model != nil {
		q.Set("model", *req.Model)
	}
	if req.StartTime != nil {
		q.Set("startTime", req.StartTime.UTC().Format(time.RFC3339Nano))
	}
	if req.EndTime != nil {
		q.Set("endTime", req.EndTime.UTC().Format(time.RFC3339Nano))
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

	var out List[*ProviderUsageEvent]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetProviderUsageSummary aggregates provider token usage for a tenant
// over a time window. GroupBy is encoded on the wire as a comma-separated
// list, in the order provided; the API service parses it the same way.
// StartTime is an inclusive lower bound; EndTime is exclusive, matching
// the documented bound semantics shared with the events list. Time fields
// are serialized as RFC3339Nano (UTC).
//
// The response carries one bucket per distinct group key. Provider and
// ProviderModelID are populated on every bucket when grouped OR filtered;
// TaskID and WorkstreamID only when grouped. BucketGranularity is set
// when a time dimension is in GroupBy and tells the caller whether
// BucketStartTime represents an hour, day, or month.
func (c *Client) GetProviderUsageSummary(
	ctx context.Context,
	req *GetProviderUsageSummaryRequest,
) (*GetProviderUsageSummaryResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "provider-usage-summary")
	q := u.Query()
	if len(req.GroupBy) > 0 {
		parts := make([]string, len(req.GroupBy))
		for i, dim := range req.GroupBy {
			parts[i] = string(dim)
		}
		q.Set("groupBy", strings.Join(parts, ","))
	}
	if req.WorkstreamID != nil {
		q.Set("workstreamID", *req.WorkstreamID)
	}
	if req.TaskID != nil {
		q.Set("taskID", *req.TaskID)
	}
	if req.Provider != nil {
		q.Set("provider", *req.Provider)
	}
	if req.Model != nil {
		q.Set("model", *req.Model)
	}
	if req.StartTime != nil {
		q.Set("startTime", req.StartTime.UTC().Format(time.RFC3339Nano))
	}
	if req.EndTime != nil {
		q.Set("endTime", req.EndTime.UTC().Format(time.RFC3339Nano))
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

	var out GetProviderUsageSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
