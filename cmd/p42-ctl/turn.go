package main

import (
	"context"
	"fmt"
	"time"

	"github.com/plan42-ai/clock"
	"github.com/plan42-ai/sdk-go/internal/util"
	"github.com/plan42-ai/sdk-go/p42"
)

type TurnOptions struct {
	Create     CreateTurnOptions      `cmd:"" help:"Create a new turn for a task."`
	List       ListTurnsOptions       `cmd:"" help:"List turns for a given task."`
	Update     UpdateTurnOptions      `cmd:"" help:"Update an existing turn."`
	Get        GetTurnOptions         `cmd:"" help:"Get a specific turn by index."`
	GetLast    GetLastTurnOptions     `cmd:"" help:"Get the last turn for a given task."`
	ListActive ListActiveTurnsOptions `cmd:"" help:"List active turns."`
}

type CreateTurnOptions struct {
	TenantID     string  `help:"The ID of the tenant owning the task we are adding a turn to." short:"i" required:""`
	TaskID       string  `help:"The task to add a turn to." short:"t" required:""`
	JSON         string  `help:"The json file containing the turn definition." short:"j" default:"-"`
	WorkstreamID *string `help:"Optional. The workstream ID the task belongs to." name:"workstream-id" short:"w" optional:""`
}

func (o *CreateTurnOptions) Run(ctx context.Context, s *SharedOptions) error {
	if err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags); err != nil {
		return err
	}
	var req p42.CreateTurnRequest
	err := readJsonFile(o.JSON, &req)
	if err != nil {
		return err
	}
	err = loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	req.WorkstreamID = o.WorkstreamID

	var task *p42.Task
	if o.WorkstreamID != nil {
		task, err = o.getWorkstreamTask(ctx, s, req.FeatureFlags)
	} else {
		task, err = o.getTask(ctx, s, req.FeatureFlags)
	}
	if err != nil {
		return err
	}

	getReq := &p42.GetLastTurnRequest{TenantID: o.TenantID, TaskID: o.TaskID, WorkstreamID: o.WorkstreamID}
	getReq.FeatureFlags = req.FeatureFlags
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	last, err := s.Client.GetLastTurn(ctx, getReq)
	if err != nil {
		if !is404(err) || o.WorkstreamID == nil {
			return err
		}
	}

	req.TenantID = o.TenantID
	req.TaskID = o.TaskID
	if last == nil {
		req.TurnIndex = 1
	} else {
		req.TurnIndex = last.TurnIndex + 1
	}
	req.TaskVersion = task.Version
	req.FeatureFlags = getReq.FeatureFlags
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	turn, err := s.Client.CreateTurn(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(s.Stdout, turn)
}

func (o *CreateTurnOptions) getTask(ctx context.Context, s *SharedOptions, featureFlags p42.FeatureFlags) (*p42.Task, error) {
	getTaskReq := &p42.GetTaskRequest{TenantID: o.TenantID, TaskID: o.TaskID}
	getTaskReq.FeatureFlags = featureFlags
	processDelegatedAuth(s, &getTaskReq.DelegatedAuthInfo)

	return s.Client.GetTask(ctx, getTaskReq)
}

func (o *CreateTurnOptions) getWorkstreamTask(ctx context.Context, s *SharedOptions, featureFlags p42.FeatureFlags) (*p42.Task, error) {
	getTaskReq := &p42.GetWorkstreamTaskRequest{TenantID: o.TenantID, WorkstreamID: *o.WorkstreamID, TaskID: o.TaskID}
	getTaskReq.FeatureFlags = featureFlags
	processDelegatedAuth(s, &getTaskReq.DelegatedAuthInfo)

	return s.Client.GetWorkstreamTask(ctx, getTaskReq)
}

type ListTurnsOptions struct {
	TenantID       string  `help:"The ID of the tennant that owns the task we want to list turns for." short:"i" required:""`
	TaskID         string  `help:"The ID of the task we want to list turns for." short:"t" required:""`
	WorkstreamID   *string `help:"The ID of the workstream the task belongs to." short:"w" optional:""`
	IncludeDeleted bool    `help:"Set to return turns even if they are define on a deleted task." short:"d"`
}

func (o *ListTurnsOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &p42.ListTurnsRequest{
		TenantID:       o.TenantID,
		TaskID:         o.TaskID,
		WorkstreamID:   o.WorkstreamID,
		IncludeDeleted: util.Pointer(o.IncludeDeleted),
	}
	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	var token *string
	for {
		req.Token = token
		processDelegatedAuth(s, &req.DelegatedAuthInfo)

		resp, err := s.Client.ListTurns(ctx, req)
		if err != nil {
			return err
		}
		for _, tr := range resp.Items {
			if err := printJSON(s.Stdout, tr); err != nil {
				return err
			}
		}
		if resp.NextToken == nil {
			break
		}
		token = resp.NextToken
	}
	return nil
}

type UpdateTurnOptions struct {
	TenantID     string  `help:"The ID of the tennant that owns the task and turn" short:"i" required:""`
	TaskID       string  `help:"The ID of the task that contains the turn" short:"t" required:""`
	TurnIndex    int     `help:"The index of the turn to update" name:"turn-index" short:"n" required:""`
	JSON         string  `help:"The json file containing the updates to make" short:"j" default:"-"`
	WorkstreamID *string `help:"Optional. The id of the workstream the task belongs to." name:"workstream-id" short:"w" optional:""`
}

func (o *UpdateTurnOptions) Run(ctx context.Context, s *SharedOptions) error {
	if err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags); err != nil {
		return err
	}
	var req p42.UpdateTurnRequest
	err := readJsonFile(o.JSON, &req)
	if err != nil {
		return err
	}
	err = loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	req.WorkstreamID = o.WorkstreamID
	req.TenantID = o.TenantID
	req.TaskID = o.TaskID
	req.TurnIndex = o.TurnIndex
	getReq := &p42.GetTurnRequest{
		TenantID:       o.TenantID,
		TaskID:         o.TaskID,
		TurnIndex:      o.TurnIndex,
		IncludeDeleted: util.Pointer(true),
	}
	getReq.WorkstreamID = req.WorkstreamID
	getReq.FeatureFlags = req.FeatureFlags
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	turn, err := s.Client.GetTurn(ctx, getReq)
	if err != nil {
		return err
	}
	req.Version = turn.Version
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	updated, err := s.Client.UpdateTurn(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(s.Stdout, updated)
}

type GetTurnOptions struct {
	TenantID       string  `help:"The ID of the tennant that owns the task" name:"tenant-id" short:"i" required:""`
	TaskID         string  `help:"The ID of the task containing the turn" name:"task-id" short:"t" required:""`
	TurnIndex      int     `help:"The index of the turn to fetch" name:"turn-index" short:"n" required:""`
	IncludeDeleted bool    `help:"Include the turn even if defined on a deleted task" short:"d"`
	WorkstreamID   *string `help:"Optional. The id of the workstream the task belongs to." name:"workstream-id" short:"w" optional:""`
}

func (o *GetTurnOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &p42.GetTurnRequest{
		TenantID:       o.TenantID,
		TaskID:         o.TaskID,
		TurnIndex:      o.TurnIndex,
		IncludeDeleted: util.Pointer(o.IncludeDeleted),
		WorkstreamID:   o.WorkstreamID,
	}
	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	turn, err := s.Client.GetTurn(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(s.Stdout, turn)
}

type GetLastTurnOptions struct {
	TenantID       string  `help:"The ID of the tennant that owns the task" name:"tenant-id" short:"i" required:""`
	TaskID         string  `help:"The ID of the task to fetch the last turn for" name:"task-id" short:"t" required:""`
	WorkstreamID   *string `help:"The ID of the workstream containing the task." name:"workstream-id" short:"w" optional:""`
	IncludeDeleted bool    `help:"Set to return a turn on a deleted task" short:"d"`
}

func (o *GetLastTurnOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &p42.GetLastTurnRequest{
		TenantID:       o.TenantID,
		TaskID:         o.TaskID,
		WorkstreamID:   o.WorkstreamID,
		IncludeDeleted: util.Pointer(o.IncludeDeleted),
	}
	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	turn, err := s.Client.GetLastTurn(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(s.Stdout, turn)
}

type ListActiveTurnsOptions struct {
	Partition int     `help:"The partition number to fetch active turns from." short:"p" required:""`
	From      *string `help:"The inclusive minimum last updated time to fetch." short:"f" optional:""`
	To        string  `help:"The exclusive maximum last updated time to fetch." short:"t" default:"-5m"`
}

func (o *ListActiveTurnsOptions) Run(ctx context.Context, s *SharedOptions) error {
	now := time.Now()
	toValue := o.To
	if toValue == "" {
		toValue = "-5m"
	}
	if o.Partition < 0 {
		return fmt.Errorf("partition must be non-negative")
	}

	var minUpdatedAt *time.Time
	if o.From != nil {
		minTime, err := clock.ParseTimeOrRelativeDuration(*o.From, now)
		if err != nil {
			return err
		}
		minUpdatedAt = &minTime
	}

	maxUpdatedAt, err := clock.ParseTimeOrRelativeDuration(toValue, now)
	if err != nil {
		return err
	}

	req := &p42.ListActiveTurnsRequest{
		Partition:    o.Partition,
		MinUpdatedAt: minUpdatedAt,
		MaxUpdatedAt: maxUpdatedAt,
	}

	err = loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}

	var token *string
	for {
		req.Token = token
		processDelegatedAuth(s, &req.DelegatedAuthInfo)

		resp, err := s.Client.ListActiveTurns(ctx, req)
		if err != nil {
			return err
		}

		for _, turn := range resp.Items {
			err = printJSON(s.Stdout, turn)
			if err != nil {
				return err
			}
		}

		if resp.NextToken == nil {
			break
		}
		token = resp.NextToken
	}

	return nil
}
