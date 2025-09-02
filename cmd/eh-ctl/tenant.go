package main

import (
	"context"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	"github.com/google/uuid"
)

type TenantOptions struct {
	CreateUser  CreateUserOptions     `cmd:"create-user"`
	CurrentUser GetCurrentUserOptions `cmd:"current-user"`
	Get         GetTenantOptions      `cmd:"get"`
}

type CreateUserOptions struct {
	FullName   string  `help:"The user's full name" name:"full-name" short:"n" required:""`
	FirstName  string  `help:"The user's first name" name:"first-name" short:"f" required:""`
	LastName   string  `help:"The user's last name" name:"last-name" short:"l" required:""`
	Email      string  `help:"The user's email address" short:"e" required:""`
	PictureURL *string `help:"The user's profile picture URL" short:"p" optional:""`
}

func (o *CreateUserOptions) Run(ctx context.Context, shared *SharedOptions) error {
	req := &eh.CreateTenantRequest{
		TenantID:   uuid.NewString(),
		Type:       eh.TenantTypeUser,
		FullName:   &o.FullName,
		Email:      &o.Email,
		FirstName:  &o.FirstName,
		LastName:   &o.LastName,
		PictureURL: o.PictureURL,
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
	req := &eh.GetCurrentUserRequest{}
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
	req := &eh.GetTenantRequest{
		TenantID: o.TenantID,
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	t, err := s.Client.GetTenant(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(t)
}
