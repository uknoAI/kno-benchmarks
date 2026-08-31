package harness

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
)

// fakeExec answers with a canned --json document per stage. No network, no
// binary, no money.
type fakeExec struct {
	mu    sync.Mutex
	calls []string
	docs  map[string]string
}

func (f *fakeExec) Exec(_ context.Context, _ string, args []string, _ string) (ExecResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	stage := args[0]
	f.calls = append(f.calls, stage)
	doc := f.docs[stage]
	if doc == "" {
		doc = `{"run_id":"r-` + stage + `"}`
	}
	return ExecResult{Stdout: []byte(doc), PeakRSSBytes: 1 << 20, Wall: 1000}, nil
}

func tenConfigs(agent string) *Matrix {
	m := &Matrix{
		SchemaVersion: 1,
		Defaults:      Defaults{Agent: agent, Repetitions: 1},
		BaseCell:      Cell{Assets: 1, Cases: 1, Concurrency: 1},
	}
	for i := range 10 {
		m.Configurations = append(m.Configurations, Config{
			ID:          string(rune('a' + i)),
			Assets:      1,
			Cases:       1,
			Concurrency: 1,
			Stages:      []string{"baseline"},
			Repetitions: 1,
			MaxCostUSD:  10,
			MaxCalls:    100,
		})
	}
	return m
}

func TestAggregateBudgetAbortsTheRestOfTheMatrix(t *testing.T) {
	t.Parallel()
	// Each configuration reports one dollar of spend. The cap is $2.50, so it
	// is crossed on the third configuration and configurations four through
	// ten must not execute.
	ex := &fakeExec{docs: map[string]string{
		"baseline": `{"run_id":"r","spent_usd":"$1.00"}`,
	}}
	r := &Runner{
		Bin:       "unused",
		Matrix:    tenConfigs("openai:gpt-not-real"),
		Track:     TrackDryRun,
		WorkDir:   t.TempDir(),
		BudgetUSD: 2.50,
		Executor:  ex,
	}
	run, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(ex.calls) != 3 {
		t.Fatalf("executed %d configurations, want exactly 3 before the cap fired: %v", len(ex.calls), ex.calls)
	}
	if !run.BudgetAborted {
		t.Fatal("run is not marked budget_aborted")
	}
	if !run.Partial {
		t.Fatal("a run that stopped early must be marked partial")
	}
	if run.SpentUSD != 3 {
		t.Fatalf("spent %v, want 3", run.SpentUSD)
	}
	for _, id := range []string{"d", "e", "f", "g", "h", "i", "j"} {
		if strings.Contains(run.PartialReason, id) {
			continue
		}
		t.Fatalf("partial reason does not name the configurations that did not run: %q", run.PartialReason)
	}
}

func TestUnaccountedSpendStopsAPaidRun(t *testing.T) {
	t.Parallel()
	// `kno value --json` reports no spend field as of v0.1.2. For a paid
	// agent that is unaccounted, not free, and the run must stop rather than
	// book it as zero against the aggregate cap.
	m := &Matrix{
		SchemaVersion: 1,
		Defaults:      Defaults{Agent: "openai:gpt-not-real", Repetitions: 1},
		BaseCell:      Cell{Assets: 1, Cases: 1, Concurrency: 1},
		Configurations: []Config{{
			ID: "paid", Assets: 1, Cases: 1, Concurrency: 1,
			Stages: []string{"baseline", "value"}, Repetitions: 1,
			MaxCostUSD: 10, MaxCalls: 100,
		}},
	}
	ex := &fakeExec{docs: map[string]string{
		"baseline": `{"run_id":"b","spent_usd":"$0.10"}`,
		"value":    `{"run_id":"v"}`,
	}}
	r := &Runner{Bin: "unused", Matrix: m, Track: TrackDryRun, WorkDir: t.TempDir(), BudgetUSD: 100, Executor: ex}
	run, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !run.Partial {
		t.Fatal("a run that could not account for spend must be marked partial")
	}
	if !strings.Contains(run.PartialReason, "reports no spend") {
		t.Fatalf("partial reason = %q, want it to name the unaccounted spend", run.PartialReason)
	}
}

func TestFakeAgentNeedsNoCapsAndIsNotCharged(t *testing.T) {
	t.Parallel()
	m := &Matrix{
		SchemaVersion: 1,
		Defaults:      Defaults{Agent: FakeAgent, Repetitions: 1},
		BaseCell:      Cell{Assets: 1, Cases: 1, Concurrency: 1},
		Configurations: []Config{{
			ID: "free", Assets: 1, Cases: 1, Concurrency: 1,
			Stages: []string{"baseline", "value", "select", "export"}, Repetitions: 1,
		}},
	}
	ex := &fakeExec{docs: map[string]string{"baseline": `{"run_id":"b","spent_usd":"$0.00"}`}}
	r := &Runner{Bin: "unused", Matrix: m, Track: TrackLocal, WorkDir: t.TempDir(), Executor: ex}
	run, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if run.Partial {
		t.Fatalf("unexpected partial run: %s", run.PartialReason)
	}
	if len(ex.calls) != 4 {
		t.Fatalf("drove %v, want all four stages", ex.calls)
	}
	if run.Citable {
		t.Fatal("no run this repository can currently produce is citable")
	}
}

func TestResultWriterRefusesToOverwrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run := &Run{SchemaVersion: ResultSchemaVersion, Track: TrackLocal, RunID: "abc", Kno: KnoBuild{Version: "v0.0.0"}}
	path, err := run.Write(dir)
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat: %v", err)
	}
	if _, err := run.Write(dir); err == nil {
		t.Fatal("the writer overwrote an existing result; results are append-only")
	}
	blob, err := os.ReadFile(path) //nolint:gosec // test fixture.
	if err != nil {
		t.Fatal(err)
	}
	var back Run
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
}
