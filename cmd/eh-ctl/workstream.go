package main

import (
	"context"
	"fmt"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	"github.com/google/uuid"
)

// WorkstreamOptions is the root for all workstream related sub-commands.
type WorkstreamOptions struct {
	Create          CreateWorkstreamOptions          `cmd:"" help:"Create a new workstream."`
	Get             GetWorkstreamOptions             `cmd:"" help:"Get a workstream by ID."`
	Delete          DeleteWorkstreamOptions          `cmd:"" help:"Soft delete a workstream."`
	List            ListWorkstreamsOptions           `cmd:"" help:"List workstreams for a tenant."`
	Update          UpdateWorkstreamOptions          `cmd:"" help:"Update a workstream."`
	AddShortName    AddWorkstreamShortNameOptions    `cmd:"" help:"Add a short name to a workstream."`
	ListShortNames  ListWorkstreamShortNamesOptions  `cmd:"" help:"List shortnames for a workstream or tenant."`
	DeleteShortName DeleteWorkstreamShortNameOptions `cmd:"" help:"Permanently Delete a short name from a tenant."`
	MoveShortName   MoveShortNameOptions             `cmd:"" help:"Move a short name from one workstream to another."`
}

// CreateWorkstreamOptions contains the flags for the `workstream create` command.
type CreateWorkstreamOptions struct {
	TenantID string `help:"The id of the tenant to create the workstream in." name:"tenant-id" short:"i" required:""`
	JSON     string `help:"The file containing the workstream JSON. Use '-' to read from stdin." short:"j" default:"-"`
}

func (o *CreateWorkstreamOptions) Run(ctx context.Context, s *SharedOptions) error {
	if err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags); err != nil {
		return err
	}
	var req eh.CreateWorkstreamRequest
	err := readJsonFile(o.JSON, &req)
	if err != nil {
		return err
	}

	// Load feature flags after the JSON has been decoded so that CLI overrides
	// are applied on top.
	err = loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}

	req.TenantID = o.TenantID
	req.WorkstreamID = uuid.NewString()

	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	ws, err := s.Client.CreateWorkstream(ctx, &req)
	if err != nil {
		return err
	}

	return printJSON(ws)
}

// MoveShortNameOptions contains the flags for the `workstream move-short-name` command.
type MoveShortNameOptions struct {
	TenantID      string  `help:"The id of the tenant to move the short name in." name:"tenant-id" short:"i" required:""`
	WorkstreamID  string  `help:"The id of the workstream to move the short name to." name:"workstream-id" short:"w" required:""`
	ShortName     string  `help:"The short name to move." name:"short-name" short:"S" required:""`
	NewSourceName *string `help:"Optional string indicating a new short name to add to the source workstream (e.g. Archive-1234)." name:"new-source-name"`
}

func (o *MoveShortNameOptions) Run(ctx context.Context, s *SharedOptions) error {
	listReq := &eh.ListWorkstreamsRequest{
		TenantID:       o.TenantID,
		IncludeDeleted: pointer(true),
		ShortName:      pointer(o.ShortName),
	}

	if err := loadFeatureFlags(s, &listReq.FeatureFlags); err != nil {
		return err
	}
	processDelegatedAuth(s, &listReq.DelegatedAuthInfo)

	srcResp, err := s.Client.ListWorkstreams(ctx, listReq)
	if err != nil {
		return err
	}

	if len(srcResp.Items) == 0 {
		return fmt.Errorf("short name %q not found in tenant %q", o.ShortName, o.TenantID)
	}

	// Fetch destination workstream to get its current version
	getDestReq := &eh.GetWorkstreamRequest{
		TenantID:       o.TenantID,
		WorkstreamID:   o.WorkstreamID,
		IncludeDeleted: pointer(true),
	}
	getDestReq.FeatureFlags = listReq.FeatureFlags
	processDelegatedAuth(s, &getDestReq.DelegatedAuthInfo)

	destWs, err := s.Client.GetWorkstream(ctx, getDestReq)
	if err != nil {
		return err
	}

	moveReq := eh.MoveShortNameRequest{
		TenantID:                     o.TenantID,
		Name:                         o.ShortName,
		SourceWorkstreamID:           srcResp.Items[0].WorkstreamID,
		DestinationWorkstreamID:      o.WorkstreamID,
		SourceWorkstreamVersion:      srcResp.Items[0].Version,
		DestinationWorkstreamVersion: destWs.Version,
		SetDefaultOnDestination:      false,
	}

	if o.NewSourceName != nil && *o.NewSourceName != "" {
		moveReq.ReplacementName = o.NewSourceName
	}

	moveReq.FeatureFlags = getDestReq.FeatureFlags
	processDelegatedAuth(s, &moveReq.DelegatedAuthInfo)

	moveResp, err := s.Client.MoveShortName(ctx, &moveReq)
	if err != nil {
		return err
	}

	return printJSON(moveResp)
}

// DeleteWorkstreamShortNameOptions contains the flags for the `workstream delete-short-name` command.
type DeleteWorkstreamShortNameOptions struct {
	TenantID     string `help:"The id of the tenant to delete the short name from." name:"tenant-id" short:"i" required:""`
	WorkstreamID string `help:"The id of the workstream to delete the short name from." name:"workstream-id" short:"w" required:""`
	ShortName    string `help:"The short name to delete." name:"short-name" short:"S" required:""`
}

func (o *DeleteWorkstreamShortNameOptions) Run(ctx context.Context, s *SharedOptions) error {
	// First, retrieve the workstream so we can obtain its current version for optimistic locking.

	getReq := &eh.GetWorkstreamRequest{
		TenantID:     o.TenantID,
		WorkstreamID: o.WorkstreamID,
	}

	if err := loadFeatureFlags(s, &getReq.FeatureFlags); err != nil {
		return err
	}

	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)

	ws, err := s.Client.GetWorkstream(ctx, getReq)
	if err != nil {
		return err
	}

	req := &eh.DeleteWorkstreamShortNameRequest{
		TenantID:     o.TenantID,
		WorkstreamID: o.WorkstreamID,
		Name:         o.ShortName,
		Version:      ws.Version,
	}

	req.FeatureFlags = getReq.FeatureFlags
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	return s.Client.DeleteWorkstreamShortName(ctx, req)
}

// ListWorkstreamShortNamesOptions contains the flags for the
// `workstream list-short-names` command.
type ListWorkstreamShortNamesOptions struct {
	TenantID     string  `help:"The id of the tenant to list short names for." name:"tenant-id" short:"i" required:""`
	WorkstreamID *string `help:"Optional. When set, filter short names based on workstream." name:"workstream-id" short:"w" optional:""`
}

// Run executes the `workstream list-short-names` command.
func (o *ListWorkstreamShortNamesOptions) Run(ctx context.Context, s *SharedOptions) error {
	// Build the initial request.
	req := &eh.ListWorkstreamShortNamesRequest{
		TenantID: o.TenantID,
	}

	// Load feature flag overrides, if any were provided at the CLI level.
	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return err
	}

	// Apply optional workstream filter.
	if o.WorkstreamID != nil {
		req.WorkstreamID = o.WorkstreamID
	}

	var token *string
	for {
		req.Token = token

		// Apply delegated auth, if configured.
		processDelegatedAuth(s, &req.DelegatedAuthInfo)

		resp, err := s.Client.ListWorkstreamShortNames(ctx, req)
		if err != nil {
			return err
		}

		// Print each short name on its own line.
		for _, sn := range resp.Items {
			_ = printJSON(sn)
		}

		if resp.NextToken == nil {
			break
		}
		token = resp.NextToken
	}

	return nil
}

// UpdateWorkstreamOptions contains the flags for the `workstream update` command.
type UpdateWorkstreamOptions struct {
	TenantID     string `help:"The id of the tenant that owns the workstream." name:"tenant-id" short:"i" required:""`
	WorkstreamID string `help:"The id of the workstream to update." name:"workstream-id" short:"w" required:""`
	JSON         string `help:"The file containing the workstream JSON. Use '-' to read from stdin." short:"j" default:"-"`
}

// Run executes the `workstream update` command.
// nolint: dupl
func (o *UpdateWorkstreamOptions) Run(ctx context.Context, s *SharedOptions) error {
	if err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags); err != nil {
		return err
	}
	var req eh.UpdateWorkstreamRequest
	err := readJsonFile(o.JSON, &req)
	if err != nil {
		return err
	}

	// Load feature flags overrides after JSON so CLI overrides win.
	err = loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}

	// Populate path parameters
	req.TenantID = o.TenantID
	req.WorkstreamID = o.WorkstreamID

	// Retrieve current workstream to get its version for concurrency control.
	getReq := &eh.GetWorkstreamRequest{
		TenantID:       o.TenantID,
		WorkstreamID:   o.WorkstreamID,
		IncludeDeleted: pointer(true),
	}
	getReq.FeatureFlags = req.FeatureFlags
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)

	ws, err := s.Client.GetWorkstream(ctx, getReq)
	if err != nil {
		return err
	}

	req.Version = ws.Version
	req.FeatureFlags = getReq.FeatureFlags
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	updated, err := s.Client.UpdateWorkstream(ctx, &req)
	if err != nil {
		return err
	}

	return printJSON(updated)
}

// ListWorkstreamsOptions contains the flags for the `workstream list` command.
type ListWorkstreamsOptions struct {
	TenantID       string  `help:"The id of the tenant to list workstreams for." name:"tenant-id" short:"i" required:""`
	IncludeDeleted bool    `help:"When set, include deleted workstreams in the list." short:"d" optional:""`
	ShortName      *string `help:"When set, filter workstreams based on short name." name:"short-name" short:"S" optional:""`
}

// Run executes the `workstream list` command.
func (o *ListWorkstreamsOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.ListWorkstreamsRequest{
		TenantID:       o.TenantID,
		IncludeDeleted: pointer(o.IncludeDeleted),
		ShortName:      o.ShortName,
	}

	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return err
	}

	var token *string
	for {
		req.Token = token

		processDelegatedAuth(s, &req.DelegatedAuthInfo)

		resp, err := s.Client.ListWorkstreams(ctx, req)
		if err != nil {
			return err
		}

		for _, ws := range resp.Items {
			if err := printJSON(ws); err != nil {
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

// GetWorkstreamOptions contains the flags for the `workstream get` command.
type GetWorkstreamOptions struct {
	TenantID       string `help:"The id of the tenant to get the workstream from." name:"tenant-id" short:"i" required:""`
	WorkstreamID   string `help:"The id of the workstream to get." name:"workstream-id" short:"w" required:""`
	IncludeDeleted bool   `help:"Include deleted workstreams in the result." short:"d" optional:""`
}

func (o *GetWorkstreamOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.GetWorkstreamRequest{
		TenantID:       o.TenantID,
		WorkstreamID:   o.WorkstreamID,
		IncludeDeleted: pointer(o.IncludeDeleted),
	}

	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return err
	}

	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	ws, err := s.Client.GetWorkstream(ctx, req)
	if err != nil {
		return err
	}

	return printJSON(ws)
}

// DeleteWorkstreamOptions contains the flags for the `workstream delete` command.
type DeleteWorkstreamOptions struct {
	TenantID     string `help:"The id of the tenant to delete the workstream from." name:"tenant-id" short:"i" required:""`
	WorkstreamID string `help:"The id of the workstream to delete." name:"workstream-id" short:"w" required:""`
}

func (o *DeleteWorkstreamOptions) Run(ctx context.Context, s *SharedOptions) error {
	// Build a GetWorkstream request to fetch the current version so we can pass
	// the appropriate If-Match header in the delete call.
	getReq := &eh.GetWorkstreamRequest{
		TenantID:     o.TenantID,
		WorkstreamID: o.WorkstreamID,
	}

	// Load feature flags from shared options (if any).
	if err := loadFeatureFlags(s, &getReq.FeatureFlags); err != nil {
		return err
	}

	// Attach delegated auth info if provided.
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)

	ws, err := s.Client.GetWorkstream(ctx, getReq)
	if err != nil {
		return err
	}

	delReq := &eh.DeleteWorkstreamRequest{
		TenantID:     o.TenantID,
		WorkstreamID: o.WorkstreamID,
		Version:      ws.Version,
	}
	delReq.FeatureFlags = getReq.FeatureFlags
	processDelegatedAuth(s, &delReq.DelegatedAuthInfo)

	return s.Client.DeleteWorkstream(ctx, delReq)
}

// AddWorkstreamShortNameOptions contains the flags for the `workstream add-short-name` command.
type AddWorkstreamShortNameOptions struct {
	TenantID     string `help:"The id of the tenant to add the short name to." name:"tenant-id" short:"i" required:""`
	WorkstreamID string `help:"The id of the workstream to add the short name to." name:"workstream-id" short:"w" required:""`
	ShortName    string `help:"The short name to add." name:"short-name" short:"S" required:""`
}

func (o *AddWorkstreamShortNameOptions) Run(ctx context.Context, s *SharedOptions) error {
	// Retrieve the current version of the workstream first so that we can set
	// the correct If-Match header when adding the short name.

	getReq := &eh.GetWorkstreamRequest{
		TenantID:     o.TenantID,
		WorkstreamID: o.WorkstreamID,
	}

	// Load feature flags (if any) so they are applied to subsequent requests.
	if err := loadFeatureFlags(s, &getReq.FeatureFlags); err != nil {
		return err
	}

	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)

	ws, err := s.Client.GetWorkstream(ctx, getReq)
	if err != nil {
		return err
	}

	req := eh.AddWorkstreamShortNameRequest{
		TenantID:          o.TenantID,
		WorkstreamID:      o.WorkstreamID,
		Name:              o.ShortName,
		WorkstreamVersion: ws.Version,
		FeatureFlags:      getReq.FeatureFlags,
	}

	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	return s.Client.AddWorkstreamShortName(ctx, &req)
}
