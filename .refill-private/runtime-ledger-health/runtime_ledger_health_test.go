package runtimeledgerhealth

import (
	"os"
	"path/filepath"
	"testing"

	"sensor-calibration-release/internal/application/workflow"
	"sensor-calibration-release/internal/storage/jsonstore"
)

func TestVerifyReadsCurrentLedgerState(t *testing.T) {
	dir := t.TempDir()
	store, err := jsonstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = workflow.New(store).CreateBatch(workflow.CreateBatchCommand{
		CommandMeta: workflow.CommandMeta{Actor: "tech", IdempotencyKey: "create", ExpectedVersion: 0},
		StationCode: "ST-01",
		Title:       "运行期完整性检查",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte("corrupted\n"), 0640); err != nil {
		t.Fatal(err)
	}

	if report, err := store.Verify(); err == nil && report.Valid {
		t.Fatalf("TestVerifyReadsCurrentLedgerState: 磁盘账本损坏后仍报告 valid=true: %+v", report)
	}
}
