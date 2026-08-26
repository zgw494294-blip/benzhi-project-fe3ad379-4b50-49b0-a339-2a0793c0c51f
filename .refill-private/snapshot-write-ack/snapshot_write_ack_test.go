package snapshotwriteack_test

import (
	"os"
	"path/filepath"
	"testing"

	"sensor-calibration-release/internal/application/workflow"
	"sensor-calibration-release/internal/storage/jsonstore"
)

func TestSnapshotWriteFailureCannotBeAcknowledged(t *testing.T) {
	dir := t.TempDir()
	store, err := jsonstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(store)
	created, err := service.CreateBatch(workflow.CreateBatchCommand{
		CommandMeta: workflow.CommandMeta{Actor: "tech", ExpectedVersion: 0, IdempotencyKey: "batch-1"},
		StationCode: "ST-01",
		Title:       "快照故障复现",
	})
	if err != nil {
		t.Fatal(err)
	}

	snapshotPath := filepath.Join(dir, "snapshot.json")
	if err := os.Remove(snapshotPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(snapshotPath, 0700); err != nil {
		t.Fatal(err)
	}

	_, err = service.RegisterSensor(created.BatchID, workflow.RegisterSensorCommand{
		CommandMeta: workflow.CommandMeta{Actor: "tech", ExpectedVersion: created.Version, IdempotencyKey: "sensor-1"},
		SensorCode:  "S-01",
		Metric:      "temperature",
		Unit:        "C",
		RangeMin:    -20,
		RangeMax:    60,
	})
	if err == nil {
		t.Fatalf("快照写入失败时不应确认传感器登记成功")
	}
	snapshot, snapshotErr := store.Snapshot()
	if snapshotErr != nil {
		t.Fatal(snapshotErr)
	}
	if len(snapshot.Sensors) != 0 {
		t.Fatalf("失败提交不应发布传感器投影，实际数量=%d", len(snapshot.Sensors))
	}
	if got := len(store.Events("")); got != 1 {
		t.Fatalf("失败提交不应追加审计事件，实际数量=%d", got)
	}
	if err := os.Remove(snapshotPath); err != nil {
		t.Fatal(err)
	}
	reopened, err := jsonstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := reopened.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered.Sensors) != 0 {
		t.Fatalf("失败提交恢复后不应出现传感器，实际数量=%d", len(recovered.Sensors))
	}
}
