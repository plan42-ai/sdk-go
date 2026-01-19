package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/plan42-ai/sdk-go/internal/util"
	"github.com/plan42-ai/sdk-go/p42"
)

type GithubOptions struct {
	AddOrg           AddGithubOrgOptions           `cmd:"" help:"Add a GitHub organization."`
	AddConnection    AddGithubConnectionOptions    `cmd:"" help:"Add a GitHub connection to a tenant."`
	ListConnections  ListGithubConnectionsOptions  `cmd:"" help:"List Github connections for a tenant."`
	GetConnection    GetGithubConnectionOptions    `cmd:"" help:"Fetch Github connections for a tenant."`
	UpdateConnection UpdateGithubConnectionOptions `cmd:"" help:"Update a GitHub connection for a tenant."`
	DeleteConnection DeleteGithubConnectionOptions `cmd:"" help:"Permanently Delete a GitHub connection from a tenant."`
	ListOrgs         ListGithubOrgsOptions         `cmd:"" help:"List GitHub organizations."`
	GetOrg           GetGithubOrgOptions           `cmd:"" help:"Get a GitHub organization."`
	UpdateOrg        UpdateGithubOrgOptions        `cmd:"" help:"Update a GitHub organization."`
	DeleteOrg        DeleteGithubOrgOptions        `cmd:"" help:"Delete a GitHub organization."`
	SearchRepos      SearchGithubReposOptions      `cmd:"" help:"Search repositories within a GitHub organization."`
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
	IncludeDeleted bool    `help:"Include deleted github orgs" short:"d"`
	TenantID       *string `help:"The id of the tenant to list github orgs for." name:"tenant-id" short:"i" optional:""`
	ConnectionID   *string `help:"The id of the github connection to list orgs for." name:"connection-id" short:"c" optional:""`
	Search         *string `help:"A search string to filter orgs. Requires --connection-id." name:"search" optional:""`
}

func (o *ListGithubOrgsOptions) Run(ctx context.Context, s *SharedOptions) error {
	tenantProvided := o.TenantID != nil
	connectionProvided := o.ConnectionID != nil

	if tenantProvided != connectionProvided {
		return fmt.Errorf("both --tenant-id and --connection-id must be provided together")
	}

	if tenantProvided && (o.IncludeDeleted) {
		return fmt.Errorf("--tenant-id and --connection-id cannot be used with --include-deleted or --name")
	}

	if o.Search != nil && !connectionProvided {
		return fmt.Errorf("--search requires --connection-id")
	}

	if tenantProvided {
		return o.listOrgsForConnection(ctx, s)
	}
	if s.DelegatedAuthType != nil || s.DelegatedToken != nil {
		return fmt.Errorf(delegatedAuthNotSupported, "github list-orgs")
	}
	err := ensureNoFeatureFlags(s, "github list-orgs")
	if err != nil {
		return err
	}
	var token *string
	for {
		req := &p42.ListGithubOrgsRequest{
			Token:          token,
			IncludeDeleted: util.Pointer(o.IncludeDeleted),
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

func (o *ListGithubOrgsOptions) listOrgsForConnection(ctx context.Context, s *SharedOptions) error {
	req := &p42.ListOrgsForGithubConnectionRequest{
		TenantID:     *o.TenantID,
		ConnectionID: *o.ConnectionID,
		Search:       o.Search,
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

		for _, orgName := range resp.Items {
			fmt.Println(orgName)
		}

		if resp.NextToken == nil {
			break
		}

		req.Token = resp.NextToken
	}

	return nil
}

type SearchGithubReposOptions struct {
	TenantID     string `help:"The id of the tenant that owns the github connection." name:"tenant-id" short:"i" required:""`
	ConnectionID string `help:"The id of the github connection to search." name:"connection-id" short:"c" required:""`
	OrgName      string `help:"The github organization to search." name:"org" required:""`
	Search       string `help:"The search string to filter repositories." name:"search" required:""`
}

func (o *SearchGithubReposOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &p42.SearchReposRequest{
		TenantID:     o.TenantID,
		ConnectionID: o.ConnectionID,
		OrgName:      o.OrgName,
		Search:       o.Search,
	}
	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)
	for {
		resp, err := s.Client.SearchRepos(ctx, req)
		if err != nil {
			return err
		}

		for _, repo := range resp.Items {
			fmt.Println(repo)
		}

		if resp.NextToken == nil {
			break
		}

		req.Token = resp.NextToken
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
		IncludeDeleted: util.Pointer(o.IncludeDeleted),
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

	getReq := &p42.GetGithubOrgRequest{OrgID: o.InternalOrgID, IncludeDeleted: util.Pointer(true)}
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
