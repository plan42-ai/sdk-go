package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	"github.com/google/uuid"
)

type TaskOptions struct {
	Create CreateTaskOptions `cmd:"create"`
	Update UpdateTaskOptions `cmd:"update"`
	Delete DeleteTaskOptions `cmd:"delete"`
	List   ListTasksOptions  `cmd:"list"`
	Get    GetTaskOptions    `cmd:"get"`
}

type CreateTaskOptions struct {
	TenantID     string  `help:"The id of the tenant owning the task to create." short:"i" required:""`
	WorkstreamID *string `help:"Optional id of the workstream to create the task in." name:"workstream-id" short:"w"`
	JSON         string  `help:"The json file to load the task from" short:"j" default:"-"`
}

func (o *CreateTaskOptions) Run(ctx context.Context, s *SharedOptions) error {
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

	req.TenantID = o.TenantID
	req.TaskID = uuid.NewString()
	req.WorkstreamID = o.WorkstreamID
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

	req.TenantID = o.TenantID
	req.TaskID = o.TaskID

	getReq := &eh.GetTaskRequest{TenantID: o.TenantID, TaskID: o.TaskID, IncludeDeleted: pointer(true)}
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	task, err := s.Client.GetTask(ctx, getReq)
	if err != nil {
		return err
	}
	req.Version = task.Version
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
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	task, err := s.Client.GetTask(ctx, getReq)
	if err != nil {
		return err
	}

	req := &eh.DeleteTaskRequest{TenantID: o.TenantID, TaskID: o.TaskID, Version: task.Version}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)
	return s.Client.DeleteTask(ctx, req)
}

type ListTasksOptions struct {
	TenantID       string  `help:"The ID of the tenant to list tasks for." short:"i" required:""`
	WorkstreamID   *string `help:"Optional workstream ID. When specified tasks in that workstream are returned." short:"w" optional:""`
	IncludeDeleted bool    `help:"When set, includes deleted tasks in the results." short:"d" optional:""`
}

func (o *ListTasksOptions) Run(ctx context.Context, s *SharedOptions) error {
	var token *string
	for {
		req := &eh.ListTasksRequest{
			TenantID:       o.TenantID,
			Token:          token,
			IncludeDeleted: pointer(o.IncludeDeleted),
		}
		if o.WorkstreamID != nil {
			req.WorkstreamID = o.WorkstreamID
		}
		processDelegatedAuth(s, &req.DelegatedAuthInfo)

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
		token = resp.NextToken
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
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	task, err := s.Client.GetTask(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(task)
}
