package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/plan42-ai/sdk-go/internal/util"
	"github.com/plan42-ai/sdk-go/p42"
)

// UpdateTaskCmdOptions contains the flags for the top-level `update-task` command.
type UpdateTaskCmdOptions struct {
	TenantID       string   `help:"The ID of the tenant that owns the task." name:"tenant-id" required:""`
	TaskID         string   `help:"The ID of the task to update." name:"task-id" required:""`
	Version        int      `help:"The current version of the task (optimistic concurrency)." name:"version" required:""`
	Title          *string  `help:"New title for the task." name:"title" optional:""`
	Prompt         *string  `help:"New prompt for the task." name:"prompt" optional:""`
	Model          *string  `help:"Model to use (e.g. 'GPT-5.1 Codex')." name:"model" optional:""`
	ReasoningLevel *string  `help:"Reasoning level: Low, Medium, or High." name:"reasoning-level" optional:""`
	RepoInfoJSON   *string  `help:"RepoInfo as a JSON string, e.g. '{\"org/repo\":{\"FeatureBranch\":\"fb\",\"TargetBranch\":\"main\"}}'." name:"repo-info" optional:""`
	Deleted        *bool    `help:"Set to false to restore an archived task." name:"deleted" optional:"" negatable:""`
	NewFileIDs     []string `help:"File IDs to attach to the task. Repeatable." name:"new-file-id" optional:""`
}

func (o *UpdateTaskCmdOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &p42.UpdateTaskRequest{
		TenantID: o.TenantID,
		TaskID:   o.TaskID,
		Version:  o.Version,
		Title:    o.Title,
		Prompt:   o.Prompt,
	}

	if o.Model != nil {
		m := p42.ModelType(*o.Model)
		req.Model = &m
	}

	if o.ReasoningLevel != nil {
		rl := p42.ReasoningLevel(*o.ReasoningLevel)
		req.ReasoningLevel = &rl
	}

	if o.RepoInfoJSON != nil {
		var ri map[string]*p42.RepoInfo
		if err := json.Unmarshal([]byte(*o.RepoInfoJSON), &ri); err != nil {
			return fmt.Errorf("invalid --repo-info JSON: %w", err)
		}
		req.RepoInfo = &ri
	}

	if o.Deleted != nil {
		req.Deleted = o.Deleted
	}

	if len(o.NewFileIDs) > 0 {
		req.NewFileIDs = util.Pointer(o.NewFileIDs)
	}

	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	task, err := s.Client.UpdateTask(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(s.Stdout, task)
}
