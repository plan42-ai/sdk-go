package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/plan42-ai/sdk-go/internal/util"
	"github.com/plan42-ai/sdk-go/p42"
)

type GithubOptions struct {
	AddOrg                AddGithubOrgOptions                `cmd:"" help:"Add a GitHub organization."`
	AddConnection         AddGithubConnectionOptions         `cmd:"" help:"Add a GitHub connection to a tenant."`
	ListConnections       ListGithubConnectionsOptions       `cmd:"" help:"List Github connections for a tenant."`
	GetConnection         GetGithubConnectionOptions         `cmd:"" help:"Fetch Github connections for a tenant."`
	UpdateConnection      UpdateGithubConnectionOptions      `cmd:"" help:"Update a GitHub connection for a tenant."`
	DeleteConnection      DeleteGithubConnectionOptions      `cmd:"" help:"Permanently Delete a GitHub connection from a tenant."`
	ListOrgsForConnection ListGithubOrgsForConnectionOptions `cmd:"" help:"List GitHub organizations for a connection."`
	ListOrgs              ListGithubOrgsOptions              `cmd:"" help:"List GitHub organizations."`
	GetOrg                GetGithubOrgOptions                `cmd:"" help:"Get a GitHub organization."`
	UpdateOrg             UpdateGithubOrgOptions             `cmd:"" help:"Update a GitHub organization."`
	DeleteOrg             DeleteGithubOrgOptions             `cmd:"" help:"Delete a GitHub organization."`
	GetTenantCreds        GetTenantGithubCredsOptions        `cmd:"" help:"Fetch GitHub credentials for a tenant."`
	UpdateTenantCreds     UpdateTenantGithubCredsOptions     `cmd:"" help:"Update GitHub credentials for a tenant."`
	FindUsers             FindGithubUsersOptions             `cmd:"" help:"Find tenants given their github login or user id."`
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
		req := &p42.FindGithubUserRequest{
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

type AddGithubConnectionOptions struct {
	TenantID string `help:"The ID of the tenant to create the connection for." name:"tenant-id" short:"i" required:""`
	JSON     string `help:"The JSON file containing the connection definition." name:"json" short:"j" default:"-"`
}

func (o *AddGithubConnectionOptions) Run(ctx context.Context, s *SharedOptions) error {
	if err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags); err != nil {
		return err
	}
	var req p42.CreateGithubConnectionRequest
	err := readJsonFile(o.JSON, &req)
	if err != nil {
		return err
	}
	err = loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	req.TenantID = o.TenantID
	req.ConnectionID = uuid.NewString()
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	connection, err := s.Client.CreateGithubConnection(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(connection)
}

type GetGithubConnectionOptions struct {
	TenantID     string `help:"The ID of the tenant that owns the connection." name:"tenant-id" short:"i" required:""`
	ConnectionID string `help:"The ID of the connection to fetch." name:"connection-id" short:"c" required:""`
}

func (o *GetGithubConnectionOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &p42.GetGithubConnectionRequest{
		TenantID:     o.TenantID,
		ConnectionID: o.ConnectionID,
	}

	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}

	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	connection, err := s.Client.GetGithubConnection(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(connection)
}

type ListGithubConnectionsOptions struct {
	TenantID string `help:"The tenant ID to list github connections for." name:"tenant-id" short:"i" required:""`
	Private  *bool  `help:"Set to filter on private / public github connections." name:"private" optional:""`
}

func (o *ListGithubConnectionsOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &p42.ListGithubConnectionsRequest{
		TenantID: o.TenantID,
		Private:  o.Private,
	}

	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}

	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	for {
		resp, err := s.Client.ListGithubConnections(ctx, req)
		if err != nil {
			return err
		}

		for _, connection := range resp.Items {
			err = printJSON(connection)
			if err != nil {
				return err
			}
		}

		if resp.NextToken == nil {
			break
		}

		req.Token = resp.NextToken
	}

	return nil
}

type ListGithubOrgsForConnectionOptions struct {
	TenantID     string `help:"The tenant ID that owns the connection." name:"tenant-id" short:"i" required:""`
	ConnectionID string `help:"The GitHub connection ID to list orgs for." name:"connection-id" short:"c" required:""`
	MaxResults   *int   `help:"Max number of orgs to return per page." name:"max-results" short:"m" optional:""`
}

func (o *ListGithubOrgsForConnectionOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &p42.ListOrgsForGithubConnectionRequest{
		TenantID:     o.TenantID,
		ConnectionID: o.ConnectionID,
		MaxResults:   o.MaxResults,
	}

	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	for {
		resp, err := s.Client.ListOrgsForGithubConnection(ctx, req)
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

		req.Token = resp.NextToken
	}

	return nil
}

type UpdateGithubConnectionOptions struct {
	TenantID     string `help:"The id of the tenant that owns the connection." name:"tenant-id" short:"i" required:""`
	ConnectionID string `help:"The ID of the connection to update." name:"connection-id" short:"c" required:""`
	JSON         string `help:"The JSON file containing the connection updates. Use '-' to read from stdin." name:"json" short:"j" default:"-"`
}

func (o *UpdateGithubConnectionOptions) Run(ctx context.Context, s *SharedOptions) error {
	err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags)
	if err != nil {
		return err
	}
	var req p42.UpdateGithubConnectionRequest
	err = readJsonFile(o.JSON, &req)
	if err != nil {
		return err
	}
	err = loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	req.TenantID = o.TenantID
	req.ConnectionID = o.ConnectionID

	getReq := &p42.GetGithubConnectionRequest{
		TenantID:     o.TenantID,
		ConnectionID: o.ConnectionID,
	}
	getReq.FeatureFlags = req.FeatureFlags
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)

	current, err := s.Client.GetGithubConnection(ctx, getReq)
	if err != nil {
		return err
	}

	req.Version = current.Version
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	updated, err := s.Client.UpdateGithubConnection(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(updated)
}

type DeleteGithubConnectionOptions struct {
	TenantID     string `help:"The ID of the tenant that owns the connection." name:"tenant-id" short:"i" required:""`
	ConnectionID string `help:"The ID of the connection to delete." name:"connection-id" short:"c" required:""`
}

func (o *DeleteGithubConnectionOptions) Run(ctx context.Context, s *SharedOptions) error {
	getReq := &p42.GetGithubConnectionRequest{
		TenantID:     o.TenantID,
		ConnectionID: o.ConnectionID,
	}
	err := loadFeatureFlags(s, &getReq.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)

	connection, err := s.Client.GetGithubConnection(ctx, getReq)
	if err != nil {
		return err
	}

	req := &p42.DeleteGithubConnectionRequest{
		TenantID:     o.TenantID,
		ConnectionID: o.ConnectionID,
		Version:      connection.Version,
	}
	req.FeatureFlags = getReq.FeatureFlags
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	return s.Client.DeleteGithubConnection(ctx, req)
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

	req := &p42.AddGithubOrgRequest{
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
		req := &p42.ListGithubOrgsRequest{
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

	req := &p42.GetGithubOrgRequest{
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
	var req p42.UpdateGithubOrgRequest
	if err := readJsonFile(o.JSON, &req); err != nil {
		return err
	}

	req.OrgID = o.InternalOrgID

	getReq := &p42.GetGithubOrgRequest{OrgID: o.InternalOrgID, IncludeDeleted: pointer(true)}
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
	getReq := &p42.GetGithubOrgRequest{OrgID: o.InternalOrgID}
	org, err := s.Client.GetGithubOrg(ctx, getReq)
	if err != nil {
		return err
	}

	req := &p42.DeleteGithubOrgRequest{OrgID: o.InternalOrgID, Version: org.Version}
	return s.Client.DeleteGithubOrg(ctx, req)
}

// GetTenantGithubCredsOptions retrieves GitHub credentials for a tenant.
type GetTenantGithubCredsOptions struct {
	TenantID string `help:"The ID of the tenant to fetch GitHub credentials for." name:"tenant-id" short:"i" required:""`
}

func (o *GetTenantGithubCredsOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &p42.GetTenantGithubCredsRequest{
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
	var req p42.UpdateTenantGithubCredsRequest
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
	getReq := &p42.GetTenantGithubCredsRequest{TenantID: o.TenantID}
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
