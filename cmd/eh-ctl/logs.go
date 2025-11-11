package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
)

type LogsOptions struct {
	Stream StreamLogsOptions `cmd:"" help:"Stream logs for a given turn."`
	Upload UploadLogsOptions `cmd:"" help:"Upload a batch of logs for a given turn."`
}

type StreamLogsOptions struct {
	TenantID       string `help:"The id of the tenant that owns the task / turn to stream logs for." name:"tenant-id" short:"i" required:""`
	TaskID         string `help:"The id of the task to stream logs for." name:"task-id" short:"t" required:""`
	TurnIndex      int    `help:"The turn to stream logs for." name:"turn-index" short:"n" required:""`
	IncludeDeleted bool   `help:"Include logs for turns on deleted tasks" short:"d"`
}

func (o *StreamLogsOptions) Run(_ context.Context, s *SharedOptions) error {
	// TODO: Modify this so that NewLogStream uses the passed in context.
	var flags eh.FeatureFlags
	err := loadFeatureFlags(s, &flags)
	if err != nil {
		return err
	}
	ls := eh.NewLogStream(s.Client, o.TenantID, o.TaskID, o.TurnIndex, 1000, eh.WithIncludeDeleted(o.IncludeDeleted), eh.WithFeatureFlags(flags.FeatureFlags))
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

	getTurnReq := &eh.GetTurnRequest{TenantID: o.TenantID, TaskID: o.TaskID, TurnIndex: o.TurnIndex}
	err := loadFeatureFlags(s, &getTurnReq.FeatureFlags)
	if err != nil {
		return err
	}
	processDelegatedAuth(s, &getTurnReq.DelegatedAuthInfo)
	turn, err := s.Client.GetTurn(ctx, getTurnReq)
	if err != nil {
		return err
	}

	lastReq := &eh.GetLastTurnLogRequest{TenantID: o.TenantID, TaskID: o.TaskID, TurnIndex: o.TurnIndex}
	lastReq.FeatureFlags = getTurnReq.FeatureFlags
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
		Client:       s.Client,
		TenantID:     o.TenantID,
		TaskID:       o.TaskID,
		TurnIndex:    o.TurnIndex,
		Version:      turn.Version,
		StartIndex:   last.Index + 1,
		Logs:         logsCh,
		FeatureFlags: getTurnReq.FeatureFlags.FeatureFlags,
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
