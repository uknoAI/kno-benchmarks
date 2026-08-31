// Package harness drives a released kno binary as a subprocess and records
// what it measured.
//
// It contains no Go benchmark. That is deliberate: uknoAI/kno's `make
// bench-diff` is a tripwire that greps `^func Benchmark` in `*_test.go` and
// hard-fails the moment one appears. This repository measures whole stage
// invocations of a shipped artifact from the outside, at a different altitude,
// and must not disturb that gate.
package harness

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// FakeAgent is the only agent this repository runs without a human in the
// loop. It is local, deterministic and free, so a run of it costs nothing and
// measures engine orchestration rather than someone else's queue depth.
const FakeAgent = "fake:"

// Matrix is matrix.yaml: the configurations to measure, committed before any
// data exists so that nobody can decide after seeing the numbers which
// configurations they meant to run.
type Matrix struct {
	SchemaVersion  int      `yaml:"schema_version"`
	Track          string   `yaml:"track"`
	Defaults       Defaults `yaml:"defaults"`
	BaseCell       Cell     `yaml:"base_cell"`
	Configurations []Config `yaml:"configurations"`

	// SHA256 is the hex digest of the file this Matrix was decoded from. It
	// travels with every result: a figure whose matrix hash is not in the
	// repository's history is not citable.
	SHA256 string `yaml:"-"`
}

// Defaults are the per-configuration values a configuration may override.
type Defaults struct {
	Agent       string `yaml:"agent"`
	Goal        string `yaml:"goal"`
	Repetitions int    `yaml:"repetitions"`
	Warmups     int    `yaml:"warmups"`
}

// Cell is a point in the (Assets, Cases, Concurrency) space.
type Cell struct {
	Assets      int `yaml:"assets"`
	Cases       int `yaml:"cases"`
	Concurrency int `yaml:"concurrency"`
}

// Config is one measured configuration: a cell, the stages to drive over it,
// and how many times.
type Config struct {
	ID          string   `yaml:"id"`
	Axis        string   `yaml:"axis"`
	Assets      int      `yaml:"assets"`
	Cases       int      `yaml:"cases"`
	Concurrency int      `yaml:"concurrency"`
	Stages      []string `yaml:"stages"`
	Agent       string   `yaml:"agent"`
	Repetitions int      `yaml:"repetitions"`
	Note        string   `yaml:"note"`

	// WarmupsSet is a pointer so that an explicit `warmups: 0` is honoured
	// rather than silently replaced by the matrix default. The memory probe
	// asks for no warm-up on purpose, and a matrix that says 0 while the
	// harness runs 1 would make the committed file a description of something
	// else.
	WarmupsSet *int `yaml:"warmups"`
	// Warmups is the resolved count, filled by Resolve.
	Warmups int `yaml:"-"`

	// Deferred configurations are carried in the file rather than deleted, so
	// that an omission from the published set is visible instead of silent.
	Deferred       bool   `yaml:"deferred"`
	DeferredReason string `yaml:"deferred_reason"`

	// MaxCostUSD and MaxCalls are the two per-invocation ceilings from the
	// plan's cost-control layer 1. They are required for any agent other than
	// fake: and meaningless for fake:, which spends nothing.
	MaxCostUSD float64 `yaml:"max_cost_usd"`
	MaxCalls   int     `yaml:"max_calls"`
}

// KnownStages are the stages kno v0.1.2 actually ships. `validate` is absent
// on purpose: there is no `kno validate` command, and a benchmark of an
// unimplemented stage would be fiction.
var KnownStages = map[string]bool{
	"baseline": true,
	"value":    true,
	"select":   true,
	"export":   true,
}

// LoadMatrix reads and validates a matrix file, recording its digest.
func LoadMatrix(path string) (*Matrix, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the path is an operator-supplied matrix file.
	if err != nil {
		return nil, fmt.Errorf("read matrix %s: %w", path, err)
	}
	sum := sha256.Sum256(raw)

	var m Matrix
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse matrix %s: %w", path, err)
	}
	m.SHA256 = hex.EncodeToString(sum[:])

	if err := m.validate(); err != nil {
		return nil, fmt.Errorf("matrix %s: %w", path, err)
	}
	return &m, nil
}

func (m *Matrix) validate() error {
	if m.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schema_version %d", m.SchemaVersion)
	}
	if len(m.Configurations) == 0 {
		return fmt.Errorf("no configurations")
	}
	seen := map[string]bool{}
	for i := range m.Configurations {
		c := m.Resolve(&m.Configurations[i])
		if c.ID == "" {
			return fmt.Errorf("configuration %d has no id", i)
		}
		if seen[c.ID] {
			return fmt.Errorf("duplicate configuration id %q", c.ID)
		}
		seen[c.ID] = true
		if len(c.Stages) == 0 {
			return fmt.Errorf("configuration %q drives no stage", c.ID)
		}
		for _, s := range c.Stages {
			if !KnownStages[s] {
				return fmt.Errorf("configuration %q names unknown stage %q", c.ID, s)
			}
		}
		if c.Deferred && c.DeferredReason == "" {
			return fmt.Errorf("configuration %q is deferred with no reason", c.ID)
		}
		if c.Deferred {
			continue
		}
		if c.Cases < 1 || c.Assets < 1 || c.Concurrency < 1 {
			return fmt.Errorf("configuration %q has a non-positive dimension", c.ID)
		}
		if c.Repetitions < 1 {
			return fmt.Errorf("configuration %q has no repetitions", c.ID)
		}
		// Cost-control layer 1, checked at load rather than at spend time: a
		// configuration that can reach a provider carries both ceilings, in
		// two different units, or it does not load at all.
		if c.Agent != FakeAgent {
			if c.MaxCostUSD <= 0 || c.MaxCalls <= 0 {
				return fmt.Errorf("configuration %q names agent %q without both max_cost_usd and max_calls", c.ID, c.Agent)
			}
		}
	}
	return nil
}

// Resolve returns a copy of c with matrix defaults filled in.
func (m *Matrix) Resolve(c *Config) Config {
	out := *c
	if out.Agent == "" {
		out.Agent = m.Defaults.Agent
	}
	if out.Agent == "" {
		out.Agent = FakeAgent
	}
	if out.Repetitions == 0 {
		out.Repetitions = m.Defaults.Repetitions
	}
	if out.WarmupsSet != nil {
		out.Warmups = *out.WarmupsSet
	} else {
		out.Warmups = m.Defaults.Warmups
	}
	if out.Warmups < 0 {
		out.Warmups = 0
	}
	if out.Assets == 0 {
		out.Assets = m.BaseCell.Assets
	}
	if out.Cases == 0 {
		out.Cases = m.BaseCell.Cases
	}
	if out.Concurrency == 0 {
		out.Concurrency = m.BaseCell.Concurrency
	}
	return out
}

// Active returns the resolved configurations that are not deferred.
func (m *Matrix) Active() []Config {
	var out []Config
	for i := range m.Configurations {
		c := m.Resolve(&m.Configurations[i])
		if c.Deferred {
			continue
		}
		out = append(out, c)
	}
	return out
}

// LintMatrix reports every reason the matrix at path would be rejected,
// including a paid configuration missing either cost ceiling on any stage it
// drives. It returns findings rather than an error so a lint run can report
// all of them at once.
func LintMatrix(path string) []string {
	m, err := LoadMatrix(path)
	if err != nil {
		return []string{fmt.Sprintf("matrix: %v", err)}
	}
	var out []string
	p := NewPaths("/nonexistent")
	ids := RunIDs{Baseline: "b", Value: "v", Select: "s"}
	for _, c := range m.Active() {
		for _, stage := range c.Stages {
			if _, err := BuildArgs(stage, c, p, ids); err != nil {
				out = append(out, fmt.Sprintf("matrix: configuration %q stage %q: %v", c.ID, stage, err))
			}
		}
	}
	return out
}
