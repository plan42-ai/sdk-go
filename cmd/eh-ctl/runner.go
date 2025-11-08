package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	"github.com/google/uuid"
)

type RunnerOptions struct {
	Create        CreateRunnerOptions        `cmd:""`
	List          ListRunnerOptions          `cmd:""`
	Get           GetRunnerOptions           `cmd:""`
	Delete        DeleteRunnerOptions        `cmd:""`
	Update        UpdateRunnerOptions        `cmd:""`
	GenerateToken GenerateRunnerTokenOptions `cmd:""`
	ListTokens    ListRunnerTokensOptions    `cmd:""`
}

type CreateRunnerOptions struct {
	TenantID string `help:"The tenant ID to create the runner under." name:"tenant-id" short:"i" required:""`
	JSON     string `help:"The JSON file containing the runner definition. Use '-' to read from stdin." short:"j" default:"-"`
}

func (o *CreateRunnerOptions) Run(ctx context.Context, s *SharedOptions) error {
	err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags)
	if err != nil {
		return err
	}
	var req eh.CreateRunnerRequest
	err = readJsonFile(o.JSON, &req)
	if err != nil {
		return err
	}
	err = loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	req.TenantID = o.TenantID
	if req.RunnerID == "" {
		req.RunnerID = uuid.NewString()
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	runner, err := s.Client.CreateRunner(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(runner)
}

type ListRunnerOptions struct {
	TenantID       string `help:"The tenant ID to list runners for." name:"tenant-id" short:"i" required:""`
	IncludeDeleted bool   `help:"When set, includes deleted runners in the results." short:"d" optional:""`
}

func (o *ListRunnerOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.ListRunnersRequest{
		TenantID:       o.TenantID,
		IncludeDeleted: pointer(o.IncludeDeleted),
	}

	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	for {
		resp, err := s.Client.ListRunners(ctx, req)
		if err != nil {
			return err
		}

		for _, runner := range resp.Items {
			if err := printJSON(runner); err != nil {
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

type GetRunnerOptions struct {
	TenantID       string `help:"The tenant ID that owns the runner to fetch." name:"tenant-id" short:"i" required:""`
	RunnerID       string `help:"The runner ID to fetch." name:"runner-id" short:"r" required:""`
	IncludeDeleted bool   `help:"Set to return a deleted runner." name:"include-deleted" short:"d" optional:""`
}

func (o *GetRunnerOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.GetRunnerRequest{
		TenantID:       o.TenantID,
		RunnerID:       o.RunnerID,
		IncludeDeleted: pointer(o.IncludeDeleted),
	}

	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}

	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	runner, err := s.Client.GetRunner(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(runner)
}

// DeleteRunnerOptions contains the flags for the `runner delete` command.
type DeleteRunnerOptions struct {
	TenantID string `help:"The tenant ID to connect to." name:"tenant-id" short:"i" required:""`
	RunnerID string `help:"The runner ID to delete." name:"runner-id" short:"r" required:""`
}

// Run executes the `runner delete` command.
func (o *DeleteRunnerOptions) Run(ctx context.Context, s *SharedOptions) error {
	getReq := &eh.GetRunnerRequest{
		TenantID: o.TenantID,
		RunnerID: o.RunnerID,
	}

	err := loadFeatureFlags(s, &getReq.FeatureFlags)
	if err != nil {
		return err
	}

	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)

	runner, err := s.Client.GetRunner(ctx, getReq)
	if err != nil {
		return err
	}

	delReq := &eh.DeleteRunnerRequest{
		TenantID: o.TenantID,
		RunnerID: o.RunnerID,
		Version:  runner.Version,
	}
	delReq.FeatureFlags = getReq.FeatureFlags
	processDelegatedAuth(s, &delReq.DelegatedAuthInfo)

	return s.Client.DeleteRunner(ctx, delReq)
}

type UpdateRunnerOptions struct {
	TenantID string `help:"The tenant ID to connect to." name:"tenant-id" short:"i" required:""`
	RunnerID string `help:"The runner ID to fetch." name:"runner-id" short:"r" required:""`
	JSON     string `help:"The json file containing the runner update. Use '-' to read from stdin." short:"j" default:"-"`
}

func (o *UpdateRunnerOptions) Run(ctx context.Context, s *SharedOptions) error {
	err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags)
	if err != nil {
		return err
	}
	var req eh.UpdateRunnerRequest
	err = readJsonFile(o.JSON, &req)
	if err != nil {
		return err
	}
	err = loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	req.TenantID = o.TenantID
	req.RunnerID = o.RunnerID

	getReq := &eh.GetRunnerRequest{
		TenantID:       o.TenantID,
		RunnerID:       o.RunnerID,
		IncludeDeleted: pointer(true),
	}
	getReq.FeatureFlags = req.FeatureFlags
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	runner, err := s.Client.GetRunner(ctx, getReq)
	if err != nil {
		return err
	}
	req.Version = runner.Version
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	updated, err := s.Client.UpdateRunner(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(updated)
}

type GenerateRunnerTokenOptions struct {
	TenantID string `help:"The tenant ID that owns the runner." name:"tenant-id" short:"i" required:""`
	RunnerID string `help:"The runner ID to generate a token for." name:"runner-id" short:"r" required:""`
}

func (o *GenerateRunnerTokenOptions) Run(ctx context.Context, s *SharedOptions) error {
	if !s.ShowSecrets {
		return fmt.Errorf("you must specify `-s` when calling generate-token")
	}

	req := &eh.GenerateRunnerTokenRequest{
		TenantID: o.TenantID,
		RunnerID: o.RunnerID,
	}

	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	resp, err := s.Client.GenerateRunnerToken(ctx, req)
	if err != nil {
		return err
	}

	return printJSON(resp)
}

type ListRunnerTokensOptions struct {
	TenantID string `help:"The tenant ID to list tokens for." name:"tenant-id" short:"i" required:""`
	RunnerID string `help:"The runner ID to list tokens for." name:"runner-id" short:"r" required:""`
}

func (o *ListRunnerTokensOptions) Run(ctx context.Context, s *SharedOptions) error {
	if s.ShowSecrets {
		return errors.New("invalid `-s`: runner token values cannot be fetched after they are generated")
	}

	req := &eh.ListRunnerTokensRequest{
		TenantID: o.TenantID,
		RunnerID: o.RunnerID,
	}

	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return err
	}

	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	for {
		resp, err := s.Client.ListRunnerTokens(ctx, req)
		if err != nil {
			return err
		}

		for _, token := range resp.Items {
			if err := printJSON(token); err != nil {
				return err
			}
		}

		if resp.NextPageToken == nil {
			break
		}

		req.NextPageToken = resp.NextPageToken
	}

	return nil
}
