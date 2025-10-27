package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	"github.com/google/uuid"
)

type TaskOptions struct {
	Create CreateTaskOptions `cmd:""`
	Update UpdateTaskOptions `cmd:""`
	Delete DeleteTaskOptions `cmd:""`
	List   ListTasksOptions  `cmd:""`
	Get    GetTaskOptions    `cmd:""`
	Move   MoveTaskOptions   `cmd:""`
}

// MoveTaskOptions contains the flags for the `task move` command.
type MoveTaskOptions struct {
	TenantID     string `help:"The id of the tenant to move the task in." name:"tenant-id" short:"i" required:""`
	TaskID       string `help:"The id of the task to move." name:"task-id" short:"t" required:""`
	WorkstreamID string `help:"The id of the workstream to move the task to." name:"workstream-id" short:"w" required:""`
}

func (o *MoveTaskOptions) Run(ctx context.Context, s *SharedOptions) error {
	// Retrieve the task to obtain its current version and source workstream.
	getTaskReq := &eh.GetTaskRequest{
		TenantID: o.TenantID,
		TaskID:   o.TaskID,
	}

	if err := loadFeatureFlags(s, &getTaskReq.FeatureFlags); err != nil {
		return err
	}
	processDelegatedAuth(s, &getTaskReq.DelegatedAuthInfo)

	task, err := s.Client.GetTask(ctx, getTaskReq)
	if err != nil {
		return err
	}

	if task.WorkstreamID == nil {
		return fmt.Errorf("task %s is not associated with a workstream", o.TaskID)
	}

	sourceWSID := *task.WorkstreamID

	// Fetch versions for source and destination workstreams.
	getSourceWSReq := &eh.GetWorkstreamRequest{
		TenantID:     o.TenantID,
		WorkstreamID: sourceWSID,
	}
	getSourceWSReq.FeatureFlags = getTaskReq.FeatureFlags
	processDelegatedAuth(s, &getSourceWSReq.DelegatedAuthInfo)

	sourceWS, err := s.Client.GetWorkstream(ctx, getSourceWSReq)
	if err != nil {
		return err
	}

	getDestWSReq := &eh.GetWorkstreamRequest{
		TenantID:     o.TenantID,
		WorkstreamID: o.WorkstreamID,
	}
	getDestWSReq.FeatureFlags = getTaskReq.FeatureFlags
	processDelegatedAuth(s, &getDestWSReq.DelegatedAuthInfo)

	destWS, err := s.Client.GetWorkstream(ctx, getDestWSReq)
	if err != nil {
		return err
	}

	moveReq := &eh.MoveTaskRequest{
		TenantID:                     o.TenantID,
		TaskID:                       o.TaskID,
		DestinationWorkstreamID:      o.WorkstreamID,
		TaskVersion:                  task.Version,
		SourceWorkstreamVersion:      sourceWS.Version,
		DestinationWorkstreamVersion: destWS.Version,
	}
	moveReq.FeatureFlags = getTaskReq.FeatureFlags
	processDelegatedAuth(s, &moveReq.DelegatedAuthInfo)

	resp, err := s.Client.MoveTask(ctx, moveReq)
	if err != nil {
		return err
	}

	return printJSON(resp)
}

type CreateTaskOptions struct {
	TenantID string `help:"The id of the tenant owning the task to create." short:"i" required:""`
	JSON     string `help:"The json file to load the task from" short:"j" default:"-"`
}

func (o *CreateTaskOptions) Run(ctx context.Context, s *SharedOptions) error {
	if err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags); err != nil {
		return err
	}
	var reader *os.File
	if o.JSON == "-" {
		reader = os.Stdin
	} else {
		f, err := os.Open(o.JSON)
		if err != nil {
			return err
		}
		defer f.Close()
		reader = f
	}

	var req eh.CreateTaskRequest
	if err := json.NewDecoder(reader).Decode(&req); err != nil {
		return err
	}
	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	req.TenantID = o.TenantID
	req.TaskID = uuid.NewString()
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	task, err := s.Client.CreateTask(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(task)
}

type UpdateTaskOptions struct {
	TenantID string `help:"The ID of the tenant that owns the task to update." short:"i" required:""`
	TaskID   string `help:"The ID of the task to update." name:"task-id" short:"t" required:""`
	JSON     string `help:"The json file containing the updates." short:"j" default:"-"`
}

// nolint: dupl
func (o *UpdateTaskOptions) Run(ctx context.Context, s *SharedOptions) error {
	if err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags); err != nil {
		return err
	}
	var reader *os.File
	if o.JSON == "-" {
		reader = os.Stdin
	} else {
		f, err := os.Open(o.JSON)
		if err != nil {
			return err
		}
		defer f.Close()
		reader = f
	}

	var req eh.UpdateTaskRequest
	if err := json.NewDecoder(reader).Decode(&req); err != nil {
		return err
	}
	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	req.TenantID = o.TenantID
	req.TaskID = o.TaskID

	getReq := &eh.GetTaskRequest{TenantID: o.TenantID, TaskID: o.TaskID, IncludeDeleted: pointer(true)}
	getReq.FeatureFlags = req.FeatureFlags
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	task, err := s.Client.GetTask(ctx, getReq)
	if err != nil {
		return err
	}
	req.Version = task.Version
	req.FeatureFlags = getReq.FeatureFlags
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	updated, err := s.Client.UpdateTask(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(updated)
}

type DeleteTaskOptions struct {
	TenantID string `help:"The ID of the tennant that owns the task to delete." short:"i" required:""`
	TaskID   string `help:"The ID of the task to delete." short:"t" required:""`
}

func (o *DeleteTaskOptions) Run(ctx context.Context, s *SharedOptions) error {
	getReq := &eh.GetTaskRequest{TenantID: o.TenantID, TaskID: o.TaskID}
	err := loadFeatureFlags(s, &getReq.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	task, err := s.Client.GetTask(ctx, getReq)
	if err != nil {
		return err
	}

	req := &eh.DeleteTaskRequest{TenantID: o.TenantID, TaskID: o.TaskID, Version: task.Version}
	req.FeatureFlags = getReq.FeatureFlags
	processDelegatedAuth(s, &req.DelegatedAuthInfo)
	return s.Client.DeleteTask(ctx, req)
}

type ListTasksOptions struct {
	TenantID       string `help:"The ID of the tenant to list tasks for." short:"i" required:""`
	IncludeDeleted bool   `help:"When set, includes deleted tasks in the results." short:"d" optional:""`
}

func (o *ListTasksOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.ListTasksRequest{
		TenantID:       o.TenantID,
		IncludeDeleted: pointer(o.IncludeDeleted),
	}

	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}

	for {
		resp, err := s.Client.ListTasks(ctx, req)
		if err != nil {
			return err
		}
		for _, task := range resp.Tasks {
			if err := printJSON(task); err != nil {
				return err
			}
		}
		if resp.NextToken == nil {
			break
		}
		req.Token = resp.NextToken
	}
	return nil
}

type GetTaskOptions struct {
	TenantID       string `help:"The ID of the tenant to list tasks for" short:"i" required:""`
	TaskID         string `help:"The ID of the task to get" short:"t" required:""`
	IncludeDeleted bool   `help:"Include deleted tasks" short:"d" optional:""`
}

func (o *GetTaskOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.GetTaskRequest{
		TenantID:       o.TenantID,
		TaskID:         o.TaskID,
		IncludeDeleted: pointer(o.IncludeDeleted),
	}
	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	task, err := s.Client.GetTask(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(task)
}
