package calibration

type Snapshot struct {
	SchemaVersion      int                            `json:"schemaVersion"`
	Batches            map[string]*CalibrationBatch   `json:"batches"`
	Sensors            map[string]*SensorRevision     `json:"sensors"`
	Profiles           map[string]*CalibrationProfile `json:"profiles"`
	Measurements       map[string]*MeasurementSet     `json:"measurements"`
	Findings           map[string]*ReviewFinding      `json:"findings"`
	RecalibrationTasks map[string]*RecalibrationTask  `json:"recalibrationTasks"`
	Credentials        map[string]*ReleaseCredential  `json:"credentials"`
	Reviews            []ReviewRecord                 `json:"reviews"`
}

func NewSnapshot() *Snapshot {
	return &Snapshot{
		SchemaVersion: 1,
		Batches:       make(map[string]*CalibrationBatch), Sensors: make(map[string]*SensorRevision),
		Profiles: make(map[string]*CalibrationProfile), Measurements: make(map[string]*MeasurementSet),
		Findings: make(map[string]*ReviewFinding), Credentials: make(map[string]*ReleaseCredential),
		RecalibrationTasks: make(map[string]*RecalibrationTask),
	}
}

func (s *Snapshot) EnsureMaps() {
	if s.Batches == nil {
		s.Batches = make(map[string]*CalibrationBatch)
	}
	if s.Sensors == nil {
		s.Sensors = make(map[string]*SensorRevision)
	}
	if s.Profiles == nil {
		s.Profiles = make(map[string]*CalibrationProfile)
	}
	if s.Measurements == nil {
		s.Measurements = make(map[string]*MeasurementSet)
	}
	if s.Findings == nil {
		s.Findings = make(map[string]*ReviewFinding)
	}
	if s.RecalibrationTasks == nil {
		s.RecalibrationTasks = make(map[string]*RecalibrationTask)
	}
	if s.Credentials == nil {
		s.Credentials = make(map[string]*ReleaseCredential)
	}
}

func (s *Snapshot) IsCurrentRevision(sensorID, batchID string) bool {
	sensor := s.Sensors[sensorID]
	if sensor == nil || sensor.BatchID != batchID {
		return false
	}
	for _, candidate := range s.Sensors {
		if candidate.BatchID == batchID && candidate.SensorCode == sensor.SensorCode && candidate.Revision > sensor.Revision {
			return false
		}
	}
	return true
}

func (s *Snapshot) RecalibrationTasksForRevision(batchID, revisionID string) []*RecalibrationTask {
	out := make([]*RecalibrationTask, 0)
	for _, task := range s.RecalibrationTasks {
		if task.BatchID == batchID && task.SensorRevisionID == revisionID {
			out = append(out, task)
		}
	}
	return out
}

func (s *Snapshot) Batch(id string) (*CalibrationBatch, error) {
	b := s.Batches[id]
	if b == nil {
		return nil, NotFound("校准批次 %s 不存在", id)
	}
	return b, nil
}

func (s *Snapshot) CurrentSensors(batchID string) []*SensorRevision {
	latest := make(map[string]*SensorRevision)
	for _, sensor := range s.Sensors {
		if sensor.BatchID != batchID {
			continue
		}
		old := latest[sensor.SensorCode]
		if old == nil || sensor.Revision > old.Revision {
			latest[sensor.SensorCode] = sensor
		}
	}
	out := make([]*SensorRevision, 0, len(latest))
	for _, sensor := range latest {
		out = append(out, sensor)
	}
	return out
}

func (s *Snapshot) BatchFindings(batchID string, openOnly bool) []*ReviewFinding {
	out := make([]*ReviewFinding, 0)
	for _, finding := range s.Findings {
		if finding.BatchID == batchID && (!openOnly || finding.Status == FindingOpen) {
			out = append(out, finding)
		}
	}
	return out
}

func (s *Snapshot) CredentialForBatch(batchID string) *ReleaseCredential {
	for _, c := range s.Credentials {
		if c.BatchID == batchID {
			return c
		}
	}
	return nil
}
