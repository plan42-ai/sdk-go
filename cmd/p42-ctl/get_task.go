package main

import (
	"context"

	"github.com/plan42-ai/sdk-go/internal/util"
	"github.com/plan42-ai/sdk-go/p42"
)

// GetTaskCmdOptions contains the flags for the top-level `get-task` command.
type GetTaskCmdOptions struct {
	TenantID       string `help:"The ID of the tenant that owns the task." name:"tenant-id" required:""`
	TaskID         string `help:"The ID of the task to retrieve." name:"task-id" required:""`
	IncludeDeleted bool   `help:"Include deleted tasks in the lookup." name:"include-deleted" optional:""`
}

func (o *GetTaskCmdOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &p42.GetTaskRequest{
		TenantID:       o.TenantID,
		TaskID:         o.TaskID,
		IncludeDeleted: util.Pointer(o.IncludeDeleted),
	}
	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	task, err := s.Client.GetTask(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(s.Stdout, task)
}
