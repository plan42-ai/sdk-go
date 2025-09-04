package main

import (
	"context"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
)

type PolicyOptions struct {
	List ListPoliciesOptions `cmd:""`
}

type ListPoliciesOptions struct {
	TenantID string `help:"The ID of the tenant to list policies for" short:"i" required:""`
}

func (o *ListPoliciesOptions) Run(ctx context.Context, s *SharedOptions) error {
	var token *string
	for {
		req := &eh.ListPoliciesRequest{
			TenantID: o.TenantID,
			Token:    token,
		}
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
