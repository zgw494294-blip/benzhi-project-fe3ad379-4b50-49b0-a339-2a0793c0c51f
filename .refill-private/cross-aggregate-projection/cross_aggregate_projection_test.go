package crossaggregateprojection

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"sensor-calibration-release/internal/application/workflow"
	"sensor-calibration-release/internal/audit/ledger"
	"sensor-calibration-release/internal/domain/calibration"
	"sensor-calibration-release/internal/storage/jsonstore"
)

func TestEventCannotMutateAnotherAggregate(t *testing.T) {
	dir := t.TempDir()
	store, err := jsonstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(store)
	first, err := service.CreateBatch(workflow.CreateBatchCommand{
		CommandMeta: workflow.CommandMeta{Actor: "tech", IdempotencyKey: "first", ExpectedVersion: 0},
		StationCode: "ST-01",
		Title:       "第一批次",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateBatch(workflow.CreateBatchCommand{
		CommandMeta: workflow.CommandMeta{Actor: "tech", IdempotencyKey: "second", ExpectedVersion: 0},
		StationCode: "ST-02",
		Title:       "第二批次",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.RegisterSensor(first.BatchID, workflow.RegisterSensorCommand{
		CommandMeta: workflow.CommandMeta{Actor: "tech", IdempotencyKey: "sensor", ExpectedVersion: first.Version},
		SensorCode:  "S-01",
		Metric:      "temperature",
		Unit:        "C",
		RangeMin:    0,
		RangeMax:    100,
	})
	if err != nil {
		t.Fatal(err)
	}

	events := store.Events("")
	last := &events[len(events)-1]
	projection := calibration.NewSnapshot()
	if err := json.Unmarshal(last.Projection, projection); err != nil {
		t.Fatal(err)
	}
	projection.Batches[second.BatchID].Title = "由第一批次事件偷偷改写"
	last.Projection, err = json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Seal(last); err != nil {
		t.Fatal(err)
	}
	ledgerFile, err := os.Create(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(ledgerFile)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			_ = ledgerFile.Close()
			t.Fatal(err)
		}
	}
	if err := ledgerFile.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "snapshot.json")); err != nil {
		t.Fatal(err)
	}

	if _, err := jsonstore.Open(dir); err == nil {
		t.Fatal("TestEventCannotMutateAnotherAggregate: 审计校验接受了修改非目标聚合的事件投影")
	}
}
