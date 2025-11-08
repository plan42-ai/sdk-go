package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/alecthomas/kong"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/debugging-sucks/clock"
	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
)

const (
	delegatedAuthNotSupported = "delegated auth is not supported for %s"
	featureFlagsNotSupported  = "feature flags not supported for %s"
)

type SharedOptions struct {
	Endpoint          string     `help:"Set to override the api endpoint." optional:""`
	Insecure          bool       `help:"Don't validate the api cert." optional:""`
	Dev               bool       `help:"Point at the dev api endpoint (api.dev.x2n.ai) by default." optional:""`
	Local             bool       `help:"Short for --endpoint localhost:7443 --insecure"`
	DelegatedAuthType *string    `help:"The delegated auth type to use." optional:""`
	DelegatedToken    *string    `help:"The delegated JWT token to use." optional:""`
	FeatureFlags      *string    `help:"The JSON file containing feature flag overrides." name:"feature-flags" short:"F" optional:""`
	ShowSecrets       bool       `help:"Don't mask secrets in command output." short:"s"`
	Client            *eh.Client `kong:"-"`
}

func processDelegatedAuth(shared *SharedOptions, info *eh.DelegatedAuthInfo) {
	if shared.DelegatedAuthType != nil {
		info.AuthType = pointer(eh.AuthorizationType(*shared.DelegatedAuthType))
	}
	info.JWT = shared.DelegatedToken
}

func pointer[T any](value T) *T {
	return &value
}

func printJSON(resp any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

func is404(err error) bool {
	var apiErr *eh.Error
	if errors.As(err, &apiErr) {
		return apiErr.ResponseCode == http.StatusNotFound
	}
	return false
}

func validateJSONFeatureFlags(jsonPath string, featureFlags *string) error {
	if featureFlags != nil && jsonPath == *featureFlags {
		return fmt.Errorf("the --json and --feature-flags options must be different")
	}
	return nil
}

func loadFeatureFlags(s *SharedOptions, f *eh.FeatureFlags) error {
	if s.FeatureFlags == nil {
		return nil
	}
	var reader *os.File
	if *s.FeatureFlags == "-" {
		reader = os.Stdin
	} else {
		var err error
		reader, err = os.Open(*s.FeatureFlags)
		if err != nil {
			return nil
		}
		defer reader.Close()
	}

	return json.NewDecoder(reader).Decode(&f.FeatureFlags)
}

//nolint:revive // readJsonFile matches required naming.
func readJsonFile[T any](fileName string, ptr *T) error {
	if fileName == "-" {
		return json.NewDecoder(os.Stdin).Decode(ptr)
	}

	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewDecoder(file).Decode(ptr)
}

func ensureNoFeatureFlags(s *SharedOptions, cmd string) error {
	if s.FeatureFlags != nil {
		return fmt.Errorf(featureFlagsNotSupported, cmd)
	}
	return nil
}

type Options struct {
	SharedOptions
	Tenant      TenantOptions      `cmd:""`
	Policies    PolicyOptions      `cmd:""`
	Github      GithubOptions      `cmd:""`
	UIToken     UITokenOptions     `cmd:""`
	Environment EnvironmentOptions `cmd:""`
	Task        TaskOptions        `cmd:""`
	Turn        TurnOptions        `cmd:""`
	Logs        LogsOptions        `cmd:""`
	FeatureFlag FeatureFlagOptions `cmd:""`
	Workstream  WorkstreamOptions  `cmd:""`
	Runner      RunnerOptions      `cmd:""`
	Ctx         context.Context    `kong:"-"`
}

func main() {
	var options Options
	kongctx := kong.Parse(&options)

	err := postProcessOptions(&options)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	err = dispatchCommand(kongctx, &options)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		var conflictError *eh.ConflictError
		if errors.As(err, &conflictError) && conflictError.Current != nil {
			_ = printJSON(conflictError.Current)
		}
		os.Exit(2)
	}
}

// nolint: gocyclo
func dispatchCommand(kongctx *kong.Context, options *Options) error {
	switch kongctx.Command() {
	case "tenant create-user":
		return options.Tenant.CreateUser.Run(options.Ctx, &options.SharedOptions)
	case "tenant current-user":
		return options.Tenant.CurrentUser.Run(options.Ctx, &options.SharedOptions)
	case "tenant get":
		return options.Tenant.Get.Run(options.Ctx, &options.SharedOptions)
	case "tenant list":
		return options.Tenant.List.Run(options.Ctx, &options.SharedOptions)
	case "policies list":
		return options.Policies.List.Run(options.Ctx, &options.SharedOptions)
	case "ui-token generate":
		return options.UIToken.Generate.Run(options.Ctx, &options.SharedOptions)
	case "github add-org":
		return options.Github.AddOrg.Run(options.Ctx, &options.SharedOptions)
	case "github add-connection":
		return options.Github.AddConnection.Run(options.Ctx, &options.SharedOptions)
	case "github list-connections":
		return options.Github.ListConnections.Run(options.Ctx, &options.SharedOptions)
	case "github get-connection":
		return options.Github.GetConnection.Run(options.Ctx, &options.SharedOptions)
	case "github update-connection":
		return options.Github.UpdateConnection.Run(options.Ctx, &options.SharedOptions)
	case "github delete-connection":
		return options.Github.DeleteConnection.Run(options.Ctx, &options.SharedOptions)
	case "github list-orgs":
		return options.Github.ListOrgs.Run(options.Ctx, &options.SharedOptions)
	case "github get-org":
		return options.Github.GetOrg.Run(options.Ctx, &options.SharedOptions)
	case "github update-org":
		return options.Github.UpdateOrg.Run(options.Ctx, &options.SharedOptions)
	case "github delete-org":
		return options.Github.DeleteOrg.Run(options.Ctx, &options.SharedOptions)
	case "github find-users":
		return options.Github.FindUsers.Run(options.Ctx, &options.SharedOptions)
	case "github get-tenant-creds":
		return options.Github.GetTenantCreds.Run(options.Ctx, &options.SharedOptions)
	case "github update-tenant-creds":
		return options.Github.UpdateTenantCreds.Run(options.Ctx, &options.SharedOptions)
	case "environment create":
		return options.Environment.Create.Run(options.Ctx, &options.SharedOptions)
	case "environment get":
		return options.Environment.Get.Run(options.Ctx, &options.SharedOptions)
	case "environment update":
		return options.Environment.Update.Run(options.Ctx, &options.SharedOptions)
	case "environment delete":
		return options.Environment.Delete.Run(options.Ctx, &options.SharedOptions)
	case "environment list":
		return options.Environment.List.Run(options.Ctx, &options.SharedOptions)
	case "task create":
		return options.Task.Create.Run(options.Ctx, &options.SharedOptions)
	case "task update":
		return options.Task.Update.Run(options.Ctx, &options.SharedOptions)
	case "task delete":
		return options.Task.Delete.Run(options.Ctx, &options.SharedOptions)
	case "task list":
		return options.Task.List.Run(options.Ctx, &options.SharedOptions)
	case "task get":
		return options.Task.Get.Run(options.Ctx, &options.SharedOptions)
	case "turn create":
		return options.Turn.Create.Run(options.Ctx, &options.SharedOptions)
	case "turn list":
		return options.Turn.List.Run(options.Ctx, &options.SharedOptions)
	case "turn update":
		return options.Turn.Update.Run(options.Ctx, &options.SharedOptions)
	case "turn get":
		return options.Turn.Get.Run(options.Ctx, &options.SharedOptions)
	case "turn get-last":
		return options.Turn.GetLast.Run(options.Ctx, &options.SharedOptions)
	case "logs stream":
		return options.Logs.Stream.Run(options.Ctx, &options.SharedOptions)
	case "logs upload":
		return options.Logs.Upload.Run(options.Ctx, &options.SharedOptions)
	case "feature-flag add":
		return options.FeatureFlag.Add.Run(options.Ctx, &options.SharedOptions)
	case "feature-flag list":
		return options.FeatureFlag.List.Run(options.Ctx, &options.SharedOptions)
	case "feature-flag get":
		return options.FeatureFlag.Get.Run(options.Ctx, &options.SharedOptions)
	case "feature-flag delete":
		return options.FeatureFlag.Delete.Run(options.Ctx, &options.SharedOptions)
	case "feature-flag update":
		return options.FeatureFlag.Update.Run(options.Ctx, &options.SharedOptions)
	case "feature-flag get-tenant-flags":
		return options.FeatureFlag.GetTenantFlags.Run(options.Ctx, &options.SharedOptions)
	case "feature-flag override":
		return options.FeatureFlag.Override.Run(options.Ctx, &options.SharedOptions)
	case "feature-flag get-override":
		return options.FeatureFlag.GetOverride.Run(options.Ctx, &options.SharedOptions)
	case "feature-flag delete-override":
		return options.FeatureFlag.DeleteOverride.Run(options.Ctx, &options.SharedOptions)
	case "feature-flag list-overrides":
		return options.FeatureFlag.ListOverrides.Run(options.Ctx, &options.SharedOptions)
	case "workstream create":
		return options.Workstream.Create.Run(options.Ctx, &options.SharedOptions)
	case "workstream get":
		return options.Workstream.Get.Run(options.Ctx, &options.SharedOptions)
	case "workstream delete":
		return options.Workstream.Delete.Run(options.Ctx, &options.SharedOptions)
	case "workstream list":
		return options.Workstream.List.Run(options.Ctx, &options.SharedOptions)
	case "workstream update":
		return options.Workstream.Update.Run(options.Ctx, &options.SharedOptions)
	case "workstream add-short-name":
		return options.Workstream.AddShortName.Run(options.Ctx, &options.SharedOptions)
	case "workstream list-short-names":
		return options.Workstream.ListShortNames.Run(options.Ctx, &options.SharedOptions)
	case "workstream delete-short-name":
		return options.Workstream.DeleteShortName.Run(options.Ctx, &options.SharedOptions)
	case "workstream move-short-name":
		return options.Workstream.MoveShortName.Run(options.Ctx, &options.SharedOptions)
	case "runner create":
		return options.Runner.Create.Run(options.Ctx, &options.SharedOptions)
	case "runner list":
		return options.Runner.List.Run(options.Ctx, &options.SharedOptions)
	case "runner get":
		return options.Runner.Get.Run(options.Ctx, &options.SharedOptions)
	case "runner delete":
		return options.Runner.Delete.Run(options.Ctx, &options.SharedOptions)
	case "runner update":
		return options.Runner.Update.Run(options.Ctx, &options.SharedOptions)
	case "runner generate-token":
		return options.Runner.GenerateToken.Run(options.Ctx, &options.SharedOptions)
	default:
		return errors.New("unknown command")
	}
}

func postProcessOptions(o *Options) error {
	if o.Dev && o.Local {
		return errors.New("cannot use both --dev and --local options together")
	}

	if o.Local && o.Endpoint == "" {
		o.Endpoint = "https://localhost:7443"
	}

	if o.Dev && o.Endpoint == "" {
		o.Endpoint = "https://api.dev.x2n.ai"
	}

	if o.Endpoint == "" {
		o.Endpoint = "https://api.x2n.ai"
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
