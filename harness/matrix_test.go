package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMatrix(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "matrix.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCommittedMatrixLoadsAndLintsClean(t *testing.T) {
	t.Parallel()
	m, err := LoadMatrix("../matrix.yaml")
	if err != nil {
		t.Fatalf("the committed matrix does not load: %v", err)
	}
	if len(m.SHA256) != 64 {
		t.Fatalf("matrix digest %q is not a sha256", m.SHA256)
	}
	if findings := LintMatrix("../matrix.yaml"); len(findings) > 0 {
		t.Fatalf("committed matrix has lint findings: %v", findings)
	}
	// The stage set must never name a stage kno does not ship. `validate` is
	// the one that will be tempting the moment it exists upstream.
	for _, c := range m.Active() {
		for _, s := range c.Stages {
			if !KnownStages[s] {
				t.Fatalf("configuration %q names unknown stage %q", c.ID, s)
			}
		}
	}
}

func TestMatrixRejectsAPaidConfigurationWithoutCeilings(t *testing.T) {
	t.Parallel()
	path := writeMatrix(t, `
schema_version: 1
defaults: {agent: "openai:gpt-not-real", repetitions: 1}
base_cell: {assets: 1, cases: 1, concurrency: 1}
configurations:
  - id: paid
    assets: 1
    cases: 1
    concurrency: 1
    stages: [baseline]
`)
	_, err := LoadMatrix(path)
	if err == nil {
		t.Fatal("a paid configuration loaded without either cost ceiling")
	}
	if !strings.Contains(err.Error(), "max_cost_usd") {
		t.Fatalf("error %q does not name the missing ceiling", err)
	}
}

func TestDeferredConfigurationNeedsAReason(t *testing.T) {
	t.Parallel()
	path := writeMatrix(t, `
schema_version: 1
defaults: {agent: "fake:", repetitions: 1}
base_cell: {assets: 1, cases: 1, concurrency: 1}
configurations:
  - id: skipped
    assets: 1
    cases: 1
    concurrency: 1
    stages: [baseline]
    deferred: true
`)
	if _, err := LoadMatrix(path); err == nil {
		t.Fatal("a deferred configuration loaded with no stated reason; an omission must be visible")
	}
}

func TestExplicitZeroWarmupsIsHonoured(t *testing.T) {
	t.Parallel()
	path := writeMatrix(t, `
schema_version: 1
defaults: {agent: "fake:", repetitions: 7, warmups: 1}
base_cell: {assets: 1, cases: 1, concurrency: 1}
configurations:
  - id: inherits
    assets: 1
    cases: 1
    concurrency: 1
    stages: [baseline]
  - id: explicit-zero
    assets: 1
    cases: 1
    concurrency: 1
    stages: [baseline]
    warmups: 0
`)
	m, err := LoadMatrix(path)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, c := range m.Active() {
		got[c.ID] = c.Warmups
	}
	if got["inherits"] != 1 {
		t.Fatalf("a configuration with no warmups key got %d, want the default 1", got["inherits"])
	}
	// The memory probe asks for no warm-up on purpose. A matrix that says 0
	// while the harness runs 1 would make the committed file a description of
	// something else.
	if got["explicit-zero"] != 0 {
		t.Fatalf("an explicit warmups: 0 got %d, want 0", got["explicit-zero"])
	}
}

func TestCommittedMemoryProbeRunsNoWarmup(t *testing.T) {
	t.Parallel()
	m, err := LoadMatrix("../matrix.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range m.Active() {
		if c.ID != "mem-1m" {
			continue
		}
		if c.Warmups != 0 {
			t.Fatalf("mem-1m resolved to %d warmups; matrix.yaml commits 0", c.Warmups)
		}
		if c.Repetitions != 3 {
			t.Fatalf("mem-1m resolved to %d repetitions; matrix.yaml commits 3", c.Repetitions)
		}
		return
	}
	t.Fatal("mem-1m is not in the committed matrix")
}
