package evaluation

import (
	"math"
	"testing"

	"sensor-calibration-release/internal/domain/calibration"
)

func TestCalculateAndCompare(t *testing.T) {
	stats := Calculate(100, []float64{98, 100, 102})
	if stats.Mean != 100 || stats.AbsoluteError != 0 || stats.RelativeError != 0 || stats.Spread != 4 {
		t.Fatalf("统计结果不符合预期: %+v", stats)
	}
	profile := &calibration.CalibrationProfile{AbsoluteTolerance: 1, RelativeTolerance: 0.01, RepeatabilityLimit: 3}
	findings := Compare(100, stats, profile)
	if len(findings) != 1 || findings[0].Kind != "repeatability" {
		t.Fatalf("应只产生重复性问题: %+v", findings)
	}
}

func TestCalculateZeroReference(t *testing.T) {
	stats := Calculate(0, []float64{1, 1, 1})
	if math.IsNaN(stats.RelativeError) || math.IsInf(stats.RelativeError, 0) || stats.RelativeError != 0 {
		t.Fatalf("零标准点的相对误差应按零处理: %+v", stats)
	}
}
