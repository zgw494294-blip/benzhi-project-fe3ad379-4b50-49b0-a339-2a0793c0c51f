package rotatedledgerhandle_test

import (
	"os"
	"path/filepath"
	"testing"

	"sensor-calibration-release/internal/application/workflow"
	"sensor-calibration-release/internal/storage/jsonstore"
)

func TestLedgerRotationDoesNotLoseCommittedEvent(t *testing.T) {
	dir := t.TempDir()
	store, err := jsonstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(store)
	created, err := service.CreateBatch(workflow.CreateBatchCommand{
		CommandMeta: workflow.CommandMeta{Actor: "tech-a", ExpectedVersion: 0, IdempotencyKey: "create"},
		StationCode: "CN-SH-017",
		Title:       "轮换恢复验证",
	})
	if err != nil {
		t.Fatal(err)
	}

	ledgerPath := filepath.Join(dir, "events.jsonl")
	firstEvent, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(ledgerPath, filepath.Join(dir, "events.jsonl.1")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledgerPath, firstEvent, 0640); err != nil {
		t.Fatal(err)
	}

	registered, err := service.RegisterSensor(created.BatchID, workflow.RegisterSensorCommand{
		CommandMeta: workflow.CommandMeta{Actor: "tech-a", ExpectedVersion: created.Version, IdempotencyKey: "sensor"},
		SensorCode:  "PM25-A01",
		Metric:      "PM2.5",
		Unit:        "ug/m3",
		RangeMin:    0,
		RangeMax:    500,
	})
	if err != nil {
		t.Fatalf("轮换后的提交应成功: %v", err)
	}

	reopened, err := jsonstore.Open(dir)
	if err != nil {
		t.Fatalf("重新打开存储失败: %v", err)
	}
	snapshot, err := reopened.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Sensors[registered.ID] == nil {
		t.Fatalf("提交已返回成功，但重启后传感器 %s 丢失", registered.ID)
	}
}
