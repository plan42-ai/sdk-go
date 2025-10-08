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
