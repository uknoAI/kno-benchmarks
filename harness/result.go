package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ResultSchemaVersion is this repository's own schema version. It is not
// kno's: kno-benchmarks defines no proto message and imports no generated
// package. Breaking this shape is a kno-benchmarks concern.
const ResultSchemaVersion = 1

// Track separates what a result is for. Nothing is citable during the
// bootstrap, and the field says so rather than leaving it to a reader.
const (
	// TrackBootstrap is the four-week hosted-runner measurement whose whole
	// purpose is to measure the runner's own variance. It publishes no figure
	// about kno.
	TrackBootstrap = "bootstrap"
	// TrackLocal is a developer's machine. Recorded, never citable.
	TrackLocal = "local"
	// TrackDryRun is PR CI's proof that the harness executes. It measures
	// nothing and is excluded from every summary.
	TrackDryRun = "dry-run"
)

// KnoBuild identifies exactly which artifact was measured.
type KnoBuild struct {
	Version       string `json:"version"`
	VersionOutput string `json:"version_output"`
	Archive       string `json:"archive,omitempty"`
	ArchiveSHA256 string `json:"archive_sha256,omitempty"`
	// ChecksumsVerified is true only when ArchiveSHA256 was found in the
	// release's published checksums.txt. A result that measured a binary we
	// cannot tie to a published artifact is not citable, and this is the field
	// that decides it.
	ChecksumsVerified bool   `json:"checksums_verified"`
	ChecksumSource    string `json:"checksum_source,omitempty"`
}

// StageResult is one measured invocation of one kno stage.
type StageResult struct {
	Stage        string  `json:"stage"`
	WallMS       float64 `json:"wall_ms"`
	PeakRSSBytes int64   `json:"peak_rss_bytes"`
	ExitCode     int     `json:"exit_code"`
	KnoRunID     string  `json:"kno_run_id,omitempty"`
	// SpendUSD is nil when the stage's --json document reports no spend
	// field. As of kno v0.1.2 only `baseline` reports one; `value` does not.
	// A nil here is the difference between "free" and "unaccounted", and the
	// harness refuses to run a paid agent through a stage that returns nil.
	SpendUSD *float64 `json:"spend_usd"`
	Error    string   `json:"error,omitempty"`
}

// Repetition is one pass over one configuration.
type Repetition struct {
	ConfigID    string        `json:"config_id"`
	Axis        string        `json:"axis"`
	Rep         int           `json:"rep"`
	Warmup      bool          `json:"warmup"`
	Assets      int           `json:"assets"`
	Cases       int           `json:"cases"`
	Concurrency int           `json:"concurrency"`
	Agent       string        `json:"agent"`
	StartedAt   time.Time     `json:"started_at"`
	HostBefore  HostState     `json:"host_before"`
	HostAfter   HostState     `json:"host_after"`
	Stages      []StageResult `json:"stages"`
	Error       string        `json:"error,omitempty"`
}

// Run is one execution of the matrix: the unit that is written to results/
// and never modified afterwards.
type Run struct {
	SchemaVersion int    `json:"schema_version"`
	Track         string `json:"track"`
	RunID         string `json:"run_id"`
	// Citable is false in every result this repository can currently produce.
	// The bootstrap measures the runner, not kno; the gate in RUNNER.md has
	// not cleared; and a field that defaults to true would be a claim.
	Citable        bool         `json:"citable"`
	CitableReason  string       `json:"citable_reason"`
	DryRun         bool         `json:"dry_run"`
	Partial        bool         `json:"partial"`
	PartialReason  string       `json:"partial_reason,omitempty"`
	BudgetAborted  bool         `json:"budget_aborted"`
	BudgetUSD      float64      `json:"budget_usd"`
	SpentUSD       float64      `json:"spent_usd"`
	HarnessVersion string       `json:"harness_version"`
	MatrixSHA256   string       `json:"matrix_sha256"`
	StartedAt      time.Time    `json:"started_at"`
	FinishedAt     time.Time    `json:"finished_at"`
	Kno            KnoBuild     `json:"kno"`
	Machine        Machine      `json:"machine"`
	Repetitions    []Repetition `json:"repetitions"`
}

// Write appends the run to the results tree. It refuses to overwrite: the
// append-only property is enforced in CI against the git tree, and the writer
// declining to clobber is the cheap half of the same rule.
func (r *Run) Write(resultsDir string) (string, error) {
	version := r.Kno.Version
	if version == "" {
		version = "unknown-version"
	}
	dir := filepath.Join(resultsDir, r.Track, version)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	name := fmt.Sprintf("%s-%s.json", r.StartedAt.UTC().Format("20060102T150405Z"), r.RunID)
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("refusing to overwrite existing result %s", path)
	}
	blob, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode result: %w", err)
	}
	blob = append(blob, '\n')
	if err := os.WriteFile(path, blob, 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}
