package evaluation

import "math"

type Statistics struct {
	Mean          float64 `json:"mean"`
	AbsoluteError float64 `json:"absoluteError"`
	RelativeError float64 `json:"relativeError"`
	Spread        float64 `json:"spread"`
}

func Calculate(reference float64, readings []float64) Statistics {
	if len(readings) == 0 {
		return Statistics{}
	}
	sum, min, max := 0.0, readings[0], readings[0]
	for _, value := range readings {
		sum += value
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
	}
	mean := sum / float64(len(readings))
	abs := math.Abs(mean - reference)
	rel := 0.0
	if reference != 0 {
		rel = abs / math.Abs(reference)
	}
	return Statistics{Mean: mean, AbsoluteError: abs, RelativeError: rel, Spread: max - min}
}
