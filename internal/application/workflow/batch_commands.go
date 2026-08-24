package workflow

import (
	"sensor-calibration-release/internal/domain/calibration"
	"sensor-calibration-release/internal/storage/jsonstore"
)

func (s *Service) CreateBatch(cmd CreateBatchCommand) (MutationResponse, error) {
	if err := validateMeta(cmd.CommandMeta); err != nil {
		return MutationResponse{}, calibration.Validation("%v", err)
	}
	id := stableBatchID(cmd.Actor, cmd.IdempotencyKey)
	fingerprint, err := requestFingerprint(cmd)
	if err != nil {
		return MutationResponse{}, err
	}
	response := MutationResponse{BatchID: id, ID: id}
	result, err := s.store.Commit(jsonstore.CommitRequest{AggregateID: id, ExpectedVersion: cmd.ExpectedVersion, IdempotencyKey: cmd.IdempotencyKey, RequestDigest: fingerprint, Command: "batch.created", Actor: cmd.Actor, Response: &response, Mutate: func(snapshot *calibration.Snapshot) error {
		if _, exists := snapshot.Batches[id]; exists {
			return calibration.Conflict("批次标识冲突")
		}
		batch, err := calibration.CreateBatch(id, cmd.StationCode, cmd.Title, cmd.Actor, s.now())
		if err != nil {
			return err
		}
		snapshot.Batches[id] = batch
		response.Version, response.Status = batch.Version, string(batch.Status)
		return nil
	}})
	if err != nil {
		return MutationResponse{}, err
	}
	return decodeMutation(result)
}

func (s *Service) RegisterSensor(batchID string, cmd RegisterSensorCommand) (MutationResponse, error) {
	if err := validateMeta(cmd.CommandMeta); err != nil {
		return MutationResponse{}, calibration.Validation("%v", err)
	}
	id := s.newID("sensor")
	fingerprint, err := requestFingerprint(cmd)
	if err != nil {
		return MutationResponse{}, err
	}
	response := MutationResponse{BatchID: batchID, ID: id}
	result, err := s.store.Commit(jsonstore.CommitRequest{AggregateID: batchID, ExpectedVersion: cmd.ExpectedVersion, IdempotencyKey: cmd.IdempotencyKey, RequestDigest: fingerprint, Command: "sensor.registered", Actor: cmd.Actor, Response: &response, Mutate: func(snapshot *calibration.Snapshot) error {
		sensor, err := calibration.RegisterSensor(snapshot, batchID, id, cmd.SensorCode, cmd.Metric, cmd.Unit, cmd.RangeMin, cmd.RangeMax, s.now())
		if err != nil {
			return err
		}
		batch, _ := snapshot.Batch(batchID)
		response.ID, response.Version, response.Status = sensor.ID, batch.Version, string(batch.Status)
		return nil
	}})
	if err != nil {
		return MutationResponse{}, err
	}
	return decodeMutation(result)
}

func (s *Service) LockProfile(batchID string, cmd LockProfileCommand) (MutationResponse, error) {
	if err := validateMeta(cmd.CommandMeta); err != nil {
		return MutationResponse{}, calibration.Validation("%v", err)
	}
	id := s.newID("profile")
	fingerprint, err := requestFingerprint(cmd)
	if err != nil {
		return MutationResponse{}, err
	}
	response := MutationResponse{BatchID: batchID, ID: id}
	result, err := s.store.Commit(jsonstore.CommitRequest{AggregateID: batchID, ExpectedVersion: cmd.ExpectedVersion, IdempotencyKey: cmd.IdempotencyKey, RequestDigest: fingerprint, Command: "profile.locked", Actor: cmd.Actor, Response: &response, Mutate: func(snapshot *calibration.Snapshot) error {
		profile := &calibration.CalibrationProfile{ID: id, BatchID: batchID, Points: cmd.Points, RepetitionsPerPoint: cmd.RepetitionsPerPoint, AbsoluteTolerance: cmd.AbsoluteTolerance, RelativeTolerance: cmd.RelativeTolerance, RepeatabilityLimit: cmd.RepeatabilityLimit}
		if err := calibration.LockProfile(snapshot, profile, s.now()); err != nil {
			return err
		}
		batch, _ := snapshot.Batch(batchID)
		response.Version, response.Status = batch.Version, string(batch.Status)
		return nil
	}})
	if err != nil {
		return MutationResponse{}, err
	}
	return decodeMutation(result)
}
