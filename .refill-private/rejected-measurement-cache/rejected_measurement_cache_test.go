package rejected_measurement_cache_test

import (
	"errors"
	"testing"

	"sensor-calibration-release/internal/application/workflow"
	"sensor-calibration-release/internal/domain/calibration"
	"sensor-calibration-release/internal/storage/jsonstore"
)

func TestRejectedMeasurementDoesNotPoisonStatisticsCache(t *testing.T) {
	store, err := jsonstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(store)

	created, err := service.CreateBatch(workflow.CreateBatchCommand{
		CommandMeta: meta("create", 0), StationCode: "ST-CACHE", Title: "缓存隔离复现",
	})
	if err != nil {
		t.Fatal(err)
	}
	registered, err := service.RegisterSensor(created.BatchID, workflow.RegisterSensorCommand{
		CommandMeta: meta("sensor", created.Version), SensorCode: "TEMP-01", Metric: "temperature", Unit: "C", RangeMin: 0, RangeMax: 200,
	})
	if err != nil {
		t.Fatal(err)
	}
	locked, err := service.LockProfile(created.BatchID, workflow.LockProfileCommand{
		CommandMeta: meta("profile", registered.Version), Points: []float64{0, 100}, RepetitionsPerPoint: 3,
		AbsoluteTolerance: 1, RelativeTolerance: 0.02, RepeatabilityLimit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.SubmitMeasurement(created.BatchID, workflow.SubmitMeasurementCommand{
		CommandMeta: meta("rejected", locked.Version), SensorRevisionID: registered.ID,
		ReferencePoint: 100, Readings: []float64{150, 150},
	})
	var domainErr *calibration.DomainError
	if !errors.As(err, &domainErr) || domainErr.Code != calibration.CodeValidation {
		t.Fatalf("前置请求应因重复读数不足被拒绝，实际错误为 %v", err)
	}

	accepted, err := service.SubmitMeasurement(created.BatchID, workflow.SubmitMeasurementCommand{
		CommandMeta: meta("accepted", locked.Version), SensorRevisionID: registered.ID,
		ReferencePoint: 100, Readings: []float64{100, 100, 100},
	})
	if err != nil {
		t.Fatalf("合法请求提交失败: %v", err)
	}
	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	set := snapshot.Measurements[accepted.ID]
	if set == nil {
		t.Fatalf("合法请求的读数集 %s 未持久化", accepted.ID)
	}
	findings := snapshot.BatchFindings(created.BatchID, true)
	if set.Mean != 100 || set.AbsoluteError != 0 || set.RelativeError != 0 || set.Spread != 0 || len(findings) != 0 {
		t.Fatalf("被拒绝请求污染了合法读数的统计结果: readings=%v mean=%v absoluteError=%v relativeError=%v spread=%v openFindings=%d", set.Readings, set.Mean, set.AbsoluteError, set.RelativeError, set.Spread, len(findings))
	}
}

func meta(key string, version int64) workflow.CommandMeta {
	return workflow.CommandMeta{Actor: "tech-a", ExpectedVersion: version, IdempotencyKey: key}
}
