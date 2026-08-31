// Command knobench drives the released kno binary over a committed matrix,
// summarizes the append-only results tree, and lints the repository's own
// safety rules.
//
// It defines no Go benchmark. uknoAI/kno's `make bench-diff` is a tripwire
// that fails the moment `^func Benchmark` appears in a `*_test.go` there;
// this repository measures a shipped artifact from the outside and must leave
// that gate exactly as it found it.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "run":
		err = runCmd(ctx, os.Args[2:])
	case "summarize":
		err = summarizeCmd(os.Args[2:])
	case "lint":
		err = lintCmd(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "knobench: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `knobench — drive a released kno binary over a committed matrix

  knobench run        measure the matrix and append one result file
  knobench summarize  regenerate SUMMARY.md and results/latest.json
  knobench lint       check the matrix and the workflows against this repo's own rules

Run any subcommand with -h for its flags.
`)
}
