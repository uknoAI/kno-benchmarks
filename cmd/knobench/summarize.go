package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/uknoAI/kno-benchmarks/summary"
)

func summarizeCmd(argv []string) error {
	fs := flag.NewFlagSet("summarize", flag.ExitOnError)
	root := fs.String("root", ".", "repository root")
	check := fs.Bool("check", false, "regenerate and fail on a diff, instead of writing")
	if err := fs.Parse(argv); err != nil {
		return err
	}

	rep, err := summary.Build(summary.Options{
		RepoRoot:        *root,
		ResultsDir:      filepath.Join(*root, "results"),
		MethodologyPath: "METHODOLOGY.md",
	})
	if err != nil {
		return err
	}
	if errs := rep.Validate(); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "schema check: %v\n", e)
		}
		return fmt.Errorf("%d figure(s) failed the schema check", len(errs))
	}

	md := []byte(rep.Markdown())
	latest, err := rep.LatestJSON()
	if err != nil {
		return err
	}
	targets := map[string][]byte{
		filepath.Join(*root, "SUMMARY.md"):             md,
		filepath.Join(*root, "results", "latest.json"): latest,
	}

	if *check {
		for path, want := range targets {
			got, rerr := os.ReadFile(path) //nolint:gosec // repository-relative paths.
			if rerr != nil {
				return fmt.Errorf("read %s: %w (run `knobench summarize` and commit the result)", path, rerr)
			}
			if string(got) != string(want) {
				return fmt.Errorf("%s is out of date or hand-edited: it is generated, and CI regenerates and diffs it", path)
			}
		}
		fmt.Fprintln(os.Stderr, "generated artifacts are up to date")
		return nil
	}

	for path, blob := range targets {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, blob, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", path)
	}
	return nil
}
