package commitappendrollback_test

import (
	"os"
	"path/filepath"
	"testing"

	"sensor-calibration-release/internal/application/workflow"
	"sensor-calibration-release/internal/storage/jsonstore"
)

func TestFailedAppendDoesNotMutateProjection(t *testing.T) {
	dir := t.TempDir()
	store, err := jsonstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(store)
	created, err := service.CreateBatch(workflow.CreateBatchCommand{
		CommandMeta: workflow.CommandMeta{Actor: "tech-a", ExpectedVersion: 0, IdempotencyKey: "create"},
		StationCode: "ST-ROLLBACK",
		Title:       "账本追加失败回滚",
	})
	if err != nil {
		t.Fatal(err)
	}

	ledgerPath := filepath.Join(dir, "events.jsonl")
	if err := os.Remove(ledgerPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(ledgerPath, 0750); err != nil {
		t.Fatal(err)
	}

	_, err = service.RegisterSensor(created.BatchID, workflow.RegisterSensorCommand{
		CommandMeta: workflow.CommandMeta{Actor: "tech-a", ExpectedVersion: created.Version, IdempotencyKey: "sensor"},
		SensorCode:  "S-ROLLBACK",
		Metric:     "temperature",
		Unit:       "C",
		RangeMin:   -20,
		RangeMax:   60,
	})
	if err == nil {
		t.Fatal("账本追加资源失效时请求应失败")
	}

	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Sensors) != 0 || snapshot.Batches[created.BatchID].Version != created.Version {
		t.Fatalf("追加失败后投影被污染: sensors=%d version=%d", len(snapshot.Sensors), snapshot.Batches[created.BatchID].Version)
	}
}
