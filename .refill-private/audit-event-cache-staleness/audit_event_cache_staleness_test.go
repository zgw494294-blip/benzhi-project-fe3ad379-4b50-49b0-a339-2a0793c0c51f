package audit_event_cache_staleness_test

import (
	"testing"

	"sensor-calibration-release/internal/application/workflow"
	"sensor-calibration-release/internal/storage/jsonstore"
)

func TestAuditTrailCacheIncludesEventsCommittedAfterWarmup(t *testing.T) {
	store, err := jsonstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(store)
	created, err := service.CreateBatch(workflow.CreateBatchCommand{
		CommandMeta: workflow.CommandMeta{Actor: "tech", ExpectedVersion: 0, IdempotencyKey: "create"},
		StationCode: "ST-CACHE", Title: "缓存失效复现",
	})
	if err != nil {
		t.Fatal(err)
	}
	if events, err := service.AuditTrail(created.BatchID); err != nil {
		t.Fatal(err)
	} else if got := len(events); got != 1 {
		t.Fatalf("预热审计查询应看到首个事件，得到 %d", got)
	}
	if _, err := service.RegisterSensor(created.BatchID, workflow.RegisterSensorCommand{
		CommandMeta: workflow.CommandMeta{Actor: "tech", ExpectedVersion: created.Version, IdempotencyKey: "sensor"},
		SensorCode:  "S-CACHE", Metric: "temperature", Unit: "C", RangeMin: 0, RangeMax: 100,
	}); err != nil {
		t.Fatal(err)
	}
	events, err := service.AuditTrail(created.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(events); got != 2 {
		t.Fatalf("提交新事件后审计轨迹应包含两个事件，得到 %d", got)
	}
}
