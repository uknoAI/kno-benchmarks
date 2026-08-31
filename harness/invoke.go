package harness

import (
	"fmt"
	"path/filepath"
	"strconv"
)

// StageArgs is the argument vector for one stage invocation, and the only
// place a kno command line is constructed.
//
// It refuses to build a command for a non-fake agent that is missing either
// cost ceiling. Acceptance criterion 9 asks for a lint over the harness that
// fails when either flag is absent from an invocation naming a non-`fake:`
// agent; making the builder itself refuse is stronger than a lint, because
// there is no second code path that could bypass it.
type StageArgs struct {
	Stage string
	Args  []string
}

// Paths are the generated inputs and the store for one repetition.
type Paths struct {
	Dir   string
	Cases string
	Pool  string
	DB    string
	Out   string
}

// NewPaths derives the standard file layout under dir.
func NewPaths(dir string) Paths {
	return Paths{
		Dir:   dir,
		Cases: filepath.Join(dir, "cases.jsonl"),
		Pool:  filepath.Join(dir, "pool.jsonl"),
		DB:    filepath.Join(dir, "kno.db"),
		Out:   filepath.Join(dir, "pack.md"),
	}
}

// RunIDs carries the identifiers a later stage needs from an earlier one.
type RunIDs struct {
	Baseline string
	Value    string
	Select   string
}

// BuildArgs returns the argument vector for one stage of one configuration.
func BuildArgs(stage string, c Config, p Paths, ids RunIDs) (StageArgs, error) {
	if !KnownStages[stage] {
		return StageArgs{}, fmt.Errorf("unknown stage %q", stage)
	}

	var args []string
	switch stage {
	case "baseline":
		args = []string{
			"baseline",
			"--evals", p.Cases,
			"--agent", c.Agent,
			"--concurrency", strconv.Itoa(c.Concurrency),
			"--db", p.DB,
			"--yes", "--json",
		}
	case "value":
		if ids.Baseline == "" {
			return StageArgs{}, fmt.Errorf("value needs a baseline run id")
		}
		args = []string{
			"value",
			"--evals", p.Cases,
			"--pool", p.Pool,
			"--baseline-run-id", ids.Baseline,
			"--agent", c.Agent,
			"--concurrency", strconv.Itoa(c.Concurrency),
			"--db", p.DB,
			"--yes", "--json",
		}
	case "select":
		if ids.Value == "" {
			return StageArgs{}, fmt.Errorf("select needs a value run id")
		}
		// select makes no agent call and reads no evals, so it carries no
		// agent flag and no cost ceiling: there is nothing here to spend.
		return StageArgs{Stage: stage, Args: []string{
			"select",
			"--value-run-id", ids.Value,
			"--pool", p.Pool,
			"--max-context-tokens", "10000",
			"--db", p.DB,
			"--json",
		}}, nil
	case "export":
		if ids.Select == "" {
			return StageArgs{}, fmt.Errorf("export needs a select run id")
		}
		return StageArgs{Stage: stage, Args: []string{
			"export",
			"--select-run-id", ids.Select,
			"--pool", p.Pool,
			"--destination", "context",
			"--out", p.Out,
			"--force",
			"--db", p.DB,
			"--json",
		}}, nil
	}

	// Cost-control layer 1. Two ceilings in different units, on every
	// invocation that can reach a provider: a wrong price in the table makes
	// the dollar cap wrong while the call cap stays right.
	if c.Agent != FakeAgent {
		if c.MaxCostUSD <= 0 {
			return StageArgs{}, fmt.Errorf("stage %q names agent %q without --max-cost-usd", stage, c.Agent)
		}
		if c.MaxCalls <= 0 {
			return StageArgs{}, fmt.Errorf("stage %q names agent %q without --max-calls", stage, c.Agent)
		}
		args = append(args,
			"--max-cost-usd", strconv.FormatFloat(c.MaxCostUSD, 'f', -1, 64),
			"--max-calls", strconv.Itoa(c.MaxCalls),
		)
	}
	return StageArgs{Stage: stage, Args: args}, nil
}
