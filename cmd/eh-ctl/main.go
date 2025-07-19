package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/debugging-sucks/clock"
	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	"github.com/google/uuid"
)

type SharedOptions struct {
	Endpoint          string     `help:"Set to override the api endpoint." optional:""`
	Insecure          bool       `help:"Don't validate the api cert." optional:""`
	Local             bool       `help:"Short for --endpoint localhost:7443 --insecure"`
	DelegatedAuthType *string    `help:"The delegated auth type to use." optional:""`
	DelegatedToken    *string    `help:"The delegated JWT token to use." optional:""`
	Client            *eh.Client `kong:"-"`
}

type CreateUserOptions struct {
	FullName   string  `help:"The user's full name" name:"full-name" short:"n"`
	FirstName  string  `help:"The user's first name" name:"first-name" short:"f"`
	LastName   string  `help:"The user's last name" name:"last-name" short:"l"`
	Email      string  `help:"The user's email address" short:"e"`
	PictureURL *string `help:"The user's profile picture URL" short:"p"`
}

func processDelegatedAuth(shared *SharedOptions, info *eh.DelegatedAuthInfo) {
	if shared.DelegatedAuthType != nil {
		info.AuthType = pointer(eh.AuthorizationType(*shared.DelegatedAuthType))
	}
	info.JWT = shared.DelegatedToken
}

func (o *CreateUserOptions) Run(ctx context.Context, shared *SharedOptions) error {
	req := &eh.CreateTenantRequest{
		TenantID:   uuid.NewString(),
		Type:       eh.TenantTypeUser,
		FullName:   &o.FullName,
		Email:      &o.Email,
		FirstName:  &o.FirstName,
		LastName:   &o.LastName,
		PictureURL: o.PictureURL,
	}
	processDelegatedAuth(shared, &req.DelegatedAuthInfo)

	t, err := shared.Client.CreateTenant(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(t)
}

func pointer[T any](value T) *T {
	return &value
}

func printJSON(resp any) error {
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
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	t, err := s.Client.GetTenant(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(t)
}

type GetCurrentUserOptions struct{}

func (o *GetCurrentUserOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.GetCurrentUserRequest{}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	t, err := s.Client.GetCurrentUser(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(t)
}

type ListPoliciesOptions struct {
	TenantID string `help:"The ID of the tenant to list policies for" short:"i"`
}

type GenerateUITokenOptions struct {
	TenantID string `help:"The ID of the tenant to generate the Web UI token for" short:"i"`
}

func (o *GenerateUITokenOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.GenerateWebUITokenRequest{
		TenantID: o.TenantID,
		TokenID:  uuid.NewString(),
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	resp, err := s.Client.GenerateWebUIToken(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(resp)
}

type CreateEnvironmentOptions struct {
	TenantID string `help:"The tenant ID to create the environment for" short:"i"`
	JSON     string `help:"The JSON file to load the environment definition from" short:"j" default:"-"`
}

func (o *CreateEnvironmentOptions) Run(ctx context.Context, s *SharedOptions) error {
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

	var req eh.CreateEnvironmentRequest
	if err := json.NewDecoder(reader).Decode(&req); err != nil {
		return err
	}

	req.TenantID = o.TenantID
	req.EnvironmentID = uuid.NewString()
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	env, err := s.Client.CreateEnvironment(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(env)
}

type GetEnvironmentOptions struct {
	TenantID      string `help:"The tennant ID that owns the environment being fetched." short:"i"`
	EnvironmentID string `help:"The environment ID to fetch" name:"environment-id" short:"e"`
}

func (o *GetEnvironmentOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.GetEnvironmentRequest{
		TenantID:      o.TenantID,
		EnvironmentID: o.EnvironmentID,
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	env, err := s.Client.GetEnvironment(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(env)
}

type UpdateEnvironmentOptions struct {
	TenantID      string `help:"The tennant ID that owns the environment being fetched." short:"i"`
	EnvironmentID string `help:"The ID of the environment to update" name:"environment-id" short:"e"`
	JSON          string `help:"The json file containing the environment updates" short:"j" default:"-"`
}

func (o *UpdateEnvironmentOptions) Run(ctx context.Context, s *SharedOptions) error {
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

	var req eh.UpdateEnvironmentRequest
	if err := json.NewDecoder(reader).Decode(&req); err != nil {
		return err
	}

	req.TenantID = o.TenantID
	req.EnvironmentID = o.EnvironmentID

	getReq := &eh.GetEnvironmentRequest{TenantID: o.TenantID, EnvironmentID: o.EnvironmentID}
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	env, err := s.Client.GetEnvironment(ctx, getReq)
	if err != nil {
		return err
	}
	req.Version = env.Version
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	updated, err := s.Client.UpdateEnvironment(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(updated)
}

type ListEnvironmentsOptions struct {
	TenantID string `help:"The tennant ID that owns the environments being listed." name:"tenantid" short:"i"`
}

func (o *ListEnvironmentsOptions) Run(ctx context.Context, s *SharedOptions) error {
	var token *string
	for {
		req := &eh.ListEnvironmentsRequest{
			TenantID: o.TenantID,
			Token:    token,
		}
		processDelegatedAuth(s, &req.DelegatedAuthInfo)

		resp, err := s.Client.ListEnvironments(ctx, req)
		if err != nil {
			return err
		}
		for _, env := range resp.Environments {
			if err := printJSON(env); err != nil {
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

type DeleteEnvironmentOptions struct {
	TenantID      string `help:"The tennant ID that owns the environment being fetched." short:"i"`
	EnvironmentID string `help:"The ID of the environment to update" name:"environment-id" short:"e"`
}

func (o *DeleteEnvironmentOptions) Run(ctx context.Context, s *SharedOptions) error {
	getReq := &eh.GetEnvironmentRequest{TenantID: o.TenantID, EnvironmentID: o.EnvironmentID}
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	env, err := s.Client.GetEnvironment(ctx, getReq)
	if err != nil {
		return err
	}

	req := &eh.DeleteEnvironmentRequest{TenantID: o.TenantID, EnvironmentID: o.EnvironmentID, Version: env.Version}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)
	return s.Client.DeleteEnvironment(ctx, req)
}

func (o *ListPoliciesOptions) Run(ctx context.Context, s *SharedOptions) error {
	var token *string
	for {
		req := &eh.ListPoliciesRequest{
			TenantID: o.TenantID,
			Token:    token,
		}
		processDelegatedAuth(s, &req.DelegatedAuthInfo)

		resp, err := s.Client.ListPolicies(ctx, req)
		if err != nil {
			return err
		}
		for _, pol := range resp.Policies {
			if err := printJSON(pol); err != nil {
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

type Options struct {
	SharedOptions
	Tenant struct {
		CreateUser  CreateUserOptions     `cmd:"create-user"`
		CurrentUser GetCurrentUserOptions `cmd:"current-user"`
		Get         GetTenantOptions      `cmd:"get"`
	} `cmd:""`
	Policies struct {
		List ListPoliciesOptions `cmd:"list"`
	} `cmd:""`
	UIToken struct {
		Generate GenerateUITokenOptions `cmd:"generate"`
	} `cmd:"ui-token"`
	Environment struct {
		Create CreateEnvironmentOptions `cmd:"create"`
		Get    GetEnvironmentOptions    `cmd:"get"`
		Update UpdateEnvironmentOptions `cmd:"update"`
		Delete DeleteEnvironmentOptions `cmd:"delete"`
		List   ListEnvironmentsOptions  `cmd:"list"`
	} `cmd:"environment"`
	Ctx context.Context `kong:"-"`
}

func main() {
	var options Options
	kongctx := kong.Parse(&options)

	err := postProcessOptions(&options)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	switch kongctx.Command() {
	case "tenant create-user":
		err = options.Tenant.CreateUser.Run(options.Ctx, &options.SharedOptions)
	case "tenant current-user":
		err = options.Tenant.CurrentUser.Run(options.Ctx, &options.SharedOptions)
	case "tenant get":
		err = options.Tenant.Get.Run(options.Ctx, &options.SharedOptions)
	case "policies list":
		err = options.Policies.List.Run(options.Ctx, &options.SharedOptions)
	case "ui-token generate":
		err = options.UIToken.Generate.Run(options.Ctx, &options.SharedOptions)
	case "environment create":
		err = options.Environment.Create.Run(options.Ctx, &options.SharedOptions)
	case "environment get":
		err = options.Environment.Get.Run(options.Ctx, &options.SharedOptions)
	case "environment update":
		err = options.Environment.Update.Run(options.Ctx, &options.SharedOptions)
	case "environment delete":
		err = options.Environment.Delete.Run(options.Ctx, &options.SharedOptions)
	case "environment list":
		err = options.Environment.List.Run(options.Ctx, &options.SharedOptions)
	default:
		err = errors.New("unknown command")
	}

	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		var conflictError *eh.ConflictError
		if errors.As(err, &conflictError) && conflictError.Current != nil {
			_ = printJSON(conflictError.Current)
		}
		os.Exit(2)
	}
}

func postProcessOptions(o *Options) error {
	if o.Local && o.Endpoint == "" {
		o.Endpoint = "https://localhost:7443"
	}

	if o.Endpoint == "" {
		o.Endpoint = "https://api.xpnt.ai"
	}

	if o.Local {
		o.Insecure = true
	}

	if (o.DelegatedAuthType == nil) != (o.DelegatedToken == nil) {
		return errors.New("if supplied, both --delegated-auth-type and --delegated-token must be provided together")
	}

	var options []eh.Option
	if o.Insecure {
		options = append(options, eh.WithInsecureSkipVerify())
	}

	o.Ctx = context.Background()
	awsCfg, err := config.LoadDefaultConfig(o.Ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	options = append(options, eh.WithSigv4Auth(awsCfg, clock.RealClock{}))
	o.Client = eh.NewClient(o.Endpoint, options...)
	return nil
}
