package summary

import (
	"math"
	"testing"
)

func TestPercentileHandComputed(t *testing.T) {
	t.Parallel()
	// Seven observations, the committed repetition count. p25 lands at index
	// 0.25*(7-1) = 1.5, halfway between the second and third order
	// statistics; p75 lands at 4.5.
	s := []float64{10, 12, 14, 16, 18, 20, 22}
	cases := []struct {
		name string
		q    float64
		want float64
	}{
		{"min", 0, 10},
		{"p25 interpolates between 12 and 14", 0.25, 13},
		{"median is the fourth value", 0.5, 16},
		{"p75 interpolates between 18 and 20", 0.75, 19},
		{"max", 1, 22},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Percentile(s, tc.q); got != tc.want {
				t.Fatalf("Percentile(%v) = %v, want %v", tc.q, got, tc.want)
			}
		})
	}
}

func TestComputeHandComputed(t *testing.T) {
	t.Parallel()
	// mean 4, sample variance ((4+1+0+1+4)/4) = 2.5, stddev sqrt(2.5).
	st, ok := Compute([]float64{2, 3, 4, 5, 6})
	if !ok {
		t.Fatal("Compute returned not-ok for a non-empty input")
	}
	if st.N != 5 || st.Median != 4 || st.Min != 2 || st.Max != 6 {
		t.Fatalf("unexpected stats: %+v", st)
	}
	if math.Abs(st.Mean-4) > 1e-12 {
		t.Fatalf("mean = %v, want 4", st.Mean)
	}
	if math.Abs(st.StdDev-math.Sqrt(2.5)) > 1e-12 {
		t.Fatalf("stddev = %v, want sqrt(2.5)", st.StdDev)
	}
	if math.Abs(st.CV-math.Sqrt(2.5)/4) > 1e-12 {
		t.Fatalf("cv = %v", st.CV)
	}
	if !st.CVDefined {
		t.Fatal("CV should be defined for a positive mean")
	}
}

func TestComputeEmptyIsNotAMeasurement(t *testing.T) {
	t.Parallel()
	if _, ok := Compute(nil); ok {
		t.Fatal("Compute(nil) reported ok; an empty set is not a measurement")
	}
}

func TestCVUndefinedForZeroMean(t *testing.T) {
	t.Parallel()
	st, ok := Compute([]float64{0, 0, 0})
	if !ok {
		t.Fatal("not ok")
	}
	if st.CVDefined {
		t.Fatal("CV must be undefined when the mean is zero, not conveniently zero")
	}
}
