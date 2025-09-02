package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/alecthomas/kong"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/debugging-sucks/clock"
	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
)

const delegatedAuthNotSupported = "delegated auth is not supported for %s"

type SharedOptions struct {
	Endpoint          string     `help:"Set to override the api endpoint." optional:""`
	Insecure          bool       `help:"Don't validate the api cert." optional:""`
	Dev               bool       `help:"Point at the dev api endpoint (api.dev.x2n.ai) by default." optional:""`
	Local             bool       `help:"Short for --endpoint localhost:7443 --insecure"`
	DelegatedAuthType *string    `help:"The delegated auth type to use." optional:""`
	DelegatedToken    *string    `help:"The delegated JWT token to use." optional:""`
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

type GetTurnOptions struct {
	TenantID       string `help:"The ID of the tennant that owns the task" name:"tenant-id" short:"i" required:""`
	TaskID         string `help:"The ID of the task containing the turn" name:"task-id" short:"t" required:""`
	TurnIndex      int    `help:"The index of the turn to fetch" name:"turn-index" short:"n" required:""`
	IncludeDeleted bool   `help:"Include the turn even if defined on a deleted task" short:"d"`
}

func (o *GetTurnOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.GetTurnRequest{
		TenantID:       o.TenantID,
		TaskID:         o.TaskID,
		TurnIndex:      o.TurnIndex,
		IncludeDeleted: pointer(o.IncludeDeleted),
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	turn, err := s.Client.GetTurn(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(turn)
}

type GetLastTurnOptions struct {
	TenantID       string `help:"The ID of the tennant that owns the task" name:"tenant-id" short:"i" required:""`
	TaskID         string `help:"The ID of the task to fetch the last turn for" name:"task-id" short:"t" required:""`
	IncludeDeleted bool   `help:"Set to return a turn on a deleted task" short:"d"`
}

func (o *GetLastTurnOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.GetLastTurnRequest{
		TenantID:       o.TenantID,
		TaskID:         o.TaskID,
		IncludeDeleted: pointer(o.IncludeDeleted),
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	turn, err := s.Client.GetLastTurn(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(turn)
}

type UpdateTurnOptions struct {
	TenantID  string `help:"The ID of the tennant that owns the task and turn" short:"i" required:""`
	TaskID    string `help:"The ID of the task that contains the turn" short:"t" required:""`
	TurnIndex int    `help:"The index of the turn to update" name:"turn-index" short:"n" required:""`
	JSON      string `help:"The json file containing the updates to make" short:"j" default:"-"`
}

func (o *UpdateTurnOptions) Run(ctx context.Context, s *SharedOptions) error {
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

	var req eh.UpdateTurnRequest
	if err := json.NewDecoder(reader).Decode(&req); err != nil {
		return err
	}

	req.TenantID = o.TenantID
	req.TaskID = o.TaskID
	req.TurnIndex = o.TurnIndex

	getReq := &eh.GetTurnRequest{TenantID: o.TenantID, TaskID: o.TaskID, TurnIndex: o.TurnIndex, IncludeDeleted: pointer(true)}
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	turn, err := s.Client.GetTurn(ctx, getReq)
	if err != nil {
		return err
	}
	req.Version = turn.Version
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	updated, err := s.Client.UpdateTurn(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(updated)
}

type ListTurnsOptions struct {
	TenantID       string `help:"The ID of the tennant that owns the task we want to list turns for." short:"i" required:""`
	TaskID         string `help:"The ID of the task we want to list turns for." short:"t" required:""`
	IncludeDeleted bool   `help:"Set to return turns even if they are define on a deleted task." short:"d"`
}

func (o *ListTurnsOptions) Run(ctx context.Context, s *SharedOptions) error {
	var token *string
	for {
		req := &eh.ListTurnsRequest{
			TenantID:       o.TenantID,
			TaskID:         o.TaskID,
			Token:          token,
			IncludeDeleted: pointer(o.IncludeDeleted),
		}
		processDelegatedAuth(s, &req.DelegatedAuthInfo)

		resp, err := s.Client.ListTurns(ctx, req)
		if err != nil {
			return err
		}
		for _, tr := range resp.Turns {
			if err := printJSON(tr); err != nil {
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

	getTaskReq := &eh.GetTaskRequest{TenantID: o.TenantID, TaskID: o.TaskID}
	processDelegatedAuth(s, &getTaskReq.DelegatedAuthInfo)
	task, err := s.Client.GetTask(ctx, getTaskReq)
	if err != nil {
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
	req.TaskVersion = task.Version
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	turn, err := s.Client.CreateTurn(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(turn)
}

type StreamLogsOptions struct {
	TenantID       string `help:"The id of the tenant that owns the task / turn to stream logs for." name:"tenant-id" short:"i" required:""`
	TaskID         string `help:"The id of the task to stream logs for." name:"task-id" short:"t" required:""`
	TurnIndex      int    `help:"The turn to stream logs for." name:"turn-index" short:"n" required:""`
	IncludeDeleted bool   `help:"Include logs for turns on deleted tasks" short:"d"`
}

func (o *StreamLogsOptions) Run(_ context.Context, s *SharedOptions) error {
	// TODO: Modify this to that NewLogStream uses the passed in context.
	ls := eh.NewLogStream(s.Client, o.TenantID, o.TaskID, o.TurnIndex, 1000, pointer(o.IncludeDeleted))
	defer ls.Close()

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	for log := range ls.Logs() {
		if err := enc.Encode(log); err != nil {
			return err
		}
	}

	return ls.ShutdownTimeout(2 * time.Second)
}

type UploadLogsOptions struct {
	TenantID  string `help:"The id of the tenant that owns the task / turn to upload logs for." name:"tenant-id" short:"i" required:""`
	TaskID    string `help:"The id of the task to upload logs for." name:"task-id" short:"t" required:""`
	TurnIndex int    `help:"The turn to upload logs for." name:"turn-index" short:"n" required:""`
	JSON      string `help:"The file containing the logs to upload." short:"j" default:"-"`
}

func (o *UploadLogsOptions) Run(ctx context.Context, s *SharedOptions) error {
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

	getTurnReq := &eh.GetTurnRequest{TenantID: o.TenantID, TaskID: o.TaskID, TurnIndex: o.TurnIndex}
	processDelegatedAuth(s, &getTurnReq.DelegatedAuthInfo)
	turn, err := s.Client.GetTurn(ctx, getTurnReq)
	if err != nil {
		return err
	}

	lastReq := &eh.GetLastTurnLogRequest{TenantID: o.TenantID, TaskID: o.TaskID, TurnIndex: o.TurnIndex}
	processDelegatedAuth(s, &lastReq.DelegatedAuthInfo)
	last, err := s.Client.GetLastTurnLog(ctx, lastReq)
	if err != nil && !is404(err) {
		return err
	}
	if last == nil {
		last = &eh.LastTurnLog{}
	}

	logsCh := make(chan eh.TurnLog, 1000)
	lu := eh.NewLogUploader(&eh.LogUploaderConfig{
		Client:     s.Client,
		TenantID:   o.TenantID,
		TaskID:     o.TaskID,
		TurnIndex:  o.TurnIndex,
		Version:    turn.Version,
		StartIndex: last.Index + 1,
		Logs:       logsCh,
	})

	dec := json.NewDecoder(reader)
	for {
		var entry eh.TurnLog
		if err := dec.Decode(&entry); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			_ = lu.Close()
			return err
		}
		logsCh <- entry
	}
	close(logsCh)
	return lu.ShutdownTimeout(10 * time.Second)
}

func is404(err error) bool {
	var apiErr *eh.Error
	if errors.As(err, &apiErr) {
		return apiErr.ResponseCode == http.StatusNotFound
	}
	return false
}

type Options struct {
	SharedOptions
	Tenant      TenantOptions      `cmd:"tenant"`
	Policies    PolicyOptions      `cmd:"policies"`
	Github      GithubOptions      `cmd:"github"`
	UIToken     UITokenOptions     `cmd:"ui-token"`
	Environment EnvironmentOptions `cmd:"environment"`
	Task        TaskOptions        `cmd:"task"`
	Turn        struct {
		Create  CreateTurnOptions  `cmd:"create"`
		List    ListTurnsOptions   `cmd:"list"`
		Update  UpdateTurnOptions  `cmd:"update"`
		Get     GetTurnOptions     `cmd:"get"`
		GetLast GetLastTurnOptions `cmd:"get-last"`
	} `cmd:"turn"`
	Logs struct {
		Stream StreamLogsOptions `cmd:"stream"`
		Upload UploadLogsOptions `cmd:"upload"`
	} `cmd:"logs"`
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

func dispatchCommand(kongctx *kong.Context, options *Options) error {
	switch kongctx.Command() {
	case "tenant create-user":
		return options.Tenant.CreateUser.Run(options.Ctx, &options.SharedOptions)
	case "tenant current-user":
		return options.Tenant.CurrentUser.Run(options.Ctx, &options.SharedOptions)
	case "tenant get":
		return options.Tenant.Get.Run(options.Ctx, &options.SharedOptions)
	case "policies list":
		return options.Policies.List.Run(options.Ctx, &options.SharedOptions)
	case "ui-token generate":
		return options.UIToken.Generate.Run(options.Ctx, &options.SharedOptions)
	case "github add-org":
		return options.Github.AddOrg.Run(options.Ctx, &options.SharedOptions)
	case "github list-orgs":
		return options.Github.ListOrgs.Run(options.Ctx, &options.SharedOptions)
	case "github get-org":
		return options.Github.GetOrg.Run(options.Ctx, &options.SharedOptions)
	case "github update-org":
		return options.Github.UpdateOrg.Run(options.Ctx, &options.SharedOptions)
	case "github delete-org":
		return options.Github.DeleteOrg.Run(options.Ctx, &options.SharedOptions)
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
