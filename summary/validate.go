package summary

import (
	"fmt"
	"strings"

	"github.com/uknoAI/kno-benchmarks/harness"
)

// Validate is the schema check that fails the build. It enforces the rules
// that are easy to state and easy to forget:
//
//   - no published figure lacks n, median, p25, p75 and CV;
//   - a configuration whose CV exceeds the threshold is flagged, not omitted;
//   - every figure carries a non-empty claim string;
//   - every `fake:` figure's claim says it excludes provider latency.
func (r *Report) Validate() []error {
	var errs []error
	for _, f := range r.Figures {
		id := fmt.Sprintf("%s/%s/%s/%s", f.Scope, f.ConfigID, f.Stage, f.Metric)
		if f.Claim == "" {
			errs = append(errs, fmt.Errorf("%s: empty claim string", id))
		}
		if f.Agent == harness.FakeAgent && !strings.Contains(f.Claim, "excludes provider latency") {
			errs = append(errs, fmt.Errorf("%s: a fake: figure's claim must contain %q", id, "excludes provider latency"))
		}
		if !f.HasStats {
			if len(f.Flags) == 0 {
				errs = append(errs, fmt.Errorf("%s: no statistic and no flag explaining why", id))
			}
			continue
		}
		if f.Stats.N <= 0 {
			errs = append(errs, fmt.Errorf("%s: published figure with n=%d", id, f.Stats.N))
		}
		if f.Stats.Median <= 0 || f.Stats.P25 <= 0 || f.Stats.P75 <= 0 {
			errs = append(errs, fmt.Errorf("%s: published figure missing median/p25/p75", id))
		}
		if !f.Stats.CVDefined && !f.Unstable {
			errs = append(errs, fmt.Errorf("%s: CV is undefined and the figure is not flagged unstable", id))
		}
		if f.Stats.CVDefined && f.Stats.CV > CVThreshold && !f.Unstable {
			errs = append(errs, fmt.Errorf("%s: CV %.3f exceeds the %.3f threshold but the figure is not flagged unstable", id, f.Stats.CV, CVThreshold))
		}
		if f.Citable && f.Stats.N < MinSuccessfulReps {
			errs = append(errs, fmt.Errorf("%s: citable with n=%d, below the committed floor of %d", id, f.Stats.N, MinSuccessfulReps))
		}
	}
	return errs
}
