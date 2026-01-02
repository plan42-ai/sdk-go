package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/plan42-ai/sdk-go/p42"
)

type TenantOptions struct {
	CreateUser  CreateUserOptions     `cmd:"" help:"Create a new user tenant."`
	CurrentUser GetCurrentUserOptions `cmd:"" help:"Find a user given their auth provider token."`
	Get         GetTenantOptions      `cmd:"" help:"Get a tenant by ID."`
	List        ListTenantsOptions    `cmd:"" help:"List all tenants."`
	Update      UpdateTenantOptions   `cmd:"" help:"Update a tenant by ID."`
}

type CreateUserOptions struct {
	FullName   string  `help:"The user's full name" name:"full-name" short:"n" required:""`
	FirstName  string  `help:"The user's first name" name:"first-name" short:"f" required:""`
	LastName   string  `help:"The user's last name" name:"last-name" short:"l" required:""`
	Email      string  `help:"The user's email address" short:"e" required:""`
	PictureURL *string `help:"The user's profile picture URL" short:"p" optional:""`
}

func (o *CreateUserOptions) Run(ctx context.Context, shared *SharedOptions) error {
	req := &p42.CreateTenantRequest{
		TenantID:   uuid.NewString(),
		Type:       p42.TenantTypeUser,
		FullName:   &o.FullName,
		Email:      &o.Email,
		FirstName:  &o.FirstName,
		LastName:   &o.LastName,
		PictureURL: o.PictureURL,
	}
	err := loadFeatureFlags(shared, &req.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(shared, &req.DelegatedAuthInfo)

	t, err := shared.Client.CreateTenant(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(t)
}

type GetCurrentUserOptions struct{}

func (o *GetCurrentUserOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &p42.GetCurrentUserRequest{}
	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	t, err := s.Client.GetCurrentUser(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(t)
}

type GetTenantOptions struct {
	TenantID string `help:"The ID of the tenant to fetch" short:"i" required:""`
}

func (o *GetTenantOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &p42.GetTenantRequest{
		TenantID: o.TenantID,
	}
	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	t, err := s.Client.GetTenant(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(t)
}

type ListTenantsOptions struct{}

func (o *ListTenantsOptions) Run(ctx context.Context, s *SharedOptions) error {
	if s.DelegatedAuthType != nil || s.DelegatedToken != nil {
		return fmt.Errorf(delegatedAuthNotSupported, "tenant list")
	}
	if err := ensureNoFeatureFlags(s, "tenant list"); err != nil {
		return err
	}

	var token *string
	for {
		req := &p42.ListTenantsRequest{Token: token}

		resp, err := s.Client.ListTenants(ctx, req)
		if err != nil {
			return err
		}
		for _, tenant := range resp.Items {
			if err := printJSON(tenant); err != nil {
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

type UpdateTenantOptions struct {
	TenantID string `help:"The ID of the tenant to update." name:"tenant-id" short:"i" required:""`
	JSON     string `help:"The JSON file containing the tenant updates. Use '-' to read from stdin." name:"json" short:"j" default:"-"`
}

func (o *UpdateTenantOptions) Run(ctx context.Context, s *SharedOptions) error {
	if err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags); err != nil {
		return err
	}
	var req p42.UpdateTenantRequest
	if err := readJsonFile(o.JSON, &req); err != nil {
		return err
	}
	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return err
	}
	req.TenantID = o.TenantID
	getReq := &p42.GetTenantRequest{TenantID: o.TenantID}
	getReq.FeatureFlags = req.FeatureFlags
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)

	tenant, err := s.Client.GetTenant(ctx, getReq)
	if err != nil {
		return err
	}
	req.Version = tenant.Version
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	updated, err := s.Client.UpdateTenant(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(updated)
}
