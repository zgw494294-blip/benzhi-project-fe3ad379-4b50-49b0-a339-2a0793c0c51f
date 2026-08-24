package calibration

import (
	"fmt"
	"math"
	"sort"
)

// ValidateSnapshot checks relationships that span aggregate-owned collections.
// It is used at persistence boundaries, where accepting one broken reference
// would make later event replay ambiguous.
func ValidateSnapshot(s *Snapshot) error {
	if s == nil {
		return fmt.Errorf("投影不能为空")
	}
	if s.SchemaVersion != 1 {
		return fmt.Errorf("不支持的投影 schemaVersion: %d", s.SchemaVersion)
	}
	s.EnsureMaps()
	for key, batch := range s.Batches {
		if batch == nil {
			return fmt.Errorf("批次 %s 为空", key)
		}
		if key != batch.ID {
			return fmt.Errorf("批次映射键 %s 与实体 ID %s 不一致", key, batch.ID)
		}
		if err := validateBatch(s, batch); err != nil {
			return fmt.Errorf("批次 %s: %w", batch.ID, err)
		}
	}
	for key, sensor := range s.Sensors {
		if sensor == nil || key != sensor.ID {
			return fmt.Errorf("传感器修订映射 %s 无效", key)
		}
		if err := validateSensor(s, sensor); err != nil {
			return fmt.Errorf("传感器修订 %s: %w", sensor.ID, err)
		}
	}
	for key, profile := range s.Profiles {
		if profile == nil || key != profile.ID {
			return fmt.Errorf("方案映射 %s 无效", key)
		}
		if err := validateProfile(s, profile); err != nil {
			return fmt.Errorf("方案 %s: %w", profile.ID, err)
		}
	}
	for key, set := range s.Measurements {
		if set == nil || key != set.ID {
			return fmt.Errorf("读数集映射 %s 无效", key)
		}
		if err := validateMeasurement(s, set); err != nil {
			return fmt.Errorf("读数集 %s: %w", set.ID, err)
		}
	}
	for key, finding := range s.Findings {
		if finding == nil || key != finding.ID {
			return fmt.Errorf("问题项映射 %s 无效", key)
		}
		if err := validateFinding(s, finding); err != nil {
			return fmt.Errorf("问题项 %s: %w", finding.ID, err)
		}
	}
	for key, task := range s.RecalibrationTasks {
		if task == nil || key != task.ID {
			return fmt.Errorf("返校任务映射 %s 无效", key)
		}
		if err := validateRecalibrationTask(s, task); err != nil {
			return fmt.Errorf("返校任务 %s: %w", task.ID, err)
		}
	}
	for key, credential := range s.Credentials {
		if credential == nil || key != credential.ID {
			return fmt.Errorf("凭据映射 %s 无效", key)
		}
		if err := validateCredential(s, credential); err != nil {
			return fmt.Errorf("凭据 %s: %w", credential.ID, err)
		}
	}
	for i, review := range s.Reviews {
		if review.BatchID == "" || s.Batches[review.BatchID] == nil {
			return fmt.Errorf("复核记录 %d 引用了不存在的批次", i)
		}
		if review.Reviewer == "" || (review.Decision != "approved" && review.Decision != "returned" && review.Decision != "resubmitted") {
			return fmt.Errorf("复核记录 %d 内容无效", i)
		}
	}
	return nil
}

func validateBatch(s *Snapshot, batch *CalibrationBatch) error {
	if batch.ID == "" || batch.StationCode == "" || batch.Title == "" || batch.CreatedBy == "" {
		return fmt.Errorf("基本信息不完整")
	}
	if batch.Version < 1 || batch.CreatedAt.IsZero() {
		return fmt.Errorf("版本或创建时间无效")
	}
	if !knownStatus(batch.Status) {
		return fmt.Errorf("未知状态 %s", batch.Status)
	}
	seen := make(map[string]bool)
	for _, sensorID := range batch.SensorIDs {
		if seen[sensorID] {
			return fmt.Errorf("重复引用传感器修订 %s", sensorID)
		}
		seen[sensorID] = true
		sensor := s.Sensors[sensorID]
		if sensor == nil || sensor.BatchID != batch.ID {
			return fmt.Errorf("传感器修订 %s 不属于批次", sensorID)
		}
	}
	if batch.ProfileID != "" {
		profile := s.Profiles[batch.ProfileID]
		if profile == nil || profile.BatchID != batch.ID {
			return fmt.Errorf("锁定方案不属于批次")
		}
	}
	if batch.Status != StatusDraft && batch.ProfileID == "" {
		return fmt.Errorf("非草稿批次缺少锁定方案")
	}
	if batch.Status == StatusFrozen || batch.Status == StatusReleased {
		if batch.FrozenAt == nil || batch.ReviewedBy == "" {
			return fmt.Errorf("冻结批次缺少冻结时间或复核员")
		}
		if batch.HasSampler(batch.ReviewedBy) {
			return fmt.Errorf("冻结批次违反职责分离")
		}
	} else if batch.FrozenAt != nil {
		return fmt.Errorf("未冻结批次不应有冻结时间")
	}
	if batch.Status == StatusReleased && s.CredentialForBatch(batch.ID) == nil {
		return fmt.Errorf("已放行批次缺少凭据")
	}
	return validateRevisionSequence(s, batch.ID)
}

func validateRevisionSequence(s *Snapshot, batchID string) error {
	byCode := make(map[string][]int)
	for _, sensor := range s.Sensors {
		if sensor.BatchID == batchID {
			byCode[sensor.SensorCode] = append(byCode[sensor.SensorCode], sensor.Revision)
		}
	}
	for code, revisions := range byCode {
		sort.Ints(revisions)
		for i, revision := range revisions {
			if revision != i+1 {
				return fmt.Errorf("传感器 %s 的修订序号不连续", code)
			}
		}
	}
	return nil
}

func validateSensor(s *Snapshot, sensor *SensorRevision) error {
	if s.Batches[sensor.BatchID] == nil {
		return fmt.Errorf("所属批次不存在")
	}
	if sensor.SensorCode == "" || sensor.Metric == "" || sensor.Unit == "" || sensor.Revision < 1 {
		return fmt.Errorf("身份或修订信息无效")
	}
	if !finite(sensor.RangeMin) || !finite(sensor.RangeMax) || sensor.RangeMin >= sensor.RangeMax {
		return fmt.Errorf("量程无效")
	}
	if sensor.Revision > 1 && sensor.RecalibrationNote == "" {
		return fmt.Errorf("返校修订缺少说明")
	}
	return nil
}

func validateProfile(s *Snapshot, profile *CalibrationProfile) error {
	batch := s.Batches[profile.BatchID]
	if batch == nil || batch.ProfileID != profile.ID {
		return fmt.Errorf("所属批次未引用该方案")
	}
	if profile.LockedAt == nil || len(profile.Points) < 2 || profile.RepetitionsPerPoint < 2 {
		return fmt.Errorf("方案未完整锁定")
	}
	if !finite(profile.AbsoluteTolerance) || !finite(profile.RelativeTolerance) || !finite(profile.RepeatabilityLimit) {
		return fmt.Errorf("方案阈值不是有限数值")
	}
	for i, point := range profile.Points {
		if !finite(point) || (i > 0 && point <= profile.Points[i-1]) {
			return fmt.Errorf("标准点必须严格递增")
		}
	}
	return nil
}

func validateMeasurement(s *Snapshot, set *MeasurementSet) error {
	sensor := s.Sensors[set.SensorRevisionID]
	batch := s.Batches[set.BatchID]
	if sensor == nil || batch == nil || sensor.BatchID != set.BatchID {
		return fmt.Errorf("所属批次或传感器修订无效")
	}
	profile := s.Profiles[batch.ProfileID]
	if profile == nil || !PointInProfile(profile, set.ReferencePoint) {
		return fmt.Errorf("标准点不属于锁定方案")
	}
	if len(set.Readings) != profile.RepetitionsPerPoint || set.CapturedBy == "" || set.CapturedAt.IsZero() {
		return fmt.Errorf("读数数量或采集元数据无效")
	}
	for _, reading := range set.Readings {
		if !finite(reading) || reading < sensor.RangeMin || reading > sensor.RangeMax {
			return fmt.Errorf("包含无效或越界读数")
		}
	}
	return nil
}

func validateFinding(s *Snapshot, finding *ReviewFinding) error {
	sensor := s.Sensors[finding.SensorRevisionID]
	if sensor == nil || sensor.BatchID != finding.BatchID || s.Batches[finding.BatchID] == nil {
		return fmt.Errorf("批次与传感器归属无效")
	}
	if finding.Kind == "" || finding.Severity == "" || finding.Message == "" {
		return fmt.Errorf("问题内容不完整")
	}
	if finding.Origin != "automatic" && finding.Origin != "manual" {
		return fmt.Errorf("问题来源无效")
	}
	if finding.Origin == "manual" && finding.CreatedBy == "" {
		return fmt.Errorf("人工问题缺少创建人")
	}
	if finding.Status != FindingOpen && finding.Status != FindingResolved {
		return fmt.Errorf("未知问题状态")
	}
	if finding.Status == FindingResolved {
		resolved := s.Sensors[finding.ResolvedByRevision]
		if resolved == nil || resolved.BatchID != finding.BatchID || resolved.SensorCode != sensor.SensorCode || resolved.Revision <= sensor.Revision {
			return fmt.Errorf("闭环修订无效")
		}
		if finding.ReviewedAt == nil {
			return fmt.Errorf("闭环问题缺少时间")
		}
	}
	return nil
}

func validateRecalibrationTask(s *Snapshot, task *RecalibrationTask) error {
	sensor := s.Sensors[task.SensorRevisionID]
	if sensor == nil || sensor.BatchID != task.BatchID || sensor.Revision < 2 {
		return fmt.Errorf("批次或返校修订归属无效")
	}
	batch := s.Batches[task.BatchID]
	if batch == nil || s.Profiles[batch.ProfileID] == nil || !PointInProfile(s.Profiles[batch.ProfileID], task.ReferencePoint) || task.UpdatedAt.IsZero() {
		return fmt.Errorf("标准点或更新时间无效")
	}
	switch task.Status {
	case TaskPending, TaskPassed, TaskFailed, TaskInherited:
	default:
		return fmt.Errorf("未知任务状态")
	}
	if task.Required && task.Status == TaskInherited {
		return fmt.Errorf("必测任务不能继承旧证据")
	}
	if !task.Required && task.Status != TaskInherited {
		return fmt.Errorf("非必测任务必须标记为已继承")
	}
	for _, findingID := range task.FindingIDs {
		finding := s.Findings[findingID]
		if finding == nil || finding.BatchID != task.BatchID {
			return fmt.Errorf("关联问题不存在")
		}
	}
	if task.EvidenceMeasurementID != "" && s.Measurements[task.EvidenceMeasurementID] == nil {
		return fmt.Errorf("证据读数不存在")
	}
	return nil
}

func validateCredential(s *Snapshot, credential *ReleaseCredential) error {
	batch := s.Batches[credential.BatchID]
	if batch == nil || batch.Status != StatusReleased {
		return fmt.Errorf("所属批次未放行")
	}
	if credential.BatchVersion < 1 || credential.Decision != "approved_for_deployment" || credential.ContentDigest == "" || credential.IssuedBy == "" || credential.IssuedAt.IsZero() {
		return fmt.Errorf("签发内容不完整")
	}
	seen := make(map[string]bool)
	for _, sensorID := range credential.SensorRevisionIDs {
		sensor := s.Sensors[sensorID]
		if seen[sensorID] || sensor == nil || sensor.BatchID != credential.BatchID {
			return fmt.Errorf("凭据传感器修订集合无效")
		}
		seen[sensorID] = true
	}
	return nil
}

func knownStatus(status BatchStatus) bool {
	switch status {
	case StatusDraft, StatusPlanLocked, StatusSampling, StatusFailed, StatusReadyReview, StatusReturned, StatusFrozen, StatusReleased:
		return true
	default:
		return false
	}
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
