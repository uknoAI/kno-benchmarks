package summary

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uknoAI/kno-benchmarks/harness"
)

const methodologyFixture = `# Methodology (fixture)

| ID | Rule | Applies to |
|---|---|---|
| EX-1 | The first repetition of every configuration is a warm-up and is excluded. | every configuration |
| EX-2 | A repetition that did not complete the stage being measured is excluded. | every configuration |
`

// synthRun builds a result whose wall times are exactly the given values.
func synthRun(track, version, machine string, at time.Time, wallMS []float64, warmupFirst bool) *harness.Run {
	r := &harness.Run{
		SchemaVersion: harness.ResultSchemaVersion,
		Track:         track,
		RunID:         "run-" + machine,
		StartedAt:     at,
		MatrixSHA256:  strings.Repeat("a", 64),
		Kno:           harness.KnoBuild{Version: version, ChecksumsVerified: true},
		Machine:       harness.Machine{MachineID: machine, Runner: "github-hosted:ubuntu-latest", CPUModel: "synthetic"},
	}
	for i, w := range wallMS {
		r.Repetitions = append(r.Repetitions, harness.Repetition{
			ConfigID: "base", Axis: "base", Rep: i, Warmup: warmupFirst && i == 0,
			Assets: 100, Cases: 500, Concurrency: 8, Agent: harness.FakeAgent,
			StartedAt: at.Add(time.Duration(i) * time.Second),
			Stages:    []harness.StageResult{{Stage: "baseline", WallMS: w, ExitCode: 0}},
		})
	}
	return r
}

func writeRun(t *testing.T, root string, r *harness.Run) {
	t.Helper()
	if _, err := r.Write(filepath.Join(root, "results")); err != nil {
		t.Fatal(err)
	}
}

// fixtureRoot commits the rule fixture at a date well before any test data,
// so the exclusion rules are in force. A root without git history is a
// different scenario, exercised by the back-dated test below.
func fixtureRoot(t *testing.T) string {
	t.Helper()
	return gitRootWithRulesCommittedAt(t, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
}

func gitRootWithRulesCommittedAt(t *testing.T, when time.Time) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "METHODOLOGY.md"), []byte(methodologyFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_DATE="+when.Format(time.RFC3339),
			"GIT_COMMITTER_DATE="+when.Format(time.RFC3339),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	git("add", "METHODOLOGY.md")
	git("commit", "-q", "-m", "rules")
	return root
}

func build(t *testing.T, root string) *Report {
	t.Helper()
	rep, err := Build(Options{RepoRoot: root, ResultsDir: filepath.Join(root, "results"), MethodologyPath: "METHODOLOGY.md"})
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func find(t *testing.T, rep *Report, scope, config, stage string) Figure {
	t.Helper()
	for _, f := range rep.Figures {
		if f.Scope == scope && f.ConfigID == config && f.Stage == stage && f.Metric == MetricWallMS {
			return f
		}
	}
	t.Fatalf("no %s figure for %s/%s", scope, config, stage)
	return Figure{}
}

func TestAHighVarianceConfigurationIsFlaggedNotDropped(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	// Seven observations with a coefficient of variation of about 12%: well
	// over the committed 5% threshold. It must appear, flagged.
	vals := []float64{80, 90, 95, 100, 105, 110, 120}
	writeRun(t, root, synthRun(harness.TrackBootstrap, "v0.1.2", "m1", time.Now().UTC(), vals, false))

	rep := build(t, root)
	f := find(t, rep, ScopeWithinRun, "base", "baseline")
	if !f.HasStats {
		t.Fatal("the noisy configuration produced no figure; it must be published, flagged")
	}
	if f.Stats.CV <= CVThreshold {
		t.Fatalf("fixture CV %.4f is not above the threshold; the fixture is wrong", f.Stats.CV)
	}
	if math.Abs(f.Stats.CV-0.1323) > 0.002 {
		t.Fatalf("CV = %.4f, want about 13.2%%", f.Stats.CV)
	}
	if !f.Unstable {
		t.Fatal("a CV above the threshold must set the unstable flag")
	}
	if !containsFlag(f.Flags, "unstable") {
		t.Fatalf("flags %v do not include unstable", f.Flags)
	}
	if !strings.Contains(f.Claim, "UNSTABLE") {
		t.Fatalf("claim does not carry the instability: %q", f.Claim)
	}
	if errs := rep.Validate(); len(errs) > 0 {
		t.Fatalf("a flagged unstable figure must pass the schema check: %v", errs)
	}
}

func TestNoFigureIsCitableAndLatestJSONSaysSo(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	writeRun(t, root, synthRun(harness.TrackBootstrap, "v0.1.2", "m1", time.Now().UTC(), []float64{100, 101, 102, 103, 104, 105, 106}, false))

	rep := build(t, root)
	for _, f := range rep.Figures {
		if f.Citable {
			t.Fatalf("figure %s/%s is citable; no host in this repository is a committed measurement machine", f.Scope, f.ConfigID)
		}
		if f.CitableReason == "" {
			t.Fatalf("figure %s/%s is not citable and does not say why", f.Scope, f.ConfigID)
		}
	}
	blob, err := rep.LatestJSON()
	if err != nil {
		t.Fatal(err)
	}
	var l Latest
	if err := json.Unmarshal(blob, &l); err != nil {
		t.Fatal(err)
	}
	if len(l.CitableFigures) != 0 || l.Status != "no-citable-figures" {
		t.Fatalf("latest.json claims something: %+v", l)
	}
}

func TestEveryFakeFigureSaysItExcludesProviderLatency(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	writeRun(t, root, synthRun(harness.TrackBootstrap, "v0.1.2", "m1", time.Now().UTC(), []float64{10, 11, 12, 13, 14, 15, 16}, false))
	rep := build(t, root)
	for _, f := range rep.Figures {
		if f.Agent == harness.FakeAgent && !strings.Contains(f.Claim, "excludes provider latency") {
			t.Fatalf("claim omits the latency qualification: %q", f.Claim)
		}
	}
	if errs := rep.Validate(); len(errs) > 0 {
		t.Fatalf("schema check: %v", errs)
	}
}

func TestBetweenRunSpreadSpansMachinesAndNeedsTwoRuns(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	now := time.Now().UTC()
	writeRun(t, root, synthRun(harness.TrackBootstrap, "v0.1.2", "m1", now, []float64{100, 100, 100, 100, 100, 100, 100}, false))
	writeRun(t, root, synthRun(harness.TrackBootstrap, "v0.1.2", "m2", now.Add(time.Hour), []float64{200, 200, 200, 200, 200, 200, 200}, false))

	rep := build(t, root)
	f := find(t, rep, ScopeBetweenRun, "base", "baseline")
	if f.DistinctMachines != 2 {
		t.Fatalf("between-run figure spans %d machines, want 2", f.DistinctMachines)
	}
	if f.Stats.N != 2 || f.Stats.Median != 150 {
		t.Fatalf("between-run stats %+v", f.Stats)
	}
	if !f.Unstable {
		t.Fatal("a 2x spread between machines must be flagged unstable")
	}
	if !strings.Contains(f.CitableReason, "runner fleet") {
		t.Fatalf("between-run figure must say it describes the fleet: %q", f.CitableReason)
	}

	// One machine only: no between-run spread exists.
	root2 := fixtureRoot(t)
	writeRun(t, root2, synthRun(harness.TrackBootstrap, "v0.1.2", "m1", now, []float64{100, 100, 100, 100, 100, 100, 100}, false))
	f2 := find(t, build(t, root2), ScopeBetweenRun, "base", "baseline")
	if f2.HasStats {
		t.Fatal("a single run produced a between-run spread; a spread needs at least two observations")
	}
}

func TestDryRunsAreNeverSummarized(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	r := synthRun(harness.TrackDryRun, "v0.1.2", "m1", time.Now().UTC(), []float64{1, 2, 3, 4, 5, 6, 7}, false)
	r.DryRun = true
	writeRun(t, root, r)
	rep := build(t, root)
	if len(rep.Figures) != 0 {
		t.Fatalf("a dry run produced %d figures; it measures nothing", len(rep.Figures))
	}
}

func TestInsufficientRepetitionsProduceNoFigure(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	// Seven attempted, three of them failed: four succeed, below the floor of
	// five. The observations are still in the record.
	r := synthRun(harness.TrackBootstrap, "v0.1.2", "m1", time.Now().UTC(), []float64{10, 11, 12, 13, 14, 15, 16}, false)
	for i := range 3 {
		r.Repetitions[i].Stages[0].Error = "synthetic failure"
	}
	writeRun(t, root, r)
	f := find(t, build(t, root), ScopeWithinRun, "base", "baseline")
	if f.HasStats {
		t.Fatalf("a configuration with four successful repetitions produced a figure: %+v", f.Stats)
	}
	if !containsPrefix(f.Flags, "insufficient-n") {
		t.Fatalf("flags %v do not say why there is no figure", f.Flags)
	}
}

// TestExclusionIsRefusedWhenItsRulePostdatesTheData is the back-dated fixture
// pair: identical data, one recorded before the rule was committed and one
// after. Only the later one may have its warm-up excluded.
func TestExclusionIsRefusedWhenItsRulePostdatesTheData(t *testing.T) {
	t.Parallel()
	ruleCommittedAt := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	root := gitRootWithRulesCommittedAt(t, ruleCommittedAt)

	before := synthRun(harness.TrackBootstrap, "v0.1.2", "old", ruleCommittedAt.Add(-24*time.Hour),
		[]float64{999, 10, 10, 10, 10, 10, 10, 10}, true)
	after := synthRun(harness.TrackBootstrap, "v0.1.2", "new", ruleCommittedAt.Add(24*time.Hour),
		[]float64{999, 10, 10, 10, 10, 10, 10, 10}, true)
	writeRun(t, root, before)
	writeRun(t, root, after)

	rep := build(t, root)
	if r := rep.Rules["EX-1"]; !r.Committed {
		t.Fatal("EX-1 was not dated from git history")
	}

	var old, recent Figure
	for _, f := range rep.Figures {
		if f.Scope != ScopeWithinRun {
			continue
		}
		switch f.MachineID {
		case "old":
			old = f
		case "new":
			recent = f
		}
	}
	if old.Stats.N != 8 {
		t.Fatalf("data recorded before the rule kept n=%d, want 8: the exclusion must be refused", old.Stats.N)
	}
	if !containsFlag(old.Flags, "exclusion-refused:EX-1") {
		t.Fatalf("refused exclusion is not surfaced: %v", old.Flags)
	}
	if recent.Stats.N != 7 {
		t.Fatalf("data recorded after the rule kept n=%d, want 7: the warm-up must be excluded", recent.Stats.N)
	}
	if containsFlag(recent.Flags, "exclusion-refused:EX-1") {
		t.Fatalf("exclusion was wrongly refused for data recorded after the rule: %v", recent.Flags)
	}
	if rep.RefusedExclusions["EX-1"] == 0 {
		t.Fatal("the refusal was not counted for the summary")
	}
}

func containsFlag(flags []string, want string) bool {
	for _, f := range flags {
		if f == want {
			return true
		}
	}
	return false
}

func containsPrefix(flags []string, prefix string) bool {
	for _, f := range flags {
		if strings.HasPrefix(f, prefix) {
			return true
		}
	}
	return false
}
