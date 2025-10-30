package main

import (
	"context"
	"fmt"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	"github.com/debugging-sucks/event-horizon-sdk-go/internal/util"
	"github.com/google/uuid"
)

type GithubOptions struct {
	AddOrg            AddGithubOrgOptions            `cmd:""`
	ListOrgs          ListGithubOrgsOptions          `cmd:""`
	GetOrg            GetGithubOrgOptions            `cmd:""`
	UpdateOrg         UpdateGithubOrgOptions         `cmd:""`
	DeleteOrg         DeleteGithubOrgOptions         `cmd:""`
	GetTenantCreds    GetTenantGithubCredsOptions    `cmd:""`
	UpdateTenantCreds UpdateTenantGithubCredsOptions `cmd:""`
	FindUsers         FindGithubUsersOptions         `cmd:""`
}

// FindGithubUsersOptions provides options for the `github find-users` command.
// Exactly one of GithubUserID or GithubLogin must be provided.
type FindGithubUsersOptions struct {
	GithubUserID   *int    `help:"The GitHub user id to search for." name:"github-user-id" short:"I" optional:""`
	GithubLogin    *string `help:"The GitHub login to search for." name:"github-login" short:"L" optional:""`
	IncludeDeleted bool    `help:"Include deleted github users" short:"d"`
}

func (o *FindGithubUsersOptions) Run(ctx context.Context, s *SharedOptions) error {
	if s.DelegatedAuthType != nil || s.DelegatedToken != nil {
		return fmt.Errorf(delegatedAuthNotSupported, "github find-users")
	}

	if err := ensureNoFeatureFlags(s, "github find-users"); err != nil {
		return err
	}

	// Ensure exactly one search parameter was provided.
	idProvided := o.GithubUserID != nil
	loginProvided := o.GithubLogin != nil
	if idProvided == loginProvided {
		return fmt.Errorf("exactly one of --github-user-id or --github-login must be provided")
	}

	var token *string
	for {
		req := &eh.FindGithubUserRequest{
			GithubID:       o.GithubUserID,
			GithubLogin:    o.GithubLogin,
			Token:          token,
			IncludeDeleted: util.Pointer(o.IncludeDeleted),
		}

		resp, err := s.Client.FindGithubUser(ctx, req)
		if err != nil {
			return err
		}

		for _, user := range resp.Users {
			if err := printJSON(user); err != nil {
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

type AddGithubOrgOptions struct {
	OrgName        string `help:"The name of the Github org to add." name:"org-name" short:"n" required:""`
	ExternalOrgID  int    `help:"The ID of the org in github." name:"external-org-id" short:"x" required:""`
	InstallationID int    `help:"The installation ID for the github app install." name:"installation-id" short:"I" required:""`
}

func (o *AddGithubOrgOptions) Run(ctx context.Context, s *SharedOptions) error {
	if s.DelegatedAuthType != nil || s.DelegatedToken != nil {
		return fmt.Errorf(delegatedAuthNotSupported, "github add-org")
	}
	if err := ensureNoFeatureFlags(s, "github add-org"); err != nil {
		return err
	}

	req := &eh.AddGithubOrgRequest{
		OrgID:          uuid.NewString(),
		OrgName:        o.OrgName,
		ExternalOrgID:  o.ExternalOrgID,
		InstallationID: o.InstallationID,
	}

	org, err := s.Client.AddGithubOrg(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(org)
}

type ListGithubOrgsOptions struct {
	Name           *string `help:"Return only the GitHub org whose name matches the provided value." name:"name" short:"n" optional:""`
	IncludeDeleted bool    `help:"Include deleted github orgs" short:"d"`
}

func (o *ListGithubOrgsOptions) Run(ctx context.Context, s *SharedOptions) error {
	if s.DelegatedAuthType != nil || s.DelegatedToken != nil {
		return fmt.Errorf(delegatedAuthNotSupported, "github list-orgs")
	}
	if err := ensureNoFeatureFlags(s, "github list-orgs"); err != nil {
		return err
	}
	var token *string
	for {
		req := &eh.ListGithubOrgsRequest{
			Token:          token,
			Name:           o.Name,
			IncludeDeleted: pointer(o.IncludeDeleted),
		}
		resp, err := s.Client.ListGithubOrgs(ctx, req)
		if err != nil {
			return err
		}
		for _, org := range resp.Orgs {
			if err := printJSON(org); err != nil {
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

type GetGithubOrgOptions struct {
	InternalOrgID  string `help:"The internal org id of the org to fetch" name:"internal-org-id" short:"O" required:""`
	IncludeDeleted bool   `help:"Include deleted orgs" short:"d" optional:""`
}

func (o *GetGithubOrgOptions) Run(ctx context.Context, s *SharedOptions) error {
	if s.DelegatedAuthType != nil || s.DelegatedToken != nil {
		return fmt.Errorf(delegatedAuthNotSupported, "github get-org")
	}
	if err := ensureNoFeatureFlags(s, "github get-org"); err != nil {
		return err
	}

	req := &eh.GetGithubOrgRequest{
		OrgID:          o.InternalOrgID,
		IncludeDeleted: pointer(o.IncludeDeleted),
	}

	org, err := s.Client.GetGithubOrg(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(org)
}

type UpdateGithubOrgOptions struct {
	InternalOrgID string `help:"The internal org id of the org to update." name:"internal-org-id" short:"O" required:""`
	JSON          string `help:"The json file containing the updates to apply." short:"j" default:"-"`
}

// nolint: dupl
func (o *UpdateGithubOrgOptions) Run(ctx context.Context, s *SharedOptions) error {
	if s.DelegatedAuthType != nil || s.DelegatedToken != nil {
		return fmt.Errorf(delegatedAuthNotSupported, "github update-org")
	}
	if err := ensureNoFeatureFlags(s, "github update-org"); err != nil {
		return err
	}
	var req eh.UpdateGithubOrgRequest
	if err := readJsonFile(o.JSON, &req); err != nil {
		return err
	}

	req.OrgID = o.InternalOrgID

	getReq := &eh.GetGithubOrgRequest{OrgID: o.InternalOrgID, IncludeDeleted: pointer(true)}
	org, err := s.Client.GetGithubOrg(ctx, getReq)
	if err != nil {
		return err
	}
	req.Version = org.Version

	updated, err := s.Client.UpdateGithubOrg(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(updated)
}

type DeleteGithubOrgOptions struct {
	InternalOrgID string `help:"The internal org id of the github org to delete" name:"internal-org-id" short:"O" required:""`
}

func (o *DeleteGithubOrgOptions) Run(ctx context.Context, s *SharedOptions) error {
	if s.DelegatedAuthType != nil || s.DelegatedToken != nil {
		return fmt.Errorf(delegatedAuthNotSupported, "github delete-org")
	}
	if err := ensureNoFeatureFlags(s, "github delete-org"); err != nil {
		return err
	}
	getReq := &eh.GetGithubOrgRequest{OrgID: o.InternalOrgID}
	org, err := s.Client.GetGithubOrg(ctx, getReq)
	if err != nil {
		return err
	}

	req := &eh.DeleteGithubOrgRequest{OrgID: o.InternalOrgID, Version: org.Version}
	return s.Client.DeleteGithubOrg(ctx, req)
}

// GetTenantGithubCredsOptions retrieves GitHub credentials for a tenant.
type GetTenantGithubCredsOptions struct {
	TenantID string `help:"The ID of the tenant to fetch GitHub credentials for." name:"tenant-id" short:"i" required:""`
}

func (o *GetTenantGithubCredsOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.GetTenantGithubCredsRequest{
		TenantID: o.TenantID,
	}

	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	resp, err := s.Client.GetTenantGithubCreds(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(resp)
}

// UpdateTenantGithubCredsOptions updates GitHub credentials for a tenant.
type UpdateTenantGithubCredsOptions struct {
	TenantID string `help:"The ID of the tenant whose GitHub credentials will be updated." name:"tenant-id" short:"i" required:""`
	JSON     string `help:"The JSON file containing the fields to update. Pass '-' to read from stdin." name:"json" short:"j" default:"-"`
}

func (o *UpdateTenantGithubCredsOptions) Run(ctx context.Context, s *SharedOptions) error { // nolint: funlen, dupl
	// Ensure user didn't specify the same file for both JSON body and feature flags.
	if err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags); err != nil {
		return err
	}
	var req eh.UpdateTenantGithubCredsRequest
	if err := readJsonFile(o.JSON, &req); err != nil {
		return err
	}

	// Populate path params.
	req.TenantID = o.TenantID

	// Load feature flags (allow optional overrides).
	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return err
	}

	// Retrieve current creds to obtain latest version for optimistic concurrency.
	getReq := &eh.GetTenantGithubCredsRequest{TenantID: o.TenantID}
	if err := loadFeatureFlags(s, &getReq.FeatureFlags); err != nil {
		return err
	}
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)

	current, err := s.Client.GetTenantGithubCreds(ctx, getReq)
	if err != nil {
		return err
	}

	req.Version = current.TenantVersion

	// Attach delegated auth information for the update request.
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	updated, err := s.Client.UpdateTenantGithubCreds(ctx, &req)
	if err != nil {
		return err
	}

	return printJSON(updated)
}
