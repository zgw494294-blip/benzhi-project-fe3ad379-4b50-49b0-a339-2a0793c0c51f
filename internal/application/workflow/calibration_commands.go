package workflow

import (
	"encoding/json"
	"sort"

	"sensor-calibration-release/internal/domain/calibration"
	"sensor-calibration-release/internal/policy/evaluation"
	"sensor-calibration-release/internal/storage/jsonstore"
)

func (s *Service) SubmitMeasurement(batchID string, cmd SubmitMeasurementCommand) (MutationResponse, error) {
	batchCmd := SubmitMeasurementBatchCommand{CommandMeta: cmd.CommandMeta, SensorRevisionID: cmd.SensorRevisionID, Measurements: []MeasurementInput{{ReferencePoint: cmd.ReferencePoint, Readings: cmd.Readings}}}
	result, err := s.submitMeasurementBatch(batchID, batchCmd, "measurement.submitted")
	if err != nil {
		return MutationResponse{}, err
	}
	response := MutationResponse{BatchID: result.BatchID, Version: result.Version, Status: result.Status, Replayed: result.Replayed, PendingRetestPoints: result.PendingRetestPoints, ResolvedFindingIDs: result.ResolvedFindingIDs}
	if len(result.Measurements) > 0 {
		response.ID = result.Measurements[0].ID
	}
	return response, nil
}

func (s *Service) SubmitMeasurementBatch(batchID string, cmd SubmitMeasurementBatchCommand) (MeasurementBatchResponse, error) {
	return s.submitMeasurementBatch(batchID, cmd, "measurements.batch_submitted")
}

func (s *Service) submitMeasurementBatch(batchID string, cmd SubmitMeasurementBatchCommand, command string) (MeasurementBatchResponse, error) {
	if err := validateMeta(cmd.CommandMeta); err != nil {
		return MeasurementBatchResponse{}, calibration.Validation("%v", err)
	}
	fingerprint, err := requestFingerprint(cmd)
	if err != nil {
		return MeasurementBatchResponse{}, err
	}
	sets := make([]*calibration.MeasurementSet, 0, len(cmd.Measurements))
	response := MeasurementBatchResponse{BatchID: batchID, SensorRevisionID: cmd.SensorRevisionID, Measurements: make([]MeasurementResult, 0, len(cmd.Measurements))}
	for _, input := range cmd.Measurements {
		set := &calibration.MeasurementSet{ID: s.newID("measurement"), BatchID: batchID, SensorRevisionID: cmd.SensorRevisionID, ReferencePoint: input.ReferencePoint, Readings: append([]float64(nil), input.Readings...), CapturedBy: cmd.Actor}
		s.evaluator.ApplyStatistics(set)
		sets = append(sets, set)
		response.Measurements = append(response.Measurements, MeasurementResult{ID: set.ID, ReferencePoint: set.ReferencePoint, Mean: set.Mean, AbsoluteError: set.AbsoluteError, RelativeError: set.RelativeError, Spread: set.Spread})
	}
	result, err := s.store.Commit(jsonstore.CommitRequest{AggregateID: batchID, ExpectedVersion: cmd.ExpectedVersion, IdempotencyKey: cmd.IdempotencyKey, RequestDigest: fingerprint, Command: command, Actor: cmd.Actor, Response: &response, Mutate: func(snapshot *calibration.Snapshot) error {
		if err := calibration.PutMeasurements(snapshot, sets, s.now()); err != nil {
			return err
		}
		batch := snapshot.Batches[batchID]
		profile := snapshot.Profiles[batch.ProfileID]
		resolvedSet := make(map[string]bool)
		for _, set := range sets {
			drafts := evaluation.Compare(set.ReferencePoint, evaluation.Statistics{Mean: set.Mean, AbsoluteError: set.AbsoluteError, RelativeError: set.RelativeError, Spread: set.Spread}, profile)
			findings := evaluation.Materialize(batchID, set.SensorRevisionID, drafts, func() string { return s.newID("finding") }, s.now())
			evaluation.UpdateRecalibrationTask(snapshot, set, drafts, s.now())
			for _, id := range calibration.ReplaceAutomaticFindings(snapshot, batchID, set.SensorRevisionID, set.ReferencePoint, findings, s.now()) {
				resolvedSet[id] = true
			}
		}
		evaluated, err := s.evaluator.EvaluateBatch(snapshot, batchID)
		if err != nil {
			return err
		}
		if err := calibration.SetEvaluationStatus(snapshot, batchID, evaluated.Complete); err != nil {
			return err
		}
		for id := range resolvedSet {
			response.ResolvedFindingIDs = append(response.ResolvedFindingIDs, id)
		}
		sort.Strings(response.ResolvedFindingIDs)
		response.PendingRetestPoints = pendingRetestPoints(snapshot, cmd.SensorRevisionID)
		response.Version, response.Status = batch.Version, string(batch.Status)
		return nil
	}})
	if err != nil {
		return MeasurementBatchResponse{}, err
	}
	if err := json.Unmarshal(result.Response, &response); err != nil {
		return MeasurementBatchResponse{}, err
	}
	response.Replayed = result.Replayed
	return response, nil
}

func (s *Service) Recalibrate(batchID string, cmd RecalibrateCommand) (MutationResponse, error) {
	if err := validateMeta(cmd.CommandMeta); err != nil {
		return MutationResponse{}, calibration.Validation("%v", err)
	}
	fingerprint, err := requestFingerprint(cmd)
	if err != nil {
		return MutationResponse{}, err
	}
	id := s.newID("sensor")
	response := MutationResponse{BatchID: batchID, ID: id}
	result, err := s.store.Commit(jsonstore.CommitRequest{AggregateID: batchID, ExpectedVersion: cmd.ExpectedVersion, IdempotencyKey: cmd.IdempotencyKey, RequestDigest: fingerprint, Command: "sensor.recalibrated", Actor: cmd.Actor, Response: &response, Mutate: func(snapshot *calibration.Snapshot) error {
		sensor, err := calibration.RecalibrateSensor(snapshot, batchID, cmd.SensorRevisionID, id, cmd.Note, s.now())
		if err != nil {
			return err
		}
		if _, err := evaluation.BuildRecalibrationTasks(snapshot, sensor.ID, func() string { return s.newID("retest") }, s.now()); err != nil {
			return err
		}
		batch := snapshot.Batches[batchID]
		response.ID, response.Version, response.Status = sensor.ID, batch.Version, string(batch.Status)
		response.PendingRetestPoints = pendingRetestPoints(snapshot, sensor.ID)
		return nil
	}})
	if err != nil {
		return MutationResponse{}, err
	}
	return decodeMutation(result)
}

func pendingRetestPoints(snapshot *calibration.Snapshot, revisionID string) int {
	count := 0
	for _, task := range snapshot.RecalibrationTasks {
		if task.SensorRevisionID == revisionID && task.Required && task.Status != calibration.TaskPassed {
			count++
		}
	}
	return count
}
