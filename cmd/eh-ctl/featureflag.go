package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
)

type FeatureFlagOptions struct {
	Add    AddFeatureFlagOptions    `cmd:""`
	List   ListFeatureFlagsOptions  `cmd:""`
	Get    GetFeatureFlagOptions    `cmd:""`
	Delete DeleteFeatureFlagOptions `cmd:""`
	Update UpdateFeatureFlagOptions `cmd:""`
}

type AddFeatureFlagOptions struct {
	FlagName    string  `help:"The name of the flag to add." name:"flag-name" short:"f" required:""`
	Percentage  float64 `help:"The % of customers getting the new flag by default." name:"percentage" short:"p" default:"0.0"`
	Description string  `help:"The description of the flag to add. Optional." name:"description" short:"-D" optional:""`
}

func (o *AddFeatureFlagOptions) Run(ctx context.Context, s *SharedOptions) error {
	if o.Percentage < 0 || o.Percentage > 1 {
		return fmt.Errorf("percentage must be between 0.0 and 1.0")
	}

	if s.DelegatedAuthType != nil || s.DelegatedToken != nil {
		return fmt.Errorf(delegatedAuthNotSupported, "feature-flag add")
	}

	req := &eh.CreateFeatureFlagRequest{
		FlagName:    o.FlagName,
		DefaultPct:  o.Percentage,
		Description: o.Description,
	}

	flag, err := s.Client.CreateFeatureFlag(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(flag)
}

type ListFeatureFlagsOptions struct {
	IncludeDeleted bool `help:"When set, include deleted feature flags in the results." short:"d" optional:""`
}

func (o *ListFeatureFlagsOptions) Run(ctx context.Context, s *SharedOptions) error {
	if s.DelegatedAuthType != nil || s.DelegatedToken != nil {
		return fmt.Errorf(delegatedAuthNotSupported, "feature-flag list")
	}

	var token *string
	for {
		req := &eh.ListFeatureFlagsRequest{
			Token:          token,
			IncludeDeleted: pointer(o.IncludeDeleted),
		}

		resp, err := s.Client.ListFeatureFlags(ctx, req)
		if err != nil {
			return err
		}

		for _, flag := range resp.FeatureFlags {
			if err := printJSON(flag); err != nil {
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

type GetFeatureFlagOptions struct {
	FlagName       string `help:"The name of the flag to get." name:"flag-name" short:"f" required:""`
	IncludeDeleted bool   `help:"Include deleted flags." short:"d" optional:""`
}

func (o *GetFeatureFlagOptions) Run(ctx context.Context, s *SharedOptions) error {
	if s.DelegatedAuthType != nil || s.DelegatedToken != nil {
		return fmt.Errorf(delegatedAuthNotSupported, "feature-flag get")
	}

	req := &eh.GetFeatureFlagRequest{
		FlagName:       o.FlagName,
		IncludeDeleted: pointer(o.IncludeDeleted),
	}

	flag, err := s.Client.GetFeatureFlag(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(flag)
}

type DeleteFeatureFlagOptions struct {
	FlagName string `help:"The name of the flag to delete." name:"flag-name" short:"f" required:""`
}

func (o *DeleteFeatureFlagOptions) Run(ctx context.Context, s *SharedOptions) error {
	if s.DelegatedAuthType != nil || s.DelegatedToken != nil {
		return fmt.Errorf(delegatedAuthNotSupported, "feature-flag delete")
	}

	getReq := &eh.GetFeatureFlagRequest{FlagName: o.FlagName}
	flag, err := s.Client.GetFeatureFlag(ctx, getReq)
	if err != nil {
		return err
	}

	req := &eh.DeleteFeatureFlagRequest{FlagName: o.FlagName, Version: flag.Version}
	return s.Client.DeleteFeatureFlag(ctx, req)
}

type UpdateFeatureFlagOptions struct {
	FlagName string `help:"The name of the flag to update." name:"flag-name" short:"f" required:""`
	JSON     string `help:"The json file containing the updates to apply." short:"j" default:"-"`
}

// nolint: dupl
func (o *UpdateFeatureFlagOptions) Run(ctx context.Context, s *SharedOptions) error {
	if s.DelegatedAuthType != nil || s.DelegatedToken != nil {
		return fmt.Errorf(delegatedAuthNotSupported, "feature-flag update")
	}

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

	var req eh.UpdateFeatureFlagRequest
	if err := json.NewDecoder(reader).Decode(&req); err != nil {
		return err
	}

	req.FlagName = o.FlagName

	getReq := &eh.GetFeatureFlagRequest{FlagName: o.FlagName, IncludeDeleted: pointer(true)}
	flag, err := s.Client.GetFeatureFlag(ctx, getReq)
	if err != nil {
		return err
	}
	req.Version = flag.Version

	updated, err := s.Client.UpdateFeatureFlag(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(updated)
}
