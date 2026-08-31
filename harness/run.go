package harness

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// HarnessVersion is stamped into every result so a change in how we measure is
// visible next to the numbers it produced.
const HarnessVersion = "0.1.0-bootstrap"

// ErrFatal marks a condition that must stop the whole sweep rather than fail
// one repetition. A stage that errored is data; a stage whose spend cannot be
// accounted for is a safety failure, and the two must not share a code path.
var ErrFatal = errors.New("fatal")

// ExecResult is one subprocess invocation, measured.
type ExecResult struct {
	Stdout       []byte
	Stderr       []byte
	ExitCode     int
	Wall         time.Duration
	PeakRSSBytes int64
}

// Executor runs one kno invocation. The interface exists so the matrix loop —
// including the aggregate budget abort — is testable with mocked --json
// documents and no network, no binary and no money.
type Executor interface {
	Exec(ctx context.Context, bin string, args []string, dir string) (ExecResult, error)
}

// SubprocessExecutor is the real one: it runs the released binary and reads
// wall-clock and peak RSS from the kernel.
type SubprocessExecutor struct{}

// Exec runs bin with args in dir and returns what it cost.
func (SubprocessExecutor) Exec(ctx context.Context, bin string, args []string, dir string) (ExecResult, error) {
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // bin is the operator-named kno binary.
	cmd.Dir = dir
	// The child gets no ambient credentials. `fake:` needs none, and a
	// long-lived measurement process handing its whole environment to a
	// subprocess is how a provider key ends up in a benchmark run.
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"TMPDIR=" + os.Getenv("TMPDIR"),
	}
	var stdout, stderr capBuffer
	stdout.limit, stderr.limit = 64<<20, 1<<20
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	wall := time.Since(start)

	res := ExecResult{Stdout: stdout.b, Stderr: stderr.b, Wall: wall}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
		if ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
			res.PeakRSSBytes = maxRSSBytes(int64(ru.Maxrss))
		}
	}
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return res, fmt.Errorf("exec %s: %w", bin, err)
	}
	return res, nil
}

type capBuffer struct {
	b     []byte
	limit int
}

func (c *capBuffer) Write(p []byte) (int, error) {
	if len(c.b) < c.limit {
		room := c.limit - len(c.b)
		if room > len(p) {
			room = len(p)
		}
		c.b = append(c.b, p[:room]...)
	}
	return len(p), nil
}

// Runner executes a matrix and produces one Run.
type Runner struct {
	Bin       string
	Matrix    *Matrix
	Track     string
	WorkDir   string
	BudgetUSD float64
	Machine   Machine
	Kno       KnoBuild
	Executor  Executor
	Logf      func(string, ...any)
	// KeepInputs leaves generated Cases and Assets on disk. Off by default:
	// the 1M-Case probe writes ~100 MB.
	KeepInputs bool
}

func (r *Runner) logf(format string, args ...any) {
	if r.Logf != nil {
		r.Logf(format, args...)
	}
}

// Execute drives every active configuration and returns the recorded Run.
//
// It never returns partial data silently: a run that stopped early is marked
// Partial with the reason, because a curve fitted through an incomplete sweep
// has a shape nobody measured.
func (r *Runner) Execute(ctx context.Context) (*Run, error) {
	if r.Executor == nil {
		r.Executor = SubprocessExecutor{}
	}
	id, err := newRunID()
	if err != nil {
		return nil, err
	}
	budget := &Budget{LimitUSD: r.BudgetUSD}

	run := &Run{
		SchemaVersion:  ResultSchemaVersion,
		Track:          r.Track,
		RunID:          id,
		Citable:        false,
		CitableReason:  citableReason(r.Track),
		DryRun:         r.Track == TrackDryRun,
		BudgetUSD:      r.BudgetUSD,
		HarnessVersion: HarnessVersion,
		MatrixSHA256:   r.Matrix.SHA256,
		StartedAt:      time.Now().UTC(),
		Kno:            r.Kno,
		Machine:        r.Machine,
	}

	configs := r.Matrix.Active()
	for i, c := range configs {
		if err := ctx.Err(); err != nil {
			// Cancellation aborts the sweep. Grinding through the remaining
			// configurations to record "context canceled" against each one
			// would fabricate a full-looking result out of a stopped run.
			run.Partial = true
			run.PartialReason = fmt.Sprintf("cancelled: %v; configurations %s did not execute", err, remaining(configs[i:]))
			break
		}
		if budget.Aborted() {
			run.Partial = true
			run.BudgetAborted = true
			run.PartialReason = fmt.Sprintf("aggregate budget $%.2f exceeded; configurations %s onward did not execute", r.BudgetUSD, remaining(configs[i:]))
			break
		}
		if err := r.runConfig(ctx, c, run, budget); err != nil {
			run.Partial = true
			run.PartialReason = fmt.Sprintf("configuration %q failed: %v; configurations %s did not execute", c.ID, err, remaining(configs[i+1:]))
			break
		}
	}
	run.SpentUSD = budget.Spent()
	run.BudgetAborted = budget.Aborted()
	run.FinishedAt = time.Now().UTC()
	return run, nil
}

func citableReason(track string) string {
	switch track {
	case TrackBootstrap:
		return "bootstrap track: measures the hosted runner's own variance, not kno's performance; no figure here is citable"
	case TrackDryRun:
		return "dry-run: proves the harness executes, measures nothing"
	default:
		return "no host in this repository is a committed measurement machine; see RUNNER.md"
	}
}

func remaining(cs []Config) string {
	if len(cs) == 0 {
		return "(none)"
	}
	out := ""
	for i, c := range cs {
		if i > 0 {
			out += ", "
		}
		out += c.ID
	}
	return out
}

func (r *Runner) runConfig(ctx context.Context, c Config, run *Run, budget *Budget) error {
	dir := filepath.Join(r.WorkDir, c.ID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	if !r.KeepInputs {
		defer func() { _ = os.RemoveAll(dir) }()
	}
	p := NewPaths(dir)

	// Inputs are generated once per configuration, not once per repetition:
	// they are a deterministic function of (assets, cases), so regenerating
	// them would add file-system noise to the spread without changing a byte
	// the binary reads.
	r.logf("generating %d cases and %d assets for %s", c.Cases, c.Assets, c.ID)
	if err := GenerateCases(p.Cases, c.Cases); err != nil {
		return err
	}
	if err := GenerateAssets(p.Pool, c.Assets); err != nil {
		return err
	}

	total := c.Warmups + c.Repetitions
	for rep := range total {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("%w: cancelled during %s: %w", ErrFatal, c.ID, err)
		}
		if budget.Aborted() {
			return nil
		}
		warmup := rep < c.Warmups
		rr := Repetition{
			ConfigID:    c.ID,
			Axis:        c.Axis,
			Rep:         rep,
			Warmup:      warmup,
			Assets:      c.Assets,
			Cases:       c.Cases,
			Concurrency: c.Concurrency,
			Agent:       c.Agent,
			StartedAt:   time.Now().UTC(),
			HostBefore:  ReadHostState(),
		}
		// A fresh store per repetition. Reusing one would make repetition 2
		// measure a warm SQLite file and repetition 1 a cold one, and the
		// difference would be published as variance.
		_ = os.Remove(p.DB)
		_ = os.Remove(p.DB + "-wal")
		_ = os.Remove(p.DB + "-shm")

		ids := RunIDs{}
		for _, stage := range c.Stages {
			sr, err := r.runStage(ctx, stage, c, p, &ids, budget)
			rr.Stages = append(rr.Stages, sr)
			if err != nil {
				rr.Error = err.Error()
				if errors.Is(err, ErrFatal) {
					run.Repetitions = append(run.Repetitions, rr)
					return err
				}
				break
			}
			if sr.ExitCode != 0 {
				rr.Error = fmt.Sprintf("stage %s exited %d", stage, sr.ExitCode)
				break
			}
			if budget.Aborted() {
				break
			}
		}
		rr.HostAfter = ReadHostState()
		run.Repetitions = append(run.Repetitions, rr)
		// A failed repetition is published with its error and does not abort
		// the sweep: too few successful repetitions is what disqualifies a
		// configuration, and the summarizer decides that, not the runner.
		r.logf("%s rep %d/%d warmup=%v: %s", c.ID, rep+1, total, warmup, summarize(rr))
	}
	return nil
}

func summarize(rr Repetition) string {
	if rr.Error != "" {
		return "error: " + rr.Error
	}
	out := ""
	for i, s := range rr.Stages {
		if i > 0 {
			out += " "
		}
		out += fmt.Sprintf("%s=%.0fms", s.Stage, s.WallMS)
	}
	return out
}

func (r *Runner) runStage(ctx context.Context, stage string, c Config, p Paths, ids *RunIDs, budget *Budget) (StageResult, error) {
	sa, err := BuildArgs(stage, c, p, *ids)
	if err != nil {
		return StageResult{Stage: stage, Error: err.Error()}, err
	}
	res, err := r.Executor.Exec(ctx, r.Bin, sa.Args, p.Dir)
	sr := StageResult{
		Stage:        stage,
		WallMS:       float64(res.Wall.Nanoseconds()) / 1e6,
		PeakRSSBytes: res.PeakRSSBytes,
		ExitCode:     res.ExitCode,
	}
	if err != nil {
		sr.Error = err.Error()
		return sr, err
	}
	if res.ExitCode != 0 {
		sr.Error = fmt.Sprintf("exit %d: %s", res.ExitCode, tail(res.Stderr, 400))
		return sr, nil
	}

	spend, reported, perr := parseSpend(res.Stdout)
	if perr != nil {
		// A partial parse must never produce a number that looks complete.
		sr.Error = perr.Error()
		return sr, fmt.Errorf("stage %s: %w", stage, perr)
	}
	if reported {
		sr.SpendUSD = &spend
		if budget.Add(spend) {
			return sr, nil
		}
	} else if c.Agent != FakeAgent {
		err := fmt.Errorf("%w: stage %s reports no spend for agent %q: the aggregate cap cannot account for it, so the run stops", ErrFatal, stage, c.Agent)
		sr.Error = err.Error()
		return sr, err
	}

	if id := parseRunID(res.Stdout); id != "" {
		switch stage {
		case "baseline":
			ids.Baseline = id
		case "value":
			ids.Value = id
		case "select":
			ids.Select = id
		}
		sr.KnoRunID = id
	}
	return sr, nil
}

func tail(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return "..." + string(b[len(b)-n:])
}

func newRunID() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate run id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
