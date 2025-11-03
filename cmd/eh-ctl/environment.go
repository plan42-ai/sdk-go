package main

import (
	"context"
	"strings"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	"github.com/google/uuid"
)

type EnvironmentOptions struct {
	Create CreateEnvironmentOptions `cmd:""`
	Get    GetEnvironmentOptions    `cmd:""`
	Update UpdateEnvironmentOptions `cmd:""`
	Delete DeleteEnvironmentOptions `cmd:""`
	List   ListEnvironmentsOptions  `cmd:""`
}

type CreateEnvironmentOptions struct {
	TenantID string `help:"The tenant ID to create the environment for" short:"i" required:""`
	JSON     string `help:"The JSON file to load the environment definition from" short:"j" default:"-"`
}

func (o *CreateEnvironmentOptions) Run(ctx context.Context, s *SharedOptions) error {
	if err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags); err != nil {
		return err
	}
	var req eh.CreateEnvironmentRequest
	if err := readJsonFile(o.JSON, &req); err != nil {
		return err
	}
	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	req.TenantID = o.TenantID
	req.EnvironmentID = uuid.NewString()
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	env, err := s.Client.CreateEnvironment(ctx, &req)
	if err != nil {
		return err
	}
	maskSecrets(env, s)
	return printJSON(env)
}

type GetEnvironmentOptions struct {
	TenantID       string `help:"The tennant ID that owns the environment being fetched." short:"i" required:""`
	EnvironmentID  string `help:"The environment ID to fetch" name:"environment-id" short:"e"`
	IncludeDeleted bool   `help:"Include deleted environments in the list." short:"d" optional:""`
}

func (o *GetEnvironmentOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.GetEnvironmentRequest{
		TenantID:       o.TenantID,
		EnvironmentID:  o.EnvironmentID,
		IncludeDeleted: pointer(o.IncludeDeleted),
	}
	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}

	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	env, err := s.Client.GetEnvironment(ctx, req)
	if err != nil {
		return err
	}
	maskSecrets(env, s)
	return printJSON(env)
}

type UpdateEnvironmentOptions struct {
	TenantID      string `help:"The tennant ID that owns the environment being fetched." short:"i" required:""`
	EnvironmentID string `help:"The ID of the environment to update" name:"environment-id" short:"e" required:""`
	JSON          string `help:"The json file containing the environment updates" short:"j" default:"-" required:""`
}

// nolint: dupl
func (o *UpdateEnvironmentOptions) Run(ctx context.Context, s *SharedOptions) error {
	if err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags); err != nil {
		return err
	}
	var req eh.UpdateEnvironmentRequest
	if err := readJsonFile(o.JSON, &req); err != nil {
		return err
	}
	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	req.TenantID = o.TenantID
	req.EnvironmentID = o.EnvironmentID

	getReq := &eh.GetEnvironmentRequest{TenantID: o.TenantID, EnvironmentID: o.EnvironmentID, IncludeDeleted: pointer(true)}
	getReq.FeatureFlags = req.FeatureFlags
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	env, err := s.Client.GetEnvironment(ctx, getReq)
	if err != nil {
		return err
	}
	req.Version = env.Version
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	updated, err := s.Client.UpdateEnvironment(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(updated)
}

type DeleteEnvironmentOptions struct {
	TenantID      string `help:"The tennant ID that owns the environment being fetched." short:"i"`
	EnvironmentID string `help:"The ID of the environment to update" name:"environment-id" short:"e"`
}

func (o *DeleteEnvironmentOptions) Run(ctx context.Context, s *SharedOptions) error {
	getReq := &eh.GetEnvironmentRequest{TenantID: o.TenantID, EnvironmentID: o.EnvironmentID}
	err := loadFeatureFlags(s, &getReq.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	env, err := s.Client.GetEnvironment(ctx, getReq)
	if err != nil {
		return err
	}

	req := &eh.DeleteEnvironmentRequest{TenantID: o.TenantID, EnvironmentID: o.EnvironmentID, Version: env.Version}
	req.FeatureFlags = getReq.FeatureFlags
	processDelegatedAuth(s, &req.DelegatedAuthInfo)
	return s.Client.DeleteEnvironment(ctx, req)
}

type ListEnvironmentsOptions struct {
	TenantID       string `help:"The tennant ID that owns the environments being listed." name:"tenantid" short:"i"`
	IncludeDeleted bool   `help:"Include deleted environments in the list." short:"d"`
}

func (o *ListEnvironmentsOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.ListEnvironmentsRequest{
		TenantID:       o.TenantID,
		IncludeDeleted: pointer(o.IncludeDeleted),
	}
	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	var token *string
	for {
		req.Token = token
		processDelegatedAuth(s, &req.DelegatedAuthInfo)

		resp, err := s.Client.ListEnvironments(ctx, req)
		if err != nil {
			return err
		}
		for _, env := range resp.Items {
			maskSecrets(&env, s)
			if err := printJSON(env); err != nil {
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

func maskSecrets(e *eh.Environment, s *SharedOptions) {
	if s.ShowSecrets {
		return
	}
	for i := range e.EnvVars {
		if e.EnvVars[i].IsSecret {
			e.EnvVars[i].Value = strings.Repeat("*", len(e.EnvVars[i].Value))
		}
	}
}
