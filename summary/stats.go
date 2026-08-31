// Package summary turns the append-only results tree into the two generated
// artifacts — SUMMARY.md and results/latest.json — and refuses to emit a
// figure that would be dishonest.
//
// Both artifacts are derived, never written by hand. CI regenerates them and
// diffs; a hand-edited summary fails the build.
package summary

import (
	"math"
	"sort"
)

// Stats is every figure this repository is willing to publish about a set of
// observations. There is no field for a bare mean on purpose: a mean with no
// spread does not ship.
type Stats struct {
	N      int     `json:"n"`
	Median float64 `json:"median"`
	P25    float64 `json:"p25"`
	P75    float64 `json:"p75"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"stddev"`
	// CV is the coefficient of variation, sample standard deviation over the
	// mean. It is the number the hosted-runner bootstrap exists to produce.
	CV float64 `json:"cv"`
	// CVDefined is false when the mean is not positive, which makes CV
	// meaningless. A meaningless CV is reported as undefined rather than as a
	// convenient zero.
	CVDefined bool `json:"cv_defined"`
}

// Compute returns the statistics of xs. It returns ok=false for an empty
// input rather than a zero-valued Stats that reads like a measurement.
func Compute(xs []float64) (Stats, bool) {
	if len(xs) == 0 {
		return Stats{}, false
	}
	s := make([]float64, len(xs))
	copy(s, xs)
	sort.Float64s(s)

	st := Stats{
		N:      len(s),
		Median: Percentile(s, 0.5),
		P25:    Percentile(s, 0.25),
		P75:    Percentile(s, 0.75),
		Min:    s[0],
		Max:    s[len(s)-1],
	}
	sum := 0.0
	for _, v := range s {
		sum += v
	}
	st.Mean = sum / float64(len(s))
	if len(s) > 1 {
		ss := 0.0
		for _, v := range s {
			d := v - st.Mean
			ss += d * d
		}
		st.StdDev = math.Sqrt(ss / float64(len(s)-1))
	}
	if st.Mean > 0 {
		st.CV = st.StdDev / st.Mean
		st.CVDefined = true
	}
	return st, true
}

// Percentile is linear interpolation between order statistics — the
// definition R calls type 7 and NumPy uses by default. It is named here
// because an off-by-one in a percentile silently changes every published
// interval, and a reader must be able to reproduce the number.
//
// s must be sorted ascending and non-empty.
func Percentile(s []float64, q float64) float64 {
	if len(s) == 1 {
		return s[0]
	}
	h := q * float64(len(s)-1)
	lo := int(math.Floor(h))
	frac := h - float64(lo)
	if lo+1 >= len(s) {
		return s[len(s)-1]
	}
	return s[lo] + frac*(s[lo+1]-s[lo])
}
