package concurrentcredentialcache_test

import (
	"sync"
	"testing"

	"sensor-calibration-release/internal/application/workflow"
	"sensor-calibration-release/internal/storage/jsonstore"
)

func TestConcurrentCredentialVerificationIsRaceFree(t *testing.T) {
	store, err := jsonstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(store)
	created, err := service.CreateBatch(workflow.CreateBatchCommand{
		CommandMeta: workflow.CommandMeta{Actor: "tech-a", IdempotencyKey: "create"},
		StationCode: "ST-RACE", Title: "并发核验缓存复现",
	})
	if err != nil {
		t.Fatal(err)
	}
	registered, err := service.RegisterSensor(created.BatchID, workflow.RegisterSensorCommand{
		CommandMeta: workflow.CommandMeta{Actor: "tech-a", ExpectedVersion: created.Version, IdempotencyKey: "sensor"},
		SensorCode:  "S-RACE", Metric: "temperature", Unit: "C", RangeMin: 0, RangeMax: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	locked, err := service.LockProfile(created.BatchID, workflow.LockProfileCommand{
		CommandMeta: workflow.CommandMeta{Actor: "tech-a", ExpectedVersion: registered.Version, IdempotencyKey: "profile"},
		Points:      []float64{0, 50}, RepetitionsPerPoint: 3, AbsoluteTolerance: 1,
		RelativeTolerance: 0.02, RepeatabilityLimit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	submitted, err := service.SubmitMeasurementBatch(created.BatchID, workflow.SubmitMeasurementBatchCommand{
		CommandMeta:      workflow.CommandMeta{Actor: "tech-a", ExpectedVersion: locked.Version, IdempotencyKey: "measurement"},
		SensorRevisionID: registered.ID,
		Measurements: []workflow.MeasurementInput{
			{ReferencePoint: 0, Readings: []float64{0, 0.1, 0.2}},
			{ReferencePoint: 50, Readings: []float64{49.9, 50, 50.1}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	approved, err := service.Review(created.BatchID, workflow.ReviewCommand{
		CommandMeta: workflow.CommandMeta{Actor: "reviewer-b", ExpectedVersion: submitted.Version, IdempotencyKey: "review"},
		Decision:    "approve",
	})
	if err != nil {
		t.Fatal(err)
	}
	issued, err := service.Issue(created.BatchID, workflow.IssueCommand{CommandMeta: workflow.CommandMeta{
		Actor: "deployer-c", ExpectedVersion: approved.Version, IdempotencyKey: "issue",
	}})
	if err != nil {
		t.Fatal(err)
	}
	credential, err := service.GetCredential(created.BatchID)
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	var done sync.WaitGroup
	done.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			result, verifyErr := service.VerifyCredential(created.BatchID, issued.ID, credential.ContentDigest)
			if verifyErr == nil && !result.Valid {
				verifyErr = errInvalidVerification{}
			}
			errs <- verifyErr
		}()
	}
	ready.Wait()
	close(start)
	done.Wait()
	close(errs)
	for verifyErr := range errs {
		if verifyErr != nil {
			t.Fatal(verifyErr)
		}
	}
}

type errInvalidVerification struct{}

func (errInvalidVerification) Error() string { return "并发核验返回了无效结果" }
