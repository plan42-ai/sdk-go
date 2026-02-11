package p42

import "testing"

func TestUpdateWorkstreamTaskRequestIsEmptyUpdate(t *testing.T) {
	req := UpdateWorkstreamTaskRequest{}
	if !req.IsEmptyUpdate() {
		t.Fatalf("expected empty request to report IsEmptyUpdate")
	}

	title := "foo"
	req.Title = &title
	if req.IsEmptyUpdate() {
		t.Fatalf("expected request with Title to report non-empty")
	}
}

func TestUpdateWorkstreamTaskRequestGetVersion(t *testing.T) {
	req := UpdateWorkstreamTaskRequest{Version: 42}
	if got := req.GetVersion(); got != req.Version {
		t.Fatalf("expected GetVersion to return %d, got %d", req.Version, got)
	}
}
