package main

import (
	"context"

	"github.com/plan42-ai/sdk-go/p42"
)

type PolicyOptions struct {
	List ListPoliciesOptions `cmd:"" help:"List policies for a tenant."`
}

type ListPoliciesOptions struct {
	TenantID string `help:"The ID of the tenant to list policies for" short:"i" required:""`
}

func (o *ListPoliciesOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &p42.ListPoliciesRequest{
		TenantID: o.TenantID,
	}
	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	var token *string
	for {
		req.Token = token
		processDelegatedAuth(s, &req.DelegatedAuthInfo)

		resp, err := s.Client.ListPolicies(ctx, req)
		if err != nil {
			return err
		}
		for _, pol := range resp.Policies {
			if err := printJSON(pol); err != nil {
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
