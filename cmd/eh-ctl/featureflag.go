package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
)

type FeatureFlagOptions struct {
	Add            AddFeatureFlagOptions            `cmd:""`
	List           ListFeatureFlagsOptions          `cmd:""`
	Get            GetFeatureFlagOptions            `cmd:""`
	Delete         DeleteFeatureFlagOptions         `cmd:""`
	Update         UpdateFeatureFlagOptions         `cmd:""`
	GetTenantFlags GetTenantFeatureFlagsOptions     `cmd:""`
	Override       OverrideFeatureFlagOptions       `cmd:""`
	GetOverride    GetFeatureFlagOverrideOptions    `cmd:""`
	ListOverrides  ListFeatureFlagOverridesOptions  `cmd:""`
	DeleteOverride DeleteFeatureFlagOverrideOptions `cmd:""`
}

type AddFeatureFlagOptions struct {
	FlagName    string  `help:"The name of the flag to add." name:"flag-name" short:"f" required:""`
	Percentage  float64 `help:"The % of customers getting the new flag by default." name:"percentage" short:"p" default:"0.0"`
	Description string  `help:"The description of the flag to add. Optional." name:"description" short:"D" optional:""`
}

func (o *AddFeatureFlagOptions) Run(ctx context.Context, s *SharedOptions) error {
	if o.Percentage < 0 || o.Percentage > 1 {
		return fmt.Errorf("percentage must be between 0.0 and 1.0")
	}

	if s.DelegatedAuthType != nil || s.DelegatedToken != nil {
		return fmt.Errorf(delegatedAuthNotSupported, "feature-flag add")
	}
	if err := ensureNoFeatureFlags(s, "feature-flag add"); err != nil {
		return err
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
	if err := ensureNoFeatureFlags(s, "feature-flag list"); err != nil {
		return err
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
	if err := ensureNoFeatureFlags(s, "feature-flag get"); err != nil {
		return err
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
	if err := ensureNoFeatureFlags(s, "feature-flag delete"); err != nil {
		return err
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

	if err := ensureNoFeatureFlags(s, "feature-flag update"); err != nil {
		return err
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

type GetTenantFeatureFlagsOptions struct {
	TenantID string `help:"The id of the tenant to get the flags for." name:"tenant-id" short:"i" required:""`
}

func (o *GetTenantFeatureFlagsOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.GetTenantFeatureFlagsRequest{TenantID: o.TenantID}
	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)
	resp, err := s.Client.GetTenantFeatureFlags(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(resp)
}

type OverrideFeatureFlagOptions struct {
	TenantID string `help:"The id of the tenant to add the override for." name:"tenant-id" short:"i" required:""`
	FlagName string `help:"The name of the flag to get." name:"flag-name" short:"f" required:""`
	Enabled  bool   `help:"When set, enables the flag. Otherwise the flag is disabled." name:"enabled" short:"e" optional:""`
}

func (o *OverrideFeatureFlagOptions) Run(ctx context.Context, s *SharedOptions) error {
	if err := ensureNoFeatureFlags(s, "feature-flag override"); err != nil {
		return err
	}
	getReq := &eh.GetFeatureFlagOverrideRequest{
		TenantID:       o.TenantID,
		FlagName:       o.FlagName,
		IncludeDeleted: pointer(true),
	}
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)

	flag, err := s.Client.GetFeatureFlagOverride(ctx, getReq)
	if err != nil {
		if !is404(err) {
			return err
		}

		createReq := &eh.CreateFeatureFlagOverrideRequest{
			TenantID: o.TenantID,
			FlagName: o.FlagName,
			Enabled:  o.Enabled,
		}
		processDelegatedAuth(s, &createReq.DelegatedAuthInfo)

		flag, err = s.Client.CreateFeatureFlagOverride(ctx, createReq)
		if err != nil {
			return err
		}
		return printJSON(flag)
	}

	updateReq := &eh.UpdateFeatureFlagOverrideRequest{
		TenantID: o.TenantID,
		FlagName: o.FlagName,
		Version:  flag.Version,
		Enabled:  pointer(o.Enabled),
		Deleted:  pointer(false),
	}

	if flag.Deleted {
		updateReq.Deleted = pointer(false)
	}
	processDelegatedAuth(s, &updateReq.DelegatedAuthInfo)

	updated, err := s.Client.UpdateFeatureFlagOverride(ctx, updateReq)
	if err != nil {
		return err
	}
	return printJSON(updated)
}

type GetFeatureFlagOverrideOptions struct {
	TenantID       string `help:"The id of the tenant to fetch the override for." name:"tenant-id" short:"i" required:""`
	FlagName       string `help:"The name of the flag to fetch the override for." name:"flag-name" short:"f" required:""`
	IncludeDeleted bool   `help:"Set to return information about deleted overrides or deleted flags." name:"include-deleted" short:"d" optional:""`
}

func (o *GetFeatureFlagOverrideOptions) Run(ctx context.Context, s *SharedOptions) error {
	if err := ensureNoFeatureFlags(s, "feature-flag get-override"); err != nil {
		return err
	}
	req := &eh.GetFeatureFlagOverrideRequest{
		TenantID:       o.TenantID,
		FlagName:       o.FlagName,
		IncludeDeleted: pointer(o.IncludeDeleted),
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	flag, err := s.Client.GetFeatureFlagOverride(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(flag)
}

type DeleteFeatureFlagOverrideOptions struct {
	TenantID string `help:"The id of the tenant to delete the override for." name:"tenant-id" short:"i" required:""`
	FlagName string `help:"The name of the flag to delete the override for." name:"flag-name" short:"f" required:""`
}

func (o *DeleteFeatureFlagOverrideOptions) Run(ctx context.Context, s *SharedOptions) error {
	if err := ensureNoFeatureFlags(s, "feature-flag delete-override"); err != nil {
		return err
	}
	getReq := &eh.GetFeatureFlagOverrideRequest{
		TenantID: o.TenantID,
		FlagName: o.FlagName,
	}
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)

	override, err := s.Client.GetFeatureFlagOverride(ctx, getReq)
	if err != nil {
		return err
	}

	delReq := &eh.DeleteFeatureFlagOverrideRequest{
		TenantID: o.TenantID,
		FlagName: o.FlagName,
		Version:  override.Version,
	}
	processDelegatedAuth(s, &delReq.DelegatedAuthInfo)

	return s.Client.DeleteFeatureFlagOverride(ctx, delReq)
}

type ListFeatureFlagOverridesOptions struct {
	TenantID       string `help:"The id of the tenant to list overrides for." name:"tenant-id" short:"i" required:""`
	IncludeDeleted bool   `help:"When set, includes deleted overrides and overrides for deleted flags in the results." name:"include-deleted" short:"d" optional:""`
}

func (o *ListFeatureFlagOverridesOptions) Run(ctx context.Context, s *SharedOptions) error {
	if err := ensureNoFeatureFlags(s, "feature-flag list-overrides"); err != nil {
		return err
	}
	var token *string
	for {
		req := &eh.ListFeatureFlagOverridesRequest{
			TenantID:       o.TenantID,
			Token:          token,
			IncludeDeleted: pointer(o.IncludeDeleted),
		}
		processDelegatedAuth(s, &req.DelegatedAuthInfo)

		resp, err := s.Client.ListFeatureFlagOverrides(ctx, req)
		if err != nil {
			return err
		}
		for _, override := range resp.FeatureFlagOverrides {
			if err := printJSON(override); err != nil {
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
