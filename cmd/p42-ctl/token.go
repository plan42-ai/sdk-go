package main

import (
	"context"

	"github.com/google/uuid"
	"github.com/plan42-ai/sdk-go/p42"
)

type UITokenOptions struct {
	Generate GenerateUITokenOptions `cmd:""`
}

type GenerateUITokenOptions struct {
	TenantID string `help:"The ID of the tenant to generate the Web UI token for" short:"i" required:""`
}

func (o *GenerateUITokenOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &p42.GenerateWebUITokenRequest{
		TenantID: o.TenantID,
		TokenID:  uuid.NewString(),
	}
	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	resp, err := s.Client.GenerateWebUIToken(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(resp)
}
