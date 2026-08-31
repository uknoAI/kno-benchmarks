package summary

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/uknoAI/kno-benchmarks/harness"
)

// Thresholds committed in METHODOLOGY.md before any data existed. They live
// here as constants so the code and the document cannot drift apart silently;
// changing either without the other is a reviewable diff.
const (
	// CVThreshold is the coefficient of variation above which a configuration
	// is published flagged `unstable`. It is never dropped: dropping a noisy
	// cell is cherry-picking with extra steps.
	CVThreshold = 0.05
	// MinSuccessfulReps is the floor below which a configuration produces no
	// summary figure. Its observations are still published.
	MinSuccessfulReps = 5
)

// Metrics.
const (
	MetricWallMS  = "wall_ms"
	MetricPeakRSS = "peak_rss_bytes"
)

// Scopes. The distinction is the whole point of the bootstrap: within-run
// spread is one machine repeating itself, between-run spread is the fleet.
const (
	ScopeWithinRun  = "within-run"
	ScopeBetweenRun = "between-run"
)

// Figure is one published statistic and everything a reader needs to know
// before quoting it.
type Figure struct {
	Scope       string `json:"scope"`
	Track       string `json:"track"`
	KnoVersion  string `json:"kno_version"`
	ConfigID    string `json:"config_id"`
	Axis        string `json:"axis,omitempty"`
	Stage       string `json:"stage"`
	Metric      string `json:"metric"`
	Unit        string `json:"unit"`
	Agent       string `json:"agent"`
	Assets      int    `json:"assets"`
	Cases       int    `json:"cases"`
	Concurrency int    `json:"concurrency"`

	RunID            string `json:"run_id,omitempty"`
	MachineID        string `json:"machine_id,omitempty"`
	DistinctMachines int    `json:"distinct_machines,omitempty"`
	Runner           string `json:"runner"`
	CPUModel         string `json:"cpu_model,omitempty"`

	HasStats bool  `json:"has_stats"`
	Stats    Stats `json:"stats"`

	Unstable bool     `json:"unstable"`
	Flags    []string `json:"flags,omitempty"`

	Citable       bool   `json:"citable"`
	CitableReason string `json:"citable_reason"`
	Claim         string `json:"claim"`
}

// RunIndex is one result file, as SUMMARY.md lists it.
type RunIndex struct {
	Path          string    `json:"path"`
	Track         string    `json:"track"`
	RunID         string    `json:"run_id"`
	KnoVersion    string    `json:"kno_version"`
	StartedAt     time.Time `json:"started_at"`
	Runner        string    `json:"runner"`
	MachineID     string    `json:"machine_id"`
	MatrixSHA256  string    `json:"matrix_sha256"`
	Repetitions   int       `json:"repetitions"`
	Partial       bool      `json:"partial"`
	BudgetAborted bool      `json:"budget_aborted"`
	ChecksumOK    bool      `json:"checksums_verified"`
}

// Report is everything the two generated artifacts are rendered from.
type Report struct {
	Figures []Figure
	Runs    []RunIndex
	Rules   map[string]Rule
	// RefusedExclusions records, per rule, how many observations the rule
	// could not be applied to because it was committed after the data.
	RefusedExclusions map[string]int
	Notes             []string
}

// Options configures a summarization.
type Options struct {
	RepoRoot        string
	ResultsDir      string
	MethodologyPath string
}

type observation struct {
	value float64
}

// Build reads every result under ResultsDir and derives the report.
func Build(opts Options) (*Report, error) {
	rules, err := LoadRules(opts.RepoRoot, opts.MethodologyPath)
	if err != nil {
		return nil, err
	}
	runs, paths, err := loadRuns(opts.ResultsDir)
	if err != nil {
		return nil, err
	}

	rep := &Report{Rules: rules, RefusedExclusions: map[string]int{}}
	for i, r := range runs {
		rel, _ := filepath.Rel(opts.RepoRoot, paths[i])
		rep.Runs = append(rep.Runs, RunIndex{
			Path:          filepath.ToSlash(rel),
			Track:         r.Track,
			RunID:         r.RunID,
			KnoVersion:    r.Kno.Version,
			StartedAt:     r.StartedAt,
			Runner:        r.Machine.Runner,
			MachineID:     r.Machine.MachineID,
			MatrixSHA256:  r.MatrixSHA256,
			Repetitions:   len(r.Repetitions),
			Partial:       r.Partial,
			BudgetAborted: r.BudgetAborted,
			ChecksumOK:    r.Kno.ChecksumsVerified,
		})
	}
	sort.Slice(rep.Runs, func(i, j int) bool { return rep.Runs[i].Path < rep.Runs[j].Path })

	within := buildWithinRun(runs, rules, rep)
	rep.Figures = append(rep.Figures, within...)
	rep.Figures = append(rep.Figures, buildBetweenRun(within)...)
	sortFigures(rep.Figures)
	return rep, nil
}

type key struct {
	track, version, machine, runID, config, stage, metric string
}

func buildWithinRun(runs []*harness.Run, rules map[string]Rule, rep *Report) []Figure {
	type bucket struct {
		obs      []observation
		attempts int
		warmIncl int
		failed   int
		refused  map[string]bool
		cfg      harness.Repetition
		run      *harness.Run
	}
	buckets := map[key]*bucket{}

	for _, r := range runs {
		if r.DryRun {
			continue // a dry run measures nothing and is never summarized.
		}
		for _, rr := range r.Repetitions {
			for _, st := range rr.Stages {
				for _, metric := range []string{MetricWallMS, MetricPeakRSS} {
					v, ok := metricValue(st, metric)
					if !ok {
						continue
					}
					k := key{r.Track, r.Kno.Version, r.Machine.MachineID, r.RunID, rr.ConfigID, st.Stage, metric}
					b := buckets[k]
					if b == nil {
						b = &bucket{refused: map[string]bool{}, cfg: rr, run: r}
						buckets[k] = b
					}
					if !rr.Warmup {
						// The floor is a floor on measured repetitions. A
						// warm-up is not one of them, so counting it here
						// would demand a repetition that the rules exclude.
						b.attempts++
					}
					if st.Error != "" || rr.Error != "" {
						// EX-2: a repetition that did not complete its stage
						// is excluded from timing statistics and published
						// with its error.
						if applyRule(rules, "EX-2", rr.StartedAt, b.refused, rep) {
							b.failed++
							continue
						}
					}
					if rr.Warmup {
						// EX-1: the warm-up repetition is excluded from
						// summary statistics and still published.
						if applyRule(rules, "EX-1", rr.StartedAt, b.refused, rep) {
							continue
						}
						b.warmIncl++
					}
					b.obs = append(b.obs, observation{value: v})
				}
			}
		}
	}

	var out []Figure
	for k, b := range buckets {
		values := make([]float64, 0, len(b.obs))
		for _, o := range b.obs {
			values = append(values, o.value)
		}
		f := Figure{
			Scope:       ScopeWithinRun,
			Track:       k.track,
			KnoVersion:  k.version,
			ConfigID:    k.config,
			Axis:        b.cfg.Axis,
			Stage:       k.stage,
			Metric:      k.metric,
			Unit:        unitOf(k.metric),
			Agent:       b.cfg.Agent,
			Assets:      b.cfg.Assets,
			Cases:       b.cfg.Cases,
			Concurrency: b.cfg.Concurrency,
			RunID:       k.runID,
			MachineID:   k.machine,
			Runner:      b.run.Machine.Runner,
			CPUModel:    b.run.Machine.CPUModel,
		}
		st, ok := Compute(values)
		required := min(MinSuccessfulReps, b.attempts)
		if ok && len(values) >= required {
			f.HasStats = true
			f.Stats = st
			if !st.CVDefined || st.CV > CVThreshold {
				f.Unstable = true
				f.Flags = append(f.Flags, "unstable")
			}
		} else {
			f.Flags = append(f.Flags, fmt.Sprintf("insufficient-n(%d of %d, need %d)", len(values), b.attempts, required))
		}
		if b.warmIncl > 0 {
			f.Flags = append(f.Flags, fmt.Sprintf("warmup-included(%d)", b.warmIncl))
		}
		for id := range b.refused {
			f.Flags = append(f.Flags, "exclusion-refused:"+id)
		}
		if b.failed > 0 {
			f.Flags = append(f.Flags, fmt.Sprintf("failed-reps(%d)", b.failed))
		}
		sort.Strings(f.Flags)
		f.Citable, f.CitableReason = citability(b.run, f)
		f.Claim = buildClaim(f)
		out = append(out, f)
	}
	sortFigures(out)
	return out
}

// applyRule returns true when the exclusion may be applied. When it may not,
// it records the refusal so the summary can say plainly that a rule was not
// honoured because it postdates the data.
func applyRule(rules map[string]Rule, id string, observedAt time.Time, refused map[string]bool, rep *Report) bool {
	r, ok := rules[id]
	if ok && r.Applies(observedAt) {
		return true
	}
	refused[id] = true
	rep.RefusedExclusions[id]++
	return false
}

// buildBetweenRun aggregates the per-run medians of a configuration across
// runs. On GitHub-hosted runners each run lands on a different physical
// machine, so this spread is the fleet's, not the software's — which is
// exactly the quantity the bootstrap exists to measure.
func buildBetweenRun(within []Figure) []Figure {
	type bkey struct{ track, version, config, stage, metric string }
	groups := map[bkey][]Figure{}
	for _, f := range within {
		if !f.HasStats {
			continue
		}
		k := bkey{f.Track, f.KnoVersion, f.ConfigID, f.Stage, f.Metric}
		groups[k] = append(groups[k], f)
	}

	var out []Figure
	for k, fs := range groups {
		machines := map[string]bool{}
		values := make([]float64, 0, len(fs))
		for _, f := range fs {
			values = append(values, f.Stats.Median)
			machines[f.MachineID] = true
		}
		st, ok := Compute(values)
		if !ok {
			continue
		}
		proto := fs[0]
		f := Figure{
			Scope:            ScopeBetweenRun,
			Track:            k.track,
			KnoVersion:       k.version,
			ConfigID:         k.config,
			Axis:             proto.Axis,
			Stage:            k.stage,
			Metric:           k.metric,
			Unit:             unitOf(k.metric),
			Agent:            proto.Agent,
			Assets:           proto.Assets,
			Cases:            proto.Cases,
			Concurrency:      proto.Concurrency,
			DistinctMachines: len(machines),
			Runner:           proto.Runner,
			HasStats:         true,
			Stats:            st,
		}
		if st.N < 2 {
			f.HasStats = false
			f.Flags = append(f.Flags, "insufficient-n(1 run; a spread needs at least two)")
		} else if !st.CVDefined || st.CV > CVThreshold {
			f.Unstable = true
			f.Flags = append(f.Flags, "unstable")
		}
		f.Citable = false
		f.CitableReason = "between-run spread across " + fmt.Sprint(len(machines)) + " machine fingerprint(s): this is a property of the runner fleet, not of kno"
		f.Claim = buildClaim(f)
		out = append(out, f)
	}
	sortFigures(out)
	return out
}

func metricValue(st harness.StageResult, metric string) (float64, bool) {
	switch metric {
	case MetricWallMS:
		if st.WallMS <= 0 {
			return 0, false
		}
		return st.WallMS, true
	case MetricPeakRSS:
		if st.PeakRSSBytes <= 0 {
			return 0, false
		}
		return float64(st.PeakRSSBytes), true
	}
	return 0, false
}

func unitOf(metric string) string {
	if metric == MetricPeakRSS {
		return "bytes"
	}
	return "milliseconds"
}

// citability is the single gate on whether a figure may be quoted. It returns
// false for everything this repository can currently produce, and says why.
func citability(r *harness.Run, f Figure) (bool, string) {
	switch {
	case r.DryRun:
		return false, "dry run: proves the harness executes, measures nothing"
	case r.Track != harness.TrackBootstrap && r.Track != "measured":
		return false, fmt.Sprintf("track %q is not a measurement track", r.Track)
	case !r.Machine.Citable:
		return false, "the measuring host is not a committed measurement machine (RUNNER.md); the hosted-runner bootstrap has not concluded"
	case !r.Kno.ChecksumsVerified:
		return false, "the measured binary's digest was not matched against a published checksums.txt"
	case !f.HasStats:
		return false, "no statistic: too few successful repetitions"
	case r.Partial:
		return false, "the run did not complete the committed matrix"
	}
	return true, ""
}

// buildClaim writes the sentence a consumer must render adjacent to the
// number. Every `fake:` figure's claim contains "excludes provider latency":
// the single most likely way this repository becomes dishonest is a true
// number quoted in a context that implies an end-to-end time.
func buildClaim(f Figure) string {
	var b strings.Builder
	scope := "one machine repeating itself"
	if f.Scope == ScopeBetweenRun {
		scope = fmt.Sprintf("%d separate runs spanning %d machine fingerprint(s)", f.Stats.N, f.DistinctMachines)
	}
	fmt.Fprintf(&b, "kno %s, stage %s, %d Cases x %d Assets, agent %s at concurrency %d: ",
		orUnknown(f.KnoVersion), f.Stage, f.Cases, f.Assets, orUnknown(f.Agent), f.Concurrency)
	if f.HasStats {
		fmt.Fprintf(&b, "median %s (IQR [%s, %s], min %s, max %s, n=%d, CV %s), %s, on %s.",
			num(f.Stats.Median, f.Unit), num(f.Stats.P25, f.Unit), num(f.Stats.P75, f.Unit),
			num(f.Stats.Min, f.Unit), num(f.Stats.Max, f.Unit), f.Stats.N, cvString(f.Stats), scope, runnerLabel(f))
	} else {
		fmt.Fprintf(&b, "no figure (%s).", strings.Join(f.Flags, "; "))
	}
	if f.Unstable {
		b.WriteString(fmt.Sprintf(" UNSTABLE: coefficient of variation exceeds the committed %.0f%% threshold.", CVThreshold*100))
	}
	if f.Agent == harness.FakeAgent {
		b.WriteString(" This figure excludes provider latency and is not an end-to-end time.")
	}
	if !f.Citable {
		b.WriteString(" NOT CITABLE: " + f.CitableReason + ".")
	}
	return b.String()
}

func runnerLabel(f Figure) string {
	if f.CPUModel == "" {
		return orUnknown(f.Runner)
	}
	return f.Runner + " (" + f.CPUModel + ")"
}

func cvString(s Stats) string {
	if !s.CVDefined {
		return "undefined"
	}
	return fmt.Sprintf("%.1f%%", s.CV*100)
}

func num(v float64, unit string) string {
	if unit == "bytes" {
		return fmt.Sprintf("%.1f MiB", v/(1024*1024))
	}
	if v >= 1000 {
		return fmt.Sprintf("%.2f s", v/1000)
	}
	return fmt.Sprintf("%.1f ms", v)
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func sortFigures(fs []Figure) {
	sort.Slice(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		ka := []string{a.Track, a.KnoVersion, a.Scope, a.ConfigID, a.Stage, a.Metric, a.RunID, a.MachineID}
		kb := []string{b.Track, b.KnoVersion, b.Scope, b.ConfigID, b.Stage, b.Metric, b.RunID, b.MachineID}
		for i := range ka {
			if ka[i] != kb[i] {
				return ka[i] < kb[i]
			}
		}
		return false
	})
}

func loadRuns(resultsDir string) ([]*harness.Run, []string, error) {
	var runs []*harness.Run
	var paths []string
	err := filepath.WalkDir(resultsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		if filepath.Base(path) == "latest.json" {
			return nil
		}
		blob, rerr := os.ReadFile(path) //nolint:gosec // walking a committed results tree.
		if rerr != nil {
			return fmt.Errorf("read %s: %w", path, rerr)
		}
		var r harness.Run
		if jerr := json.Unmarshal(blob, &r); jerr != nil {
			return fmt.Errorf("parse %s: %w", path, jerr)
		}
		if r.SchemaVersion != harness.ResultSchemaVersion {
			return fmt.Errorf("%s: unsupported schema_version %d", path, r.SchemaVersion)
		}
		runs = append(runs, &r)
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return runs, paths, nil
}
