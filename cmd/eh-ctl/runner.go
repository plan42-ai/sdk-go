package main

import (
	"context"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	"github.com/google/uuid"
)

type RunnerOptions struct {
	Create CreateRunnerOptions `cmd:""`
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
