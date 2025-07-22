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
	FullName   string  `help:"The user's full name" name:"full-name" short:"n" required:""`
	FirstName  string  `help:"The user's first name" name:"first-name" short:"f" required:""`
	LastName   string  `help:"The user's last name" name:"last-name" short:"l" required:""`
	Email      string  `help:"The user's email address" short:"e" required:""`
	PictureURL *string `help:"The user's profile picture URL" short:"p" required:""`
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
	TenantID string `help:"The ID of the tenant to fetch" short:"i" required:""`
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
	TenantID string `help:"The ID of the tenant to list policies for" short:"i" required:""`
}

type GenerateUITokenOptions struct {
	TenantID string `help:"The ID of the tenant to generate the Web UI token for" short:"i" required:""`
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
	TenantID string `help:"The tenant ID to create the environment for" short:"i" required:""`
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
	TenantID       string `help:"The tennant ID that owns the environment being fetched." short:"i" required:""`
	EnvironmentID  string `help:"The environment ID to fetch" name:"environment-id" short:"e"`
	IncludeDeleted bool   `help:"Include deleted environments in the list." short:"d" optional:""`
}

func (o *GetEnvironmentOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.GetEnvironmentRequest{
		TenantID:       o.TenantID,
		EnvironmentID:  o.EnvironmentID,
		IncludeDeleted: pointer(o.IncludeDeleted),
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	env, err := s.Client.GetEnvironment(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(env)
}

type UpdateEnvironmentOptions struct {
	TenantID      string `help:"The tennant ID that owns the environment being fetched." short:"i" required:""`
	EnvironmentID string `help:"The ID of the environment to update" name:"environment-id" short:"e" required:""`
	JSON          string `help:"The json file containing the environment updates" short:"j" default:"-" required:""`
}

// nolint: dupl
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

	getReq := &eh.GetEnvironmentRequest{TenantID: o.TenantID, EnvironmentID: o.EnvironmentID, IncludeDeleted: pointer(true)}
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
	TenantID       string `help:"The tennant ID that owns the environments being listed." name:"tenantid" short:"i"`
	IncludeDeleted bool   `help:"Include deleted environments in the list." short:"d"`
}

func (o *ListEnvironmentsOptions) Run(ctx context.Context, s *SharedOptions) error {
	var token *string
	for {
		req := &eh.ListEnvironmentsRequest{
			TenantID:       o.TenantID,
			Token:          token,
			IncludeDeleted: pointer(o.IncludeDeleted),
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

type GetTaskOptions struct {
	TenantID       string `help:"The ID of the tenant to list tasks for" short:"i" required:""`
	TaskID         string `help:"The ID of the task to get" short:"t" required:""`
	IncludeDeleted bool   `help:"Include deleted tasks" short:"d" optional:""`
}

func (o *GetTaskOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.GetTaskRequest{
		TenantID:       o.TenantID,
		TaskID:         o.TaskID,
		IncludeDeleted: pointer(o.IncludeDeleted),
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	task, err := s.Client.GetTask(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(task)
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

type CreateTurnOptions struct {
	TenantID string `help:"The ID of the tenant owning the task we are adding a turn to." short:"i" required:""`
	TaskID   string `help:"The task to add a turn to." short:"t" required:""`
	JSON     string `help:"The json file containing the turn definition." short:"j" default:"-"`
}

func (o *CreateTurnOptions) Run(ctx context.Context, s *SharedOptions) error {
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

	var req eh.CreateTurnRequest
	if err := json.NewDecoder(reader).Decode(&req); err != nil {
		return err
	}

	getReq := &eh.GetLastTurnRequest{TenantID: o.TenantID, TaskID: o.TaskID}
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	last, err := s.Client.GetLastTurn(ctx, getReq)
	if err != nil {
		return err
	}

	req.TenantID = o.TenantID
	req.TaskID = o.TaskID
	req.TurnIndex = last.TurnIndex + 1
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	turn, err := s.Client.CreateTurn(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(turn)
}

type ListTasksOptions struct {
	TenantID       string  `help:"The ID of the tenant to list tasks for." short:"i" required:""`
	WorkstreamID   *string `help:"Optional workstream ID. When specified tasks in that workstream are returned." short:"w" optional:""`
	IncludeDeleted bool    `help:"When set, includes deleted tasks in the results." short:"d" optional:""`
}

func (o *ListTasksOptions) Run(ctx context.Context, s *SharedOptions) error {
	var token *string
	for {
		req := &eh.ListTasksRequest{
			TenantID:       o.TenantID,
			Token:          token,
			IncludeDeleted: pointer(o.IncludeDeleted),
		}
		if o.WorkstreamID != nil {
			req.WorkstreamID = o.WorkstreamID
		}
		processDelegatedAuth(s, &req.DelegatedAuthInfo)

		resp, err := s.Client.ListTasks(ctx, req)
		if err != nil {
			return err
		}
		for _, task := range resp.Tasks {
			if err := printJSON(task); err != nil {
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

type DeleteTaskOptions struct {
	TenantID string `help:"The ID of the tennant that owns the task to delete." short:"i" required:""`
	TaskID   string `help:"The ID of the task to delete." short:"t" required:""`
}

func (o *DeleteTaskOptions) Run(ctx context.Context, s *SharedOptions) error {
	getReq := &eh.GetTaskRequest{TenantID: o.TenantID, TaskID: o.TaskID}
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	task, err := s.Client.GetTask(ctx, getReq)
	if err != nil {
		return err
	}

	req := &eh.DeleteTaskRequest{TenantID: o.TenantID, TaskID: o.TaskID, Version: task.Version}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)
	return s.Client.DeleteTask(ctx, req)
}

type CreateTaskOptions struct {
	TenantID     string  `help:"The id of the tenant owning the task to create." short:"i" required:""`
	WorkstreamID *string `help:"Optional id of the workstream to create the task in." name:"workstream-id" short:"w"`
	JSON         string  `help:"The json file to load the task from" short:"j" default:"-"`
}

func (o *CreateTaskOptions) Run(ctx context.Context, s *SharedOptions) error {
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

	var req eh.CreateTaskRequest
	if err := json.NewDecoder(reader).Decode(&req); err != nil {
		return err
	}

	req.TenantID = o.TenantID
	req.TaskID = uuid.NewString()
	req.WorkstreamID = o.WorkstreamID
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	task, err := s.Client.CreateTask(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(task)
}

type UpdateTaskOptions struct {
	TenantID string `help:"The ID of the tenant that owns the task to update." short:"i" required:""`
	TaskID   string `help:"The ID of the task to update." name:"task-id" short:"t" required:""`
	JSON     string `help:"The json file containing the updates." short:"j" default:"-"`
}

// nolint: dupl
func (o *UpdateTaskOptions) Run(ctx context.Context, s *SharedOptions) error {
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

	var req eh.UpdateTaskRequest
	if err := json.NewDecoder(reader).Decode(&req); err != nil {
		return err
	}

	req.TenantID = o.TenantID
	req.TaskID = o.TaskID

	getReq := &eh.GetTaskRequest{TenantID: o.TenantID, TaskID: o.TaskID, IncludeDeleted: pointer(true)}
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	task, err := s.Client.GetTask(ctx, getReq)
	if err != nil {
		return err
	}
	req.Version = task.Version
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	updated, err := s.Client.UpdateTask(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(updated)
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
	Task struct {
		Create CreateTaskOptions `cmd:"create"`
		Update UpdateTaskOptions `cmd:"update"`
		Delete DeleteTaskOptions `cmd:"delete"`
		List   ListTasksOptions  `cmd:"list"`
		Get    GetTaskOptions    `cmd:"get"`
	} `cmd:"task"`
	Turn struct {
		Create CreateTurnOptions `cmd:"create"`
	} `cmd:"turn"`
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
	case "task create":
		err = options.Task.Create.Run(options.Ctx, &options.SharedOptions)
	case "task update":
		err = options.Task.Update.Run(options.Ctx, &options.SharedOptions)
	case "task delete":
		err = options.Task.Delete.Run(options.Ctx, &options.SharedOptions)
	case "task list":
		err = options.Task.List.Run(options.Ctx, &options.SharedOptions)
	case "task get":
		err = options.Task.Get.Run(options.Ctx, &options.SharedOptions)
	case "turn create":
		err = options.Turn.Create.Run(options.Ctx, &options.SharedOptions)
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
