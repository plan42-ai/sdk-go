package p42_test

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/plan42-ai/sdk-go/p42"
	"github.com/stretchr/testify/require"
)

type fakeUploadClient struct {
	mu   sync.Mutex
	reqs []*p42.UploadTurnLogsRequest
}

func (f *fakeUploadClient) UploadTurnLogs(
	_ context.Context,
	req *p42.UploadTurnLogsRequest,
) (*p42.UploadTurnLogsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *req
	cp.Logs = append([]p42.TurnLog(nil), req.Logs...)
	f.reqs = append(f.reqs, &cp)
	return &p42.UploadTurnLogsResponse{Version: req.Version + 1}, nil
}

func TestLogUploaderDefaultConfig(t *testing.T) {
	logs := make(chan p42.TurnLog)
	fake := &fakeUploadClient{}
	lu := p42.NewLogUploader(
		&p42.LogUploaderConfig{
			Client:     fake,
			TenantID:   "t",
			TaskID:     "task",
			TurnIndex:  0,
			Version:    1,
			StartIndex: 0,
			Logs:       logs,
		},
	)
	v := reflect.ValueOf(lu).Elem()
	if v.FieldByName("maxBatchLen").Int() != 500 {
		t.Fatalf("default maxBatchLen wrong")
	}
	if v.FieldByName("maxBatchAge").Int() != int64(time.Second) {
		t.Fatalf("default maxBatchAge wrong")
	}
	if v.FieldByName("maxBatchBytes").Int() != 1048576 {
		t.Fatalf("default maxBatchBytes wrong")
	}
	lu.Close()
}

func TestLogUploaderBatchLen(t *testing.T) {
	logs := make(chan p42.TurnLog)
	fake := &fakeUploadClient{}
	cfg := &p42.LogUploaderConfig{
		Client:      fake,
		TenantID:    "t",
		TaskID:      "task",
		TurnIndex:   0,
		Version:     1,
		StartIndex:  0,
		Logs:        logs,
		MaxBatchLen: 2,
	}
	lu := p42.NewLogUploader(cfg)

	logs <- p42.TurnLog{Message: "a"}
	logs <- p42.TurnLog{Message: "b"}
	logs <- p42.TurnLog{Message: "c"}
	close(logs)

	if err := lu.ShutdownTimeout(2 * time.Second); err != nil {
		t.Fatal(err)
	}

	if len(fake.reqs) != 2 {
		t.Fatalf("expected 2 reqs, got %d", len(fake.reqs))
	}
	if len(fake.reqs[0].Logs) != 2 || len(fake.reqs[1].Logs) != 1 {
		t.Fatalf("bad batch lens")
	}
}

func TestLogUploaderBatchAge(t *testing.T) {
	logs := make(chan p42.TurnLog)
	fake := &fakeUploadClient{}
	cfg := &p42.LogUploaderConfig{
		Client:      fake,
		TenantID:    "t",
		TaskID:      "task",
		TurnIndex:   0,
		Version:     1,
		StartIndex:  0,
		Logs:        logs,
		MaxBatchAge: 50 * time.Millisecond,
	}
	lu := p42.NewLogUploader(cfg)

	logs <- p42.TurnLog{Message: "a"}
	logs <- p42.TurnLog{Message: "b"}
	time.Sleep(60 * time.Millisecond)

	if len(fake.reqs) != 1 || len(fake.reqs[0].Logs) != 2 {
		t.Fatalf("batch age flush failed")
	}
	close(logs)
	err := lu.ShutdownTimeout(time.Second)
	require.NoError(t, err)
}

func TestLogUploaderBatchSize(t *testing.T) {
	logs := make(chan p42.TurnLog)
	fake := &fakeUploadClient{}
	cfg := &p42.LogUploaderConfig{
		Client:      fake,
		TenantID:    "t",
		TaskID:      "task",
		TurnIndex:   0,
		Version:     1,
		StartIndex:  0,
		Logs:        logs,
		MaxBatchLen: 1000,
	}
	lu := p42.NewLogUploader(cfg)

	long := make([]byte, 600000)
	for i := range long {
		long[i] = 'a'
	}
	logs <- p42.TurnLog{Message: string(long)}
	logs <- p42.TurnLog{Message: string(long)}
	logs <- p42.TurnLog{Message: string(long)}
	close(logs)

	if err := lu.ShutdownTimeout(2 * time.Second); err != nil {
		t.Fatal(err)
	}

	if len(fake.reqs) != 3 {
		t.Fatalf("expected 3 reqs, got %d", len(fake.reqs))
	}
}
