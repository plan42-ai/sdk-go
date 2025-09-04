package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	"github.com/google/uuid"
)

type GithubOptions struct {
	AddOrg          AddGithubOrgOptions          `cmd:""`
	ListOrgs        ListGithubOrgsOptions        `cmd:""`
	GetOrg          GetGithubOrgOptions          `cmd:""`
	UpdateOrg       UpdateGithubOrgOptions       `cmd:""`
	DeleteOrg       DeleteGithubOrgOptions       `cmd:""`
	AssociateTenant AssociateGithubTenantOptions `cmd:""`
	GetTenantOrg    GetTenantOrgOptions          `cmd:""`
	ListTenantOrgs  ListTenantOrgsOptions        `cmd:""`
	UpdateTenantOrg UpdateTenantOrgOptions       `cmd:""`
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
	IncludeDeleted bool `help:"Include deleted github orgs" short:"d"`
}

func (o *ListGithubOrgsOptions) Run(ctx context.Context, s *SharedOptions) error {
	if s.DelegatedAuthType != nil || s.DelegatedToken != nil {
		return fmt.Errorf(delegatedAuthNotSupported, "github list-orgs")
	}
	var token *string
	for {
		req := &eh.ListGithubOrgsRequest{
			Token:          token,
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

func (o *UpdateGithubOrgOptions) Run(ctx context.Context, s *SharedOptions) error {
	if s.DelegatedAuthType != nil || s.DelegatedToken != nil {
		return fmt.Errorf(delegatedAuthNotSupported, "github update-org")
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

	var req eh.UpdateGithubOrgRequest
	if err := json.NewDecoder(reader).Decode(&req); err != nil {
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
	getReq := &eh.GetGithubOrgRequest{OrgID: o.InternalOrgID}
	org, err := s.Client.GetGithubOrg(ctx, getReq)
	if err != nil {
		return err
	}

	req := &eh.DeleteGithubOrgRequest{OrgID: o.InternalOrgID, Version: org.Version}
	return s.Client.DeleteGithubOrg(ctx, req)
}

type AssociateGithubTenantOptions struct {
	TenantID      string `help:"The tenant ID to associate the github org with" name:"tenant-id" short:"i" required:""`
	InternalOrgID string `help:"The internal org id of the github org to associate with the tenant" name:"internal-org-id" short:"O" required:""`
	JSON          string `help:"The json file to load the tenant association data from" short:"j" default:"-"`
}

func (o *AssociateGithubTenantOptions) Run(ctx context.Context, s *SharedOptions) error {
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

	var req eh.AssociateGithubOrgWithTenantRequest
	if err := json.NewDecoder(reader).Decode(&req); err != nil {
		return err
	}
	req.TenantID = o.TenantID
	req.OrgID = o.InternalOrgID
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	assoc, err := s.Client.AssociateGithubOrgWithTenant(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(assoc)
}

type GetTenantOrgOptions struct {
	TenantID       string `help:"The tenant ID to fetch the association for." name:"tenant-id" short:"i" required:""`
	InternalOrgID  string `help:"The internal org id of the associated github org." name:"internal-org-id" short:"O" required:""`
	IncludeDeleted bool   `help:"Include deleted org associations" short:"d"`
}

func (o *GetTenantOrgOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.GetTenantGithubOrgAssociationRequest{
		TenantID:       o.TenantID,
		OrgID:          o.InternalOrgID,
		IncludeDeleted: pointer(o.IncludeDeleted),
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	assoc, err := s.Client.GetTenantGithubOrgAssociation(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(assoc)
}

type ListTenantOrgsOptions struct {
	TenantID string `help:"The tenant ID to list github orgs for" name:"tenant-id" short:"i" required:""`
}

func (o *ListTenantOrgsOptions) Run(ctx context.Context, s *SharedOptions) error {
	var token *string
	for {
		req := &eh.ListTenantGithubOrgsRequest{
			TenantID: o.TenantID,
			Token:    token,
		}
		processDelegatedAuth(s, &req.DelegatedAuthInfo)

		resp, err := s.Client.ListTenantGithubOrgs(ctx, req)
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

type UpdateTenantOrgOptions struct {
	TenantID      string `help:"The tenant ID to edit the org association for" name:"tenant-id" short:"i" required:""`
	InternalOrgID string `help:"The internal org id of the github org to update" name:"internal-org-id" short:"O" required:""`
	JSON          string `help:"The json file to load the updated association metadata from" short:"j" default:"-"`
}

func (o *UpdateTenantOrgOptions) Run(ctx context.Context, s *SharedOptions) error {
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

	var req eh.UpdateTenantGithubOrgAssociationRequest
	if err := json.NewDecoder(reader).Decode(&req); err != nil {
		return err
	}

	req.TenantID = o.TenantID
	req.OrgID = o.InternalOrgID
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	getReq := &eh.GetTenantGithubOrgAssociationRequest{
		TenantID:       o.TenantID,
		OrgID:          o.InternalOrgID,
		IncludeDeleted: pointer(true),
	}
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	assoc, err := s.Client.GetTenantGithubOrgAssociation(ctx, getReq)
	if err != nil {
		return err
	}
	req.Version = assoc.Version

	updated, err := s.Client.UpdateTenantGithubOrgAssociation(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(updated)
}
