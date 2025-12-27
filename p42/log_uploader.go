package p42

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/plan42-ai/concurrency"
)

// LogUploaderConfig holds configuration for LogUploader.
type LogUploaderConfig struct {
	Client     LogUploaderClient
	TenantID   string
	TaskID     string
	TurnIndex  int
	Version    int
	StartIndex int
	Logs       <-chan TurnLog

	FeatureFlags map[string]bool

	MaxBatchLen   int
	MaxBatchAge   time.Duration
	MaxBatchBytes int
}

// LogUploaderClient abstracts the Client method used by LogUploader.
type LogUploaderClient interface {
	UploadTurnLogs(ctx context.Context, req *UploadTurnLogsRequest) (*UploadTurnLogsResponse, error)
}

// LogUploader batches logs from a channel and uploads them using the API.
type LogUploader struct {
	cg *concurrency.ContextGroup

	client    LogUploaderClient
	tenantID  string
	taskID    string
	turnIndex int
	version   int
	index     int

	logs <-chan TurnLog

	featureFlags map[string]bool

	maxBatchLen   int
	maxBatchAge   time.Duration
	maxBatchBytes int

	batch      []TurnLog
	batchBytes int
	timer      *time.Timer
}

const (
	defaultMaxBatchLen   = 500
	defaultMaxBatchAge   = time.Second
	defaultMaxBatchBytes = 1_048_576
)

var perLogOverhead = func() int {
	// Estimate the JSON overhead for a log entry when encoded in a request.
	// Use the size of an empty request with two log entries as an upper
	// bound for the overhead of a single log. This is conservative but
	// avoids needing to account for other request fields.
	req := UploadTurnLogsRequest{Logs: []TurnLog{{}, {}}}
	b, _ := json.Marshal(req)
	return len(b)
}()

// NewLogUploader creates and starts a LogUploader.
func NewLogUploader(cfg *LogUploaderConfig) *LogUploader {
	if cfg == nil {
		cfg = &LogUploaderConfig{}
	}
	if cfg.MaxBatchLen == 0 {
		cfg.MaxBatchLen = defaultMaxBatchLen
	}
	if cfg.MaxBatchAge == 0 {
		cfg.MaxBatchAge = defaultMaxBatchAge
	}
	if cfg.MaxBatchBytes == 0 {
		cfg.MaxBatchBytes = defaultMaxBatchBytes
	}

	lu := &LogUploader{
		cg:            concurrency.NewContextGroup(),
		client:        cfg.Client,
		tenantID:      cfg.TenantID,
		taskID:        cfg.TaskID,
		turnIndex:     cfg.TurnIndex,
		version:       cfg.Version,
		index:         cfg.StartIndex,
		logs:          cfg.Logs,
		featureFlags:  cfg.FeatureFlags,
		maxBatchLen:   cfg.MaxBatchLen,
		maxBatchAge:   cfg.MaxBatchAge,
		maxBatchBytes: cfg.MaxBatchBytes,
	}
	lu.timer = time.NewTimer(cfg.MaxBatchAge)
	lu.timer.Stop()

	lu.cg.Add(1)
	go lu.run()
	return lu
}

// Close cancels the uploader and waits for shutdown.
func (l *LogUploader) Close() error { return l.cg.Close() }

// ShutdownContext waits for shutdown with a context.
func (l *LogUploader) ShutdownContext(ctx context.Context) error { return l.cg.WaitContext(ctx) }

// ShutdownTimeout waits for shutdown with a timeout.
func (l *LogUploader) ShutdownTimeout(d time.Duration) error { return l.cg.WaitTimeout(d) }

func (l *LogUploader) run() {
	defer l.cg.Done()
	defer l.cg.Cancel()

	for {
		select {
		case <-l.cg.Context().Done():
			l.flush()
			return
		case <-l.timer.C:
			if len(l.batch) > 0 {
				l.flush()
			}
		case logEntry, ok := <-l.logs:
			if !ok {
				l.flush()
				return
			}
			msgLen := len(logEntry.Message)
			if msgLen > l.maxBatchBytes-perLogOverhead {
				logEntry.Message = logEntry.Message[:l.maxBatchBytes-perLogOverhead]
				msgLen = len(logEntry.Message)
			}
			entrySize := msgLen + perLogOverhead
			if len(l.batch)+1 > l.maxBatchLen || l.batchBytes+entrySize > l.maxBatchBytes {
				l.flush()
			}
			if len(l.batch) == 0 {
				l.setTimer()
			}
			l.batch = append(l.batch, logEntry)
			l.batchBytes += entrySize
			if len(l.batch) >= l.maxBatchLen || l.batchBytes >= l.maxBatchBytes {
				l.flush()
			}
		}
	}
}

func (l *LogUploader) flush() {
	if len(l.batch) == 0 {
		return
	}
	resp, err := l.client.UploadTurnLogs(
		l.cg.Context(), &UploadTurnLogsRequest{
			TenantID:     l.tenantID,
			TaskID:       l.taskID,
			TurnIndex:    l.turnIndex,
			Version:      l.version,
			Index:        l.index,
			Logs:         l.batch,
			FeatureFlags: FeatureFlags{FeatureFlags: l.featureFlags},
		},
	)
	if err != nil {
		var conflictErr *ConflictError
		if errors.As(err, &conflictErr) {
			slog.ErrorContext(l.cg.Context(), "LogUploader: conflict", "error", err)
			l.cg.Cancel()
		} else {
			slog.ErrorContext(l.cg.Context(), "LogUploader: upload error", "error", err)
		}
	} else {
		l.index += len(l.batch)
		l.version = resp.Version
	}
	l.batch = nil
	l.batchBytes = 0
	l.timer.Stop()
}

func (l *LogUploader) setTimer() {
	l.timer.Stop()
	l.timer.Reset(l.maxBatchAge)
}
