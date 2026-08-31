package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/uknoAI/kno-benchmarks/harness"
)

func runCmd(ctx context.Context, argv []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	bin := fs.String("bin", "", "path to the released kno binary (required)")
	matrixPath := fs.String("matrix", "matrix.yaml", "path to the committed matrix")
	track := fs.String("track", harness.TrackLocal, "bootstrap | local | dry-run")
	resultsDir := fs.String("results", "results", "append-only results tree")
	workDir := fs.String("workdir", "", "scratch directory for generated inputs (a temp dir by default)")
	runner := fs.String("runner", "", "runner label recorded with the result, e.g. github-hosted:ubuntu-latest (required)")
	budget := fs.Float64("budget-usd", 0, "aggregate run budget; a run whose reported spend exceeds it aborts the rest of the matrix")
	archive := fs.String("archive", "", "path to the release archive the binary came from")
	checksums := fs.String("checksums", "", "path to the release's published checksums.txt")
	keepInputs := fs.Bool("keep-inputs", false, "leave generated Cases and Assets on disk")
	if err := fs.Parse(argv); err != nil {
		return err
	}
	if *bin == "" || *runner == "" {
		fs.Usage()
		return fmt.Errorf("--bin and --runner are required")
	}
	switch *track {
	case harness.TrackBootstrap, harness.TrackLocal, harness.TrackDryRun:
	default:
		return fmt.Errorf("unknown track %q", *track)
	}

	m, err := harness.LoadMatrix(*matrixPath)
	if err != nil {
		return err
	}
	build, err := identify(ctx, *bin, *archive, *checksums)
	if err != nil {
		return err
	}

	dir := *workDir
	if dir == "" {
		dir, err = os.MkdirTemp("", "knobench-")
		if err != nil {
			return fmt.Errorf("create scratch dir: %w", err)
		}
		defer func() { _ = os.RemoveAll(dir) }()
	}

	r := &harness.Runner{
		Bin:        *bin,
		Matrix:     m,
		Track:      *track,
		WorkDir:    dir,
		BudgetUSD:  *budget,
		Machine:    harness.Fingerprint(*runner),
		Kno:        build,
		KeepInputs: *keepInputs,
		Logf:       func(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...) },
	}
	fmt.Fprintf(os.Stderr, "measuring kno %s (%s) on %s, matrix %s\n", build.Version, digestNote(build), *runner, m.SHA256[:12])

	run, err := r.Execute(ctx)
	if err != nil {
		return err
	}
	path, err := run.Write(*resultsDir)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d repetitions, partial=%v, budget-aborted=%v)\n",
		path, len(run.Repetitions), run.Partial, run.BudgetAborted)
	if run.Partial {
		return fmt.Errorf("run did not complete the committed matrix: %s", run.PartialReason)
	}
	return nil
}

func digestNote(b harness.KnoBuild) string {
	if b.ChecksumsVerified {
		return "digest verified against " + b.ChecksumSource
	}
	return "digest NOT verified — this result is not citable"
}

var versionRE = regexp.MustCompile(`(\d+\.\d+\.\d+[^\s)]*)`)

func identify(ctx context.Context, bin, archive, checksums string) (harness.KnoBuild, error) {
	out, err := exec.CommandContext(ctx, bin, "--version").Output() //nolint:gosec // operator-named binary.
	if err != nil {
		return harness.KnoBuild{}, fmt.Errorf("run %s --version: %w", bin, err)
	}
	text := strings.TrimSpace(string(out))
	b := harness.KnoBuild{VersionOutput: text}
	if m := versionRE.FindStringSubmatch(text); m != nil {
		b.Version = "v" + m[1]
	} else {
		b.Version = "unknown-version"
	}

	if archive == "" || checksums == "" {
		return b, nil
	}
	sum, err := fileSHA256(archive)
	if err != nil {
		return b, err
	}
	b.Archive = archive
	b.ArchiveSHA256 = sum
	blob, err := os.ReadFile(checksums) //nolint:gosec // operator-named file.
	if err != nil {
		return b, fmt.Errorf("read %s: %w", checksums, err)
	}
	for line := range strings.SplitSeq(string(blob), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == sum {
			b.ChecksumsVerified = true
			b.ChecksumSource = checksums + " (" + fields[1] + ")"
			break
		}
	}
	if !b.ChecksumsVerified {
		return b, fmt.Errorf("archive %s digest %s is not in %s: refusing to measure a binary that cannot be tied to a published artifact", archive, sum, checksums)
	}
	return b, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // operator-named file.
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck // read-only.
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
