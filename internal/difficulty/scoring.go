package difficulty

import (
	"math"
	"slices"
)

// rating is how hard a part is on a scale of 1 to 10, rounded to a tenth.
func rating(r Role, bars []bar) float64 {
	hard := speed(bars)
	return math.Round((1+9*clamp(hard))*10) / 10
}

// speed is how fast a part is, from 0 for a note every few seconds to 1 for 16 a second:
// what it keeps up for a while, the 90th percentile of its bars.
func speed(bars []bar) float64 {
	return scale(percentile(bars, 0.9, rate), 2, 16)
}

// percentile is the p-th percentile of a measure of bars.
func percentile(bars []bar, p float64, measure func(bar) float64) float64 {
	if len(bars) == 0 {
		return 0
	}
	vs := make([]float64, len(bars))
	for i, b := range bars {
		vs[i] = measure(b)
	}
	slices.Sort(vs)
	return vs[min(int(p*float64(len(vs))), len(vs)-1)]
}

// scale maps v from a range onto 0 to 1, linearly, clamped.
func scale(v, from, to float64) float64 { return clamp((v - from) / (to - from)) }

func clamp(v float64) float64 { return min(max(v, 0), 1) }
