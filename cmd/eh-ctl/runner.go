package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
	"github.com/google/uuid"
)

type RunnerOptions struct {
	Create        CreateRunnerOptions        `cmd:"" help:"Create a new remote runner."`
	List          ListRunnerOptions          `cmd:"" help:"List remote runners for a tenant."`
	ListQueues    ListRunnerQueuesOptions    `cmd:"" help:"List runner queues."`
	GetQueue      GetRunnerQueueOptions      `cmd:"" help:"Get metadata for a runner queue."`
	PingQueue     PingRunnerQueueOptions     `cmd:"" help:"Continuously ping a runner queue until interrupted."`
	Get           GetRunnerOptions           `cmd:"" help:"Get a remote runner by ID."`
	GetToken      GetRunnerTokenOptions      `cmd:"" help:"Get metadata for a remote runner token."`
	Delete        DeleteRunnerOptions        `cmd:"" help:"Soft delete a remote runner."`
	Update        UpdateRunnerOptions        `cmd:"" help:"Update a remote runner."`
	GenerateToken GenerateRunnerTokenOptions `cmd:"" help:"Generate a new auth token for a remote runner."`
	ListTokens    ListRunnerTokensOptions    `cmd:"" help:"List auth token metadata for a remote runner."`
	RevokeToken   RevokeRunnerTokenOptions   `cmd:"" help:"Revoke a remote runner auth token."`
}

type CreateRunnerOptions struct {
	TenantID string `help:"The tenant ID to create the runner under." name:"tenant-id" short:"i" required:""`
	JSON     string `help:"The JSON file containing the runner definition. Use '-' to read from stdin." short:"j" default:"-"`
}

func (o *CreateRunnerOptions) Run(ctx context.Context, s *SharedOptions) error {
	err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags)
	if err != nil {
		return err
	}
	var req eh.CreateRunnerRequest
	err = readJsonFile(o.JSON, &req)
	if err != nil {
		return err
	}
	err = loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	req.TenantID = o.TenantID
	if req.RunnerID == "" {
		req.RunnerID = uuid.NewString()
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	runner, err := s.Client.CreateRunner(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(runner)
}

type ListRunnerOptions struct {
	TenantID       string `help:"The tenant ID to list runners for." name:"tenant-id" short:"i" required:""`
	IncludeDeleted bool   `help:"When set, includes deleted runners in the results." short:"d" optional:""`
	RunsTasks      *bool  `help:"Optional. When set, filters runners based on whether they execute tasks." name:"runs-tasks" optional:""`
	ProxiesGithub  *bool  `help:"Optional. When set, filters runners based on whether they proxy GitHub access." name:"proxies-github" optional:""`
}

func (o *ListRunnerOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.ListRunnersRequest{
		TenantID:       o.TenantID,
		IncludeDeleted: pointer(o.IncludeDeleted),
		RunsTasks:      o.RunsTasks,
		ProxiesGithub:  o.ProxiesGithub,
	}

	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	for {
		resp, err := s.Client.ListRunners(ctx, req)
		if err != nil {
			return err
		}

		for _, runner := range resp.Items {
			if err := printJSON(runner); err != nil {
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

type ListRunnerQueuesOptions struct {
	TenantID   *string `help:"Optional. Filter queues for a tenant. Must be combined with --runner-id." name:"tenant-id" short:"i" optional:""`
	RunnerID   *string `help:"Optional. Filter queues for a runner. Must be combined with --tenant-id." name:"runner-id" short:"r" optional:""`
	Draining   *bool   `help:"Optional. Filter on draining status. Requires --tenant-id and --runner-id." name:"draining" optional:""`
	Healthy    *bool   `help:"Optional. Filter on healthy status. Requires --tenant-id and --runner-id." name:"healthy" optional:""`
	MinQueueID *string `help:"Optional inclusive minimum queue ID. Must be combined with --max-queue-id." name:"min-queue-id" optional:""`
	MaxQueueID *string `help:"Optional exclusive maximum queue ID. Must be combined with --min-queue-id." name:"max-queue-id" optional:""`
}

func (o *ListRunnerQueuesOptions) Run(ctx context.Context, s *SharedOptions) error {
	if (o.TenantID == nil) != (o.RunnerID == nil) {
		return errors.New("--tenant-id and --runner-id must be provided together")
	}
	if (o.MinQueueID == nil) != (o.MaxQueueID == nil) {
		return errors.New("--min-queue-id and --max-queue-id must be provided together")
	}
	if o.Draining != nil && o.TenantID == nil {
		return errors.New("--draining requires --tenant-id and --runner-id")
	}
	if o.Healthy != nil && o.TenantID == nil {
		return errors.New("--healthy requires --tenant-id and --runner-id")
	}

	req := &eh.ListRunnerQueuesRequest{
		TenantID:       o.TenantID,
		RunnerID:       o.RunnerID,
		IncludeDrained: o.Draining,
		IncludeHealthy: o.Healthy,
		MinQueueID:     o.MinQueueID,
		MaxQueueID:     o.MaxQueueID,
	}

	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	var token *string
	for {
		req.Token = token

		resp, err := s.Client.ListRunnerQueues(ctx, req)
		if err != nil {
			return err
		}

		for _, queue := range resp.Items {
			if err := printJSON(queue); err != nil {
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

type GetRunnerQueueOptions struct {
	TenantID string `help:"The tenant ID that owns the runner." name:"tenant-id" short:"i" required:""`
	RunnerID string `help:"The runner ID that owns the queue." name:"runner-id" short:"r" required:""`
	QueueID  string `help:"The queue ID to fetch." name:"queue-id" short:"q" required:""`
}

func (o *GetRunnerQueueOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.GetRunnerQueueRequest{
		TenantID: o.TenantID,
		RunnerID: o.RunnerID,
		QueueID:  o.QueueID,
	}

	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}

	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	queue, err := s.Client.GetRunnerQueue(ctx, req)
	if err != nil {
		return err
	}

	return printJSON(queue)
}

type PingRunnerQueueOptions struct {
	TenantID string `help:"The tenant ID that owns the runner." name:"tenant-id" short:"i" required:""`
	RunnerID string `help:"The runner ID that owns the queue." name:"runner-id" short:"r" required:""`
	QueueID  string `help:"The queue ID to ping." name:"queue-id" short:"q" required:""`
}

func (o *PingRunnerQueueOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.PingRunnerQueueRequest{
		TenantID: o.TenantID,
		RunnerID: o.RunnerID,
		QueueID:  o.QueueID,
	}

	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}

	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	loopCtx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	for {
		start := time.Now()
		err = s.Client.PingRunnerQueue(loopCtx, req)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		} else {
			fmt.Printf("latency: %s\n", time.Since(start))
		}

		select {
		case <-loopCtx.Done():
			return nil
		default:
		}
	}
}

type GetRunnerOptions struct {
	TenantID       string `help:"The tenant ID that owns the runner to fetch." name:"tenant-id" short:"i" required:""`
	RunnerID       string `help:"The runner ID to fetch." name:"runner-id" short:"r" required:""`
	IncludeDeleted bool   `help:"Set to return a deleted runner." name:"include-deleted" short:"d" optional:""`
}

func (o *GetRunnerOptions) Run(ctx context.Context, s *SharedOptions) error {
	req := &eh.GetRunnerRequest{
		TenantID:       o.TenantID,
		RunnerID:       o.RunnerID,
		IncludeDeleted: pointer(o.IncludeDeleted),
	}

	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}

	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	runner, err := s.Client.GetRunner(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(runner)
}

// DeleteRunnerOptions contains the flags for the `runner delete` command.
type DeleteRunnerOptions struct {
	TenantID string `help:"The tenant ID to connect to." name:"tenant-id" short:"i" required:""`
	RunnerID string `help:"The runner ID to delete." name:"runner-id" short:"r" required:""`
}

// Run executes the `runner delete` command.
func (o *DeleteRunnerOptions) Run(ctx context.Context, s *SharedOptions) error {
	getReq := &eh.GetRunnerRequest{
		TenantID: o.TenantID,
		RunnerID: o.RunnerID,
	}

	err := loadFeatureFlags(s, &getReq.FeatureFlags)
	if err != nil {
		return err
	}

	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)

	runner, err := s.Client.GetRunner(ctx, getReq)
	if err != nil {
		return err
	}

	delReq := &eh.DeleteRunnerRequest{
		TenantID: o.TenantID,
		RunnerID: o.RunnerID,
		Version:  runner.Version,
	}
	delReq.FeatureFlags = getReq.FeatureFlags
	processDelegatedAuth(s, &delReq.DelegatedAuthInfo)

	return s.Client.DeleteRunner(ctx, delReq)
}

type UpdateRunnerOptions struct {
	TenantID string `help:"The tenant ID to connect to." name:"tenant-id" short:"i" required:""`
	RunnerID string `help:"The runner ID to fetch." name:"runner-id" short:"r" required:""`
	JSON     string `help:"The json file containing the runner update. Use '-' to read from stdin." short:"j" default:"-"`
}

func (o *UpdateRunnerOptions) Run(ctx context.Context, s *SharedOptions) error {
	err := validateJSONFeatureFlags(o.JSON, s.FeatureFlags)
	if err != nil {
		return err
	}
	var req eh.UpdateRunnerRequest
	err = readJsonFile(o.JSON, &req)
	if err != nil {
		return err
	}
	err = loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	req.TenantID = o.TenantID
	req.RunnerID = o.RunnerID

	getReq := &eh.GetRunnerRequest{
		TenantID:       o.TenantID,
		RunnerID:       o.RunnerID,
		IncludeDeleted: pointer(true),
	}
	getReq.FeatureFlags = req.FeatureFlags
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)
	runner, err := s.Client.GetRunner(ctx, getReq)
	if err != nil {
		return err
	}
	req.Version = runner.Version
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	updated, err := s.Client.UpdateRunner(ctx, &req)
	if err != nil {
		return err
	}
	return printJSON(updated)
}

type GenerateRunnerTokenOptions struct {
	TenantID string `help:"The tenant ID that owns the runner." name:"tenant-id" short:"i" required:""`
	RunnerID string `help:"The runner ID to generate a token for." name:"runner-id" short:"r" required:""`
	TTLDays  *int   `help:"Optional token lifetime in days (1-365). Defaults to 90 days." name:"ttl-days" optional:""`
}

func (o *GenerateRunnerTokenOptions) Run(ctx context.Context, s *SharedOptions) error {
	if !s.ShowSecrets {
		return fmt.Errorf("you must specify `-s` when calling generate-token")
	}

	req := &eh.GenerateRunnerTokenRequest{
		TenantID: o.TenantID,
		RunnerID: o.RunnerID,
		TokenID:  uuid.NewString(),
		TTLDays:  o.TTLDays,
	}

	err := loadFeatureFlags(s, &req.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	resp, err := s.Client.GenerateRunnerToken(ctx, req)
	if err != nil {
		return err
	}

	return printJSON(resp)
}

type ListRunnerTokensOptions struct {
	TenantID       string `help:"The tenant ID to list tokens for." name:"tenant-id" short:"i" required:""`
	RunnerID       string `help:"The runner ID to list tokens for." name:"runner-id" short:"r" required:""`
	IncludeDeleted *bool  `help:"When set, includes deleted (revoked) tokens in the results." short:"d" optional:""`
}

func (o *ListRunnerTokensOptions) Run(ctx context.Context, s *SharedOptions) error {
	if s.ShowSecrets {
		return errors.New("invalid `-s`: runner token values cannot be fetched after they are generated")
	}

	req := &eh.ListRunnerTokensRequest{
		TenantID:       o.TenantID,
		RunnerID:       o.RunnerID,
		IncludeRevoked: o.IncludeDeleted,
	}

	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return err
	}

	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	for {
		resp, err := s.Client.ListRunnerTokens(ctx, req)
		if err != nil {
			return err
		}

		for _, token := range resp.Items {
			if err := printJSON(token); err != nil {
				return err
			}
		}

		if resp.NextToken == nil {
			break
		}

		req.NextPageToken = resp.NextToken
	}

	return nil
}

type GetRunnerTokenOptions struct {
	TenantID       string `help:"The tenant ID that owns the runner." name:"tenant-id" short:"i" required:""`
	RunnerID       string `help:"The runner ID that owns the token." name:"runner-id" short:"r" required:""`
	TokenID        string `help:"The token ID to fetch metadata for." name:"token-id" short:"k" required:""`
	IncludeDeleted bool   `help:"Set to return metadata for a revoked token." name:"include-deleted" short:"d" optional:""`
}

func (o *GetRunnerTokenOptions) Run(ctx context.Context, s *SharedOptions) error {
	if s.ShowSecrets {
		return errors.New("invalid `-s`: runner token values cannot be fetched after they are generated")
	}

	req := &eh.GetRunnerTokenRequest{
		TenantID:       o.TenantID,
		RunnerID:       o.RunnerID,
		TokenID:        o.TokenID,
		IncludeDeleted: pointer(o.IncludeDeleted),
	}

	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return err
	}

	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	token, err := s.Client.GetRunnerToken(ctx, req)
	if err != nil {
		return err
	}

	return printJSON(token)
}

type RevokeRunnerTokenOptions struct {
	TenantID string `help:"The tenant to revoke the token for." name:"tenant-id" short:"i" required:""`
	RunnerID string `help:"The runner to revoke the token for." name:"runner-id" short:"r" required:""`
	TokenID  string `help:"The id of the token to revoke." name:"token-id" short:"k" required:""`
}

func (o *RevokeRunnerTokenOptions) Run(ctx context.Context, s *SharedOptions) error {
	getReq := &eh.GetRunnerTokenRequest{
		TenantID:       o.TenantID,
		RunnerID:       o.RunnerID,
		TokenID:        o.TokenID,
		IncludeDeleted: pointer(true),
	}

	err := loadFeatureFlags(s, &getReq.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &getReq.DelegatedAuthInfo)

	token, err := s.Client.GetRunnerToken(ctx, getReq)
	if err != nil {
		return err
	}

	req := &eh.RevokeRunnerTokenRequest{
		TenantID: o.TenantID,
		RunnerID: o.RunnerID,
		TokenID:  o.TokenID,
		Version:  token.Version,
	}
	req.FeatureFlags = getReq.FeatureFlags
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	err = s.Client.RevokeRunnerToken(ctx, req)
	if err != nil {
		return err
	}

	fmt.Printf("Token %s revoked.\n", o.TokenID)
	return nil
}
