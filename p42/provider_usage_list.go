package p42

import "time"

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
	TotalTokens                   int            `json:"TotalTokens"`
	CachedReadInputTokens         int            `json:"CachedReadInputTokens"`
	CacheCreationInputTokensByTTL map[string]int `json:"CacheCreationInputTokensByTTL,omitempty"`
	ReasoningTokens               int            `json:"ReasoningTokens"`
	IngestionInputTokens          int            `json:"IngestionInputTokens"`
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
