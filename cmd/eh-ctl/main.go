package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	"github.com/google/uuid"
)

type SharedOptions struct {
	Endpoint string     `help:"Set to override the api endpoint." optional:""`
	Insecure bool       `help:"Don't validate the api cert." optional:""`
	Local    bool       `help:"Short for --endpoint localhost:7443 --insecure"`
	Client   *eh.Client `kong:"-"`
}

type CreateUserOptions struct {
	FullName  string `help:"The user's full name" name:"full-name" short:"n"`
	FirstName string `help:"The user's first name" name:"first-name" short:"f"`
	LastName  string `help:"The user's last name" name:"last-name" short:"l"`
	Email     string `help:"The user's email address" short:"e"`
}

func (o *CreateUserOptions) Run(ctx context.Context, shared *SharedOptions) error {
	req := &eh.CreateTenantRequest{
		TenantID:  uuid.NewString(),
		Type:      eh.TenantTypeUser,
		FullName:  &o.FullName,
		Email:     &o.Email,
		FirstName: &o.FirstName,
		LastName:  &o.LastName,
	}

	t, err := shared.Client.CreateTenant(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(t)
}

func printJSON(resp *eh.Tenant) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

type GetTenantOptions struct {
	TenantID string `help:"The ID of the tenant to fetch" short:"i"`
}

func (o *GetTenantOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.GetTenantRequest{
		TenantID: o.TenantID,
	}
	t, err := s.Client.GetTenant(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(t)
}

type Options struct {
	SharedOptions
	Tenant struct {
		CreateUser CreateUserOptions `cmd:"create-user"`
		Get        GetTenantOptions  `cmd:"get"`
	} `cmd:""`
}

func main() {
	var options Options
	kongctx := kong.Parse(&options)
	ctx := context.Background()

	postProcessOptions(&options)

	var err error
	switch kongctx.Command() {
	case "tenant create-user":
		err = options.Tenant.CreateUser.Run(ctx, &options.SharedOptions)
	case "tenant get":
		err = options.Tenant.Get.Run(ctx, &options.SharedOptions)
	default:
		err = errors.New("unknown command")
	}

	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func postProcessOptions(o *Options) {
	if o.Local && o.Endpoint == "" {
		o.Endpoint = "https://localhost:7443"
	}

	if o.Endpoint == "" {
		o.Endpoint = "https://api.xpnt.ai"
	}

	if o.Local {
		o.Insecure = true
	}

	var options []eh.Option
	if o.Insecure {
		options = append(options, eh.WithInsecureSkipVerify())
	}
	o.Client = eh.NewClient(o.Endpoint, options...)
}
