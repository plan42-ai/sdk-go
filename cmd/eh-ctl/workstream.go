package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	"github.com/google/uuid"
)

// WorkstreamOptions is the root for all workstream related sub-commands.
type WorkstreamOptions struct {
	Create CreateWorkstreamOptions `cmd:""`
	Get    GetWorkstreamOptions    `cmd:""`
	Delete DeleteWorkstreamOptions `cmd:""`
	List   ListWorkstreamsOptions  `cmd:""`
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

	var req eh.CreateWorkstreamRequest
	if err := json.NewDecoder(reader).Decode(&req); err != nil {
		return err
	}

	// Load feature flags after the JSON has been decoded so that CLI overrides
	// are applied on top.
	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
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

// ListWorkstreamsOptions contains the flags for the `workstream list` command.
type ListWorkstreamsOptions struct {
	TenantID       string `help:"The id of the tenant to list workstreams for." name:"tenant-id" short:"i" required:""`
	IncludeDeleted bool   `help:"When set, include deleted workstreams in the list." short:"d" optional:""`
}

// Run executes the `workstream list` command.
func (o *ListWorkstreamsOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.ListWorkstreamsRequest{
		TenantID:       o.TenantID,
		IncludeDeleted: pointer(o.IncludeDeleted),
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

		for _, ws := range resp.Workstreams {
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
