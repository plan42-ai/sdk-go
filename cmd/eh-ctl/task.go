package main

import (
	"context"
	"fmt"
	"math"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	"github.com/google/uuid"
)

type TaskOptions struct {
	Create CreateTaskOptions `cmd:"" help:"Create a new task."`
	Update UpdateTaskOptions `cmd:"" help:"Update an existing task."`
	Delete DeleteTaskOptions `cmd:"" help:"Soft delete an existing task."`
	List   ListTasksOptions  `cmd:"" help:"List tasks for a tenant or workstream."`
	Get    GetTaskOptions    `cmd:"" help:"Get a task by ID."`
	Move   MoveTaskOptions   `cmd:"" help:"Move a task from one workstream to another."`
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
	TenantID     string `help:"The id of the tenant owning the task to create." short:"i" required:""`
	JSON         string `help:"The json file to load the task from" short:"j" default:"-"`
	WorkstreamID string `help:"The id of the workstream to create the task in." name:"workstream-id" short:"w" optional:""`
}

func (o *CreateTaskOptions) Run(ctx context.Context, s *SharedOptions) error {
	if o.WorkstreamID != "" {
		return o.runWorkstream(ctx, s)
	}
	return o.runNonWorkstream(ctx, s)
}

func (o *CreateTaskOptions) runNonWorkstream(ctx context.Context, s *SharedOptions) error {
	err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags)
	if err != nil {
		return err
	}
	var req eh.CreateTaskRequest
	err = readJsonFile(o.JSON, &req)
	if err != nil {
		return err
	}
	err = loadFeatureFlags(s, &req.FeatureFlags)
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

func (o *CreateTaskOptions) runWorkstream(ctx context.Context, s *SharedOptions) error {
	err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags)
	if err != nil {
		return err
	}
	var req eh.CreateWorkstreamTaskRequest
	err = readJsonFile(o.JSON, &req)
	if err != nil {
		return err
	}
	err = loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	req.TenantID = o.TenantID
	req.WorkstreamID = o.WorkstreamID
	req.TaskID = uuid.NewString()
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	task, err := s.Client.CreateWorkstreamTask(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(task)
}

type UpdateTaskOptions struct {
	TenantID     string `help:"The ID of the tenant that owns the task to update." short:"i" required:""`
	TaskID       string `help:"The ID of the task to update." name:"task-id" short:"t" required:""`
	JSON         string `help:"The json file containing the updates." short:"j" default:"-"`
	WorkstreamID string `help:"The id of the workstream containing the task to update." name:"workstream-id" short:"w" optional:""`
}

func (o *UpdateTaskOptions) Run(ctx context.Context, s *SharedOptions) error {
	if o.WorkstreamID != "" {
		return o.runWorkstream(ctx, s)
	}
	return o.runNonWorkstream(ctx, s)
}

// nolint: dupl
func (o *UpdateTaskOptions) runNonWorkstream(ctx context.Context, s *SharedOptions) error {
	err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags)
	if err != nil {
		return err
	}
	var req eh.UpdateTaskRequest
	err = readJsonFile(o.JSON, &req)
	if err != nil {
		return err
	}
	err = loadFeatureFlags(s, &req.FeatureFlags)
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

// nolint: dupl
func (o *UpdateTaskOptions) runWorkstream(ctx context.Context, s *SharedOptions) error {
	err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags)
	if err != nil {
		return err
	}
	var req eh.UpdateWorkstreamTaskRequest
	err = readJsonFile(o.JSON, &req)
	if err != nil {
		return err
	}
	err = loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	req.TenantID = o.TenantID
	req.WorkstreamID = o.WorkstreamID
	req.TaskID = o.TaskID

	getReq := &eh.GetWorkstreamTaskRequest{
		TenantID:       o.TenantID,
		WorkstreamID:   o.WorkstreamID,
		TaskID:         o.TaskID,
		IncludeDeleted: pointer(true),
	}
	getReq.FeatureFlags = req.FeatureFlags
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	task, err := s.Client.GetWorkstreamTask(ctx, getReq)
	if err != nil {
		return err
	}
	req.Version = task.Version
	req.FeatureFlags = getReq.FeatureFlags
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	updated, err := s.Client.UpdateWorkstreamTask(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(updated)
}

type DeleteTaskOptions struct {
	TenantID     string `help:"The ID of the tennant that owns the task to delete." short:"i" required:""`
	TaskID       string `help:"The ID of the task to delete." short:"t" required:""`
	WorkstreamID string `help:"The ID of the workstream containing the task to delete." name:"workstream-id" short:"w" optional:""`
}

func (o *DeleteTaskOptions) Run(ctx context.Context, s *SharedOptions) error {
	if o.WorkstreamID != "" {
		return o.runWorkstream(ctx, s)
	}
	return o.runNonWorkstream(ctx, s)
}

func (o *DeleteTaskOptions) runNonWorkstream(ctx context.Context, s *SharedOptions) error {
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

func (o *DeleteTaskOptions) runWorkstream(ctx context.Context, s *SharedOptions) error {
	getReq := &eh.GetWorkstreamTaskRequest{TenantID: o.TenantID, WorkstreamID: o.WorkstreamID, TaskID: o.TaskID}
	err := loadFeatureFlags(s, &getReq.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	task, err := s.Client.GetWorkstreamTask(ctx, getReq)
	if err != nil {
		return err
	}

	req := &eh.DeleteWorkstreamTaskRequest{TenantID: o.TenantID, WorkstreamID: o.WorkstreamID, TaskID: o.TaskID, Version: task.Version}
	req.FeatureFlags = getReq.FeatureFlags
	processDelegatedAuth(s, &req.DelegatedAuthInfo)
	return s.Client.DeleteWorkstreamTask(ctx, req)
}

type ListTasksOptions struct {
	TenantID       string `help:"The ID of the tenant to list tasks for." short:"i" required:""`
	IncludeDeleted bool   `help:"When set, includes deleted tasks in the results." short:"d" optional:""`
	WorkstreamID   string `help:"Optional. When set, list tasks in the specified workstream." name:"workstream-id" short:"w" optional:""`
	MaxElements    *int   `help:"Maximum number of elements to retrieve" short:"m" optional:""`
}

func (o *ListTasksOptions) Run(ctx context.Context, s *SharedOptions) error {
	if o.WorkstreamID != "" {
		return o.runWorkstreamTasks(ctx, s)
	}
	return o.run(ctx, s)
}

func (o *ListTasksOptions) run(ctx context.Context, s *SharedOptions) error {
	req := &eh.ListTasksRequest{
		TenantID:       o.TenantID,
		IncludeDeleted: pointer(o.IncludeDeleted),
		MaxResults:     pointer(10),
	}

	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	remaining := math.MaxInt
	if o.MaxElements != nil {
		remaining = *o.MaxElements
	}

	for remaining > 0 {
		if remaining < *req.MaxResults {
			req.MaxResults = pointer(remaining)
		}
		resp, err := s.Client.ListTasks(ctx, req)
		if err != nil {
			return err
		}
		for _, task := range resp.Tasks {
			remaining--
			err = printJSON(task)
			if err != nil {
				return err
			}
			if remaining == 0 {
				return nil
			}
		}
		if resp.NextToken == nil {
			return nil
		}
		req.Token = resp.NextToken
	}
	return nil
}

func (o *ListTasksOptions) runWorkstreamTasks(ctx context.Context, s *SharedOptions) error {
	req := &eh.ListWorkstreamTasksRequest{
		TenantID:       o.TenantID,
		WorkstreamID:   o.WorkstreamID,
		IncludeDeleted: pointer(o.IncludeDeleted),
		MaxResults:     pointer(10),
	}

	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}

	remaining := math.MaxInt
	if o.MaxElements != nil {
		remaining = *o.MaxElements
	}

	for remaining > 0 {
		if remaining < *req.MaxResults {
			req.MaxResults = pointer(remaining)
		}
		resp, err := s.Client.ListWorkstreamTasks(ctx, req)
		if err != nil {
			return err
		}
		for _, task := range resp.Items {
			remaining--
			err = printJSON(task)
			if err != nil {
				return err
			}
			if remaining == 0 {
				return nil
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
	WorkstreamID   string `help:"The ID of the workstream to get the task from" name:"workstream-id" short:"w" optional:""`
	IncludeDeleted bool   `help:"Include deleted tasks" short:"d" optional:""`
}

func (o *GetTaskOptions) Run(ctx context.Context, s *SharedOptions) error {
	if o.WorkstreamID != "" {
		return o.runWorkstream(ctx, s)
	}
	return o.runStandalone(ctx, s)
}

func (o *GetTaskOptions) runStandalone(ctx context.Context, s *SharedOptions) error {
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

func (o *GetTaskOptions) runWorkstream(ctx context.Context, s *SharedOptions) error {
	req := &eh.GetWorkstreamTaskRequest{
		TenantID:       o.TenantID,
		WorkstreamID:   o.WorkstreamID,
		TaskID:         o.TaskID,
		IncludeDeleted: pointer(o.IncludeDeleted),
	}
	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	task, err := s.Client.GetWorkstreamTask(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(task)
}
