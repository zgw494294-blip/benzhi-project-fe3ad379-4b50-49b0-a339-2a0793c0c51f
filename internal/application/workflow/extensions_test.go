package workflow_test

import (
	"errors"
	"testing"

	"sensor-calibration-release/internal/application/workflow"
	"sensor-calibration-release/internal/domain/calibration"
	"sensor-calibration-release/internal/storage/jsonstore"
)

func TestExtendedCalibrationWorkflow(t *testing.T) {
	store, err := jsonstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(store)
	created, err := service.CreateBatch(workflow.CreateBatchCommand{CommandMeta: meta("tech-a", 0, "create"), StationCode: "ST-EXT", Title: "多点校准"})
	if err != nil {
		t.Fatal(err)
	}
	registered, err := service.RegisterSensor(created.BatchID, workflow.RegisterSensorCommand{CommandMeta: meta("tech-a", created.Version, "sensor"), SensorCode: "S-01", Metric: "temperature", Unit: "C", RangeMin: 0, RangeMax: 100})
	if err != nil {
		t.Fatal(err)
	}
	locked, err := service.LockProfile(created.BatchID, workflow.LockProfileCommand{CommandMeta: meta("tech-a", registered.Version, "profile"), Points: []float64{0, 50, 100}, RepetitionsPerPoint: 3, AbsoluteTolerance: 1, RelativeTolerance: .02, RepeatabilityLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	invalid := workflow.SubmitMeasurementBatchCommand{CommandMeta: meta("tech-a", locked.Version, "batch-measurements"), SensorRevisionID: registered.ID, Measurements: []workflow.MeasurementInput{{ReferencePoint: 0, Readings: []float64{0, .1, .2}}, {ReferencePoint: 50, Readings: []float64{49.9, 50, 50.1}}, {ReferencePoint: 100, Readings: []float64{99.9, 100, 101}}}}
	if _, err := service.SubmitMeasurementBatch(created.BatchID, invalid); err == nil {
		t.Fatal("包含越界读数的批量请求应整体失败")
	}
	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Measurements) != 0 || snapshot.Batches[created.BatchID].Version != locked.Version {
		t.Fatalf("失败批量请求不应落库或推进版本: measurements=%d version=%d", len(snapshot.Measurements), snapshot.Batches[created.BatchID].Version)
	}
	invalid.Measurements[2].Readings = []float64{99.8, 100, 100}
	submitted, err := service.SubmitMeasurementBatch(created.BatchID, invalid)
	if err != nil {
		t.Fatal(err)
	}
	if submitted.Version != locked.Version+1 || submitted.Status != string(calibration.StatusReadyReview) || len(submitted.Measurements) != 3 {
		t.Fatalf("批量提交结果不正确: %+v", submitted)
	}
	replayed, err := service.SubmitMeasurementBatch(created.BatchID, invalid)
	if err != nil || !replayed.Replayed || replayed.Measurements[0].ID != submitted.Measurements[0].ID {
		t.Fatalf("批量提交未返回原始幂等结果: %+v %v", replayed, err)
	}
	changed := invalid
	changed.Measurements = append([]workflow.MeasurementInput(nil), invalid.Measurements...)
	changed.Measurements[0].Readings = []float64{.2, .2, .2}
	if _, err := service.SubmitMeasurementBatch(created.BatchID, changed); !isCode(err, calibration.CodeConflict) {
		t.Fatalf("同一幂等键用于不同内容应冲突，实际 %v", err)
	}

	returned, err := service.Review(created.BatchID, workflow.ReviewCommand{CommandMeta: meta("reviewer-b", submitted.Version, "return"), Decision: "return", Comment: "补充现场复核证据", Corrections: []workflow.CorrectionInput{{SensorRevisionID: registered.ID, ReferencePoint: 0, ProblemType: "traceability", Severity: "major", Description: "补充零点溯源复验"}, {SensorRevisionID: registered.ID, ReferencePoint: 100, ProblemType: "stability", Severity: "major", Description: "补充满量程稳定性复验"}}})
	if err != nil || returned.Status != string(calibration.StatusReturned) || len(returned.FindingIDs) != 2 {
		t.Fatalf("结构化退回失败: %+v %v", returned, err)
	}
	revision, err := service.Recalibrate(created.BatchID, workflow.RecalibrateCommand{CommandMeta: meta("tech-a", returned.Version, "recalibrate"), SensorRevisionID: registered.ID, Note: "按补正项完成返校"})
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := service.RecalibrationTasks(created.BatchID, revision.ID)
	if err != nil || tasks.RequiredPoints != 2 || tasks.InheritedPoints != 1 || tasks.PendingPoints != 2 {
		t.Fatalf("返校任务覆盖不正确: %+v %v", tasks, err)
	}
	first, err := service.SubmitMeasurement(created.BatchID, workflow.SubmitMeasurementCommand{CommandMeta: meta("tech-a", revision.Version, "retest-zero"), SensorRevisionID: revision.ID, ReferencePoint: 0, Readings: []float64{0, .1, .2}})
	if err != nil || first.PendingRetestPoints != 1 {
		t.Fatalf("首个复验点提交失败: %+v %v", first, err)
	}
	if _, err := service.ResubmitReview(created.BatchID, workflow.ResubmitReviewCommand{CommandMeta: meta("tech-a", first.Version, "resubmit-too-early")}); !isCode(err, calibration.CodeValidation) {
		t.Fatalf("未闭环全部补正时应阻止再次送审，实际 %v", err)
	}
	second, err := service.SubmitMeasurement(created.BatchID, workflow.SubmitMeasurementCommand{CommandMeta: meta("tech-a", first.Version, "retest-full"), SensorRevisionID: revision.ID, ReferencePoint: 100, Readings: []float64{99.8, 100, 100}})
	if err != nil || second.PendingRetestPoints != 0 || second.Status != string(calibration.StatusReturned) {
		t.Fatalf("全部复验点提交失败: %+v %v", second, err)
	}
	resubmitted, err := service.ResubmitReview(created.BatchID, workflow.ResubmitReviewCommand{CommandMeta: meta("tech-a", second.Version, "resubmit"), Comment: "补正已完成"})
	if err != nil || resubmitted.Status != string(calibration.StatusReadyReview) {
		t.Fatalf("再次送审失败: %+v %v", resubmitted, err)
	}
	if _, err := service.Review(created.BatchID, workflow.ReviewCommand{CommandMeta: meta("tech-a", resubmitted.Version, "self-review"), Decision: "approve"}); !isCode(err, calibration.CodeForbidden) {
		t.Fatalf("采样人通过复核应被职责分离规则拒绝，实际 %v", err)
	}
	approved, err := service.Review(created.BatchID, workflow.ReviewCommand{CommandMeta: meta("reviewer-b", resubmitted.Version, "approve"), Decision: "approve"})
	if err != nil {
		t.Fatal(err)
	}
	issued, err := service.Issue(created.BatchID, workflow.IssueCommand{CommandMeta: meta("deployer-c", approved.Version, "issue")})
	if err != nil {
		t.Fatal(err)
	}
	credential, err := service.GetCredential(created.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	beforeEvents := len(store.Events(""))
	verified, err := service.VerifyCredential(created.BatchID, issued.ID, credential.ContentDigest)
	if err != nil || !verified.Valid || len(verified.Devices) != 1 {
		t.Fatalf("真实凭据核验失败: %+v %v", verified, err)
	}
	after, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if after.Batches[created.BatchID].Version != issued.Version || len(store.Events("")) != beforeEvents {
		t.Fatal("凭据核验必须保持批次版本和审计事件数量不变")
	}
	mismatch, err := service.VerifyCredential(created.BatchID, issued.ID, "0"+credential.ContentDigest[1:])
	if err != nil || mismatch.Valid || mismatch.Failures[0].Code != "content_digest_mismatch" {
		t.Fatalf("摘要不匹配结果不正确: %+v %v", mismatch, err)
	}

	other, err := service.CreateBatch(workflow.CreateBatchCommand{CommandMeta: meta("tech-a", 0, "other"), StationCode: "ST-EXT", Title: "后续批次"})
	if err != nil {
		t.Fatal(err)
	}
	cross, err := service.VerifyCredential(other.BatchID, issued.ID, credential.ContentDigest)
	if err != nil || cross.Valid || cross.Failures[0].Code != "credential_batch_mismatch" {
		t.Fatalf("跨批次凭据结果不正确: %+v %v", cross, err)
	}
	queue, err := service.ListBatches(workflow.BatchQueueFilter{StationCode: "ST-EXT", Limit: 1})
	if err != nil || len(queue.Items) != 1 || queue.NextCursor == "" || queue.Summary.Total != 2 || queue.Summary.ByStatus[string(calibration.StatusReleased)] != 1 || queue.Summary.ByStatus[string(calibration.StatusDraft)] != 1 {
		t.Fatalf("工作队列分页或汇总不正确: %+v %v", queue, err)
	}
	next, err := service.ListBatches(workflow.BatchQueueFilter{StationCode: "ST-EXT", Limit: 1, Cursor: queue.NextCursor})
	if err != nil || len(next.Items) != 1 || next.Items[0].Batch.ID == queue.Items[0].Batch.ID {
		t.Fatalf("工作队列稳定游标不正确: %+v %v", next, err)
	}
}

func meta(actor string, version int64, key string) workflow.CommandMeta {
	return workflow.CommandMeta{Actor: actor, ExpectedVersion: version, IdempotencyKey: key}
}

func isCode(err error, code calibration.ErrorCode) bool {
	var domainErr *calibration.DomainError
	return errors.As(err, &domainErr) && domainErr.Code == code
}
