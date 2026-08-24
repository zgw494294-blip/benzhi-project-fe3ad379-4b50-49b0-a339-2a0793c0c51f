package httpapi

import (
	"context"
	"testing"
	"time"

	"sensor-calibration-release/internal/application/workflow"
	"sensor-calibration-release/internal/storage/jsonstore"
)

func TestRunSelfcheckOverRealListener(t *testing.T) {
	store, err := jsonstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(store)
	listener, err := Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(New(service).Handler())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := RunSelfcheck(ctx, listener, server); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Batches) != 1 || len(snapshot.Credentials) != 1 {
		t.Fatalf("selfcheck 未完成放行: batches=%d credentials=%d", len(snapshot.Batches), len(snapshot.Credentials))
	}
	for _, batch := range snapshot.Batches {
		if batch.Status != "released" {
			t.Fatalf("最终状态不是 released: %s", batch.Status)
		}
	}
}
