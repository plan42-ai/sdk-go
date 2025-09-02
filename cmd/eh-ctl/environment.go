package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	"github.com/google/uuid"
)

type EnvironmentOptions struct {
	Create CreateEnvironmentOptions `cmd:"create"`
	Get    GetEnvironmentOptions    `cmd:"get"`
	Update UpdateEnvironmentOptions `cmd:"update"`
	Delete DeleteEnvironmentOptions `cmd:"delete"`
	List   ListEnvironmentsOptions  `cmd:"list"`
}

type CreateEnvironmentOptions struct {
	TenantID string `help:"The tenant ID to create the environment for" short:"i" required:""`
	JSON     string `help:"The JSON file to load the environment definition from" short:"j" default:"-"`
}

func (o *CreateEnvironmentOptions) Run(ctx context.Context, s *SharedOptions) error {
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

	var req eh.CreateEnvironmentRequest
	if err := json.NewDecoder(reader).Decode(&req); err != nil {
		return err
	}

	req.TenantID = o.TenantID
	req.EnvironmentID = uuid.NewString()
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	env, err := s.Client.CreateEnvironment(ctx, &req)
	if err != nil {
		return err
	}
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
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	env, err := s.Client.GetEnvironment(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(env)
}

type UpdateEnvironmentOptions struct {
	TenantID      string `help:"The tennant ID that owns the environment being fetched." short:"i" required:""`
	EnvironmentID string `help:"The ID of the environment to update" name:"environment-id" short:"e" required:""`
	JSON          string `help:"The json file containing the environment updates" short:"j" default:"-" required:""`
}

// nolint: dupl
func (o *UpdateEnvironmentOptions) Run(ctx context.Context, s *SharedOptions) error {
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

	var req eh.UpdateEnvironmentRequest
	if err := json.NewDecoder(reader).Decode(&req); err != nil {
		return err
	}

	req.TenantID = o.TenantID
	req.EnvironmentID = o.EnvironmentID

	getReq := &eh.GetEnvironmentRequest{TenantID: o.TenantID, EnvironmentID: o.EnvironmentID, IncludeDeleted: pointer(true)}
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
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	env, err := s.Client.GetEnvironment(ctx, getReq)
	if err != nil {
		return err
	}

	req := &eh.DeleteEnvironmentRequest{TenantID: o.TenantID, EnvironmentID: o.EnvironmentID, Version: env.Version}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)
	return s.Client.DeleteEnvironment(ctx, req)
}

type ListEnvironmentsOptions struct {
	TenantID       string `help:"The tennant ID that owns the environments being listed." name:"tenantid" short:"i"`
	IncludeDeleted bool   `help:"Include deleted environments in the list." short:"d"`
}

func (o *ListEnvironmentsOptions) Run(ctx context.Context, s *SharedOptions) error {
	var token *string
	for {
		req := &eh.ListEnvironmentsRequest{
			TenantID:       o.TenantID,
			Token:          token,
			IncludeDeleted: pointer(o.IncludeDeleted),
		}
		processDelegatedAuth(s, &req.DelegatedAuthInfo)

		resp, err := s.Client.ListEnvironments(ctx, req)
		if err != nil {
			return err
		}
		for _, env := range resp.Environments {
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
