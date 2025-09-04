package main

import (
	"context"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	"github.com/google/uuid"
)

type UITokenOptions struct {
	Generate GenerateUITokenOptions `cmd:""`
}

type GenerateUITokenOptions struct {
	TenantID string `help:"The ID of the tenant to generate the Web UI token for" short:"i" required:""`
}

func (o *GenerateUITokenOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.GenerateWebUITokenRequest{
		TenantID: o.TenantID,
		TokenID:  uuid.NewString(),
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	resp, err := s.Client.GenerateWebUIToken(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(resp)
}
