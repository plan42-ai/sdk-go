package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/debugging-sucks/event-horizon-sdk-go/eh"
)

type TurnOptions struct {
	Create  CreateTurnOptions  `cmd:"create"`
	List    ListTurnsOptions   `cmd:"list"`
	Update  UpdateTurnOptions  `cmd:"update"`
	Get     GetTurnOptions     `cmd:"get"`
	GetLast GetLastTurnOptions `cmd:"get-last"`
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
