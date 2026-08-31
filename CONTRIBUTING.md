# Contributing

This repository publishes numbers, so the contribution rules are mostly about not letting a number
mean more than it should.

## Sign your commits off

Every commit needs a Developer Certificate of Origin sign-off:

```bash
git commit -s -m "feat: ..."
```

Apache-2.0, DCO, no CLA — same as `uknoAI/kno`.

## The three rules that will get a PR rejected

1. **No number without `n` and a spread.** There is no code path that emits a bare mean, and adding
   one is the change most likely to be reverted on sight.
2. **No exclusion whose rule postdates the data.** Exclusion rules live in `METHODOLOGY.md` with
   IDs, and the summarizer dates each rule from git history. You may add a rule; it will apply to
   future observations only. That is deliberate and it is not negotiable.
3. **No `func Benchmark` in this repository.** `uknoAI/kno`'s `make bench-diff` is a tripwire that
   hard-fails the moment one appears *there*, and the separation between a per-PR micro gate and
   this macro instrument is the reason this repository exists separately. Adding a Go benchmark
   here would blur exactly the distinction `METHODOLOGY.md` §1 draws.

## Working on the harness

```bash
go test ./...                      # unit tests; no network, no binary, no money
go run ./cmd/knobench lint         # matrix + workflow rules
go run ./cmd/knobench summarize    # regenerate SUMMARY.md and results/latest.json
go run ./cmd/knobench summarize --check   # what CI runs; fails on a diff
```

To run the harness for real you need a released `kno` binary. Point `--archive` and `--checksums`
at the release archive and its published `checksums.txt`; the harness refuses to measure a binary
whose digest is not in there, because "we measured the thing users download" should be a checkable
statement rather than an assertion.

Use `--track local` for anything on your own machine. Local results are recorded and are never
citable.

## `results/` is append-only

Add files; never modify or delete one. CI fails on any modification or deletion under `results/`.
This is a norm the git history exposes, not a cryptographic guarantee — `METHODOLOGY.md` §4 says
exactly what it is and is not.

If a result is wrong, the fix is a new result and a note, never an edit. A benchmark repository
that quietly corrects its own history is worse than one with no history.

## `SUMMARY.md` and `results/latest.json` are generated

Do not edit them. CI regenerates and diffs; a hand edit fails the build. If a figure looks wrong,
the bug is in `summary/`, and the fix ships with the test that would have caught it.

## Never spend money in CI

No workflow here may reach a provider key without a GitHub environment approval, and no workflow
that could run on a self-hosted runner may be triggerable by `pull_request` or
`pull_request_target`. `knobench lint` asserts both mechanically; do not weaken it to make
something pass.
