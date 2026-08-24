package calibration

import (
	"testing"
	"time"
)

func TestProfileRangeAndFreezeSeparation(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	s := NewSnapshot()
	b, err := CreateBatch("b1", "ST-1", "校准", "tech", now)
	if err != nil {
		t.Fatal(err)
	}
	s.Batches[b.ID] = b
	if _, err := RegisterSensor(s, b.ID, "s1", "S-1", "temperature", "C", 0, 100, now); err != nil {
		t.Fatal(err)
	}
	err = LockProfile(s, &CalibrationProfile{ID: "p1", BatchID: b.ID, Points: []float64{-1, 50}, RepetitionsPerPoint: 3, AbsoluteTolerance: 1, RelativeTolerance: .02, RepeatabilityLimit: 1}, now)
	if err == nil {
		t.Fatal("超出量程的标准点应被拒绝")
	}
	if err := LockProfile(s, &CalibrationProfile{ID: "p1", BatchID: b.ID, Points: []float64{0, 50}, RepetitionsPerPoint: 3, AbsoluteTolerance: 1, RelativeTolerance: .02, RepeatabilityLimit: 1}, now); err != nil {
		t.Fatal(err)
	}
	b.Status = StatusReadyReview
	b.SampledBy = []string{"tech"}
	if err := b.Freeze("tech", now); err == nil {
		t.Fatal("采样人不能复核自己提交的批次")
	}
}
