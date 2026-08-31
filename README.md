# kno-benchmarks

Measurements of the released [`kno`](https://github.com/uknoAI/kno) binary, and the append-only
record of every measurement ever taken.

## What this repository claims today

**Nothing about kno's performance.** `results/latest.json` contains zero citable figures, and that
is the honest state of this repository, not an oversight.

It is running one experiment, and it is not about kno:

> **How much does a GitHub-hosted runner vary when it measures the same thing twice?**

That question has to be answered before anything else here means anything. A number like "valuing
1 000 Assets took 53 seconds" is a number about a machine as much as about software, and on hosted
CI the machine is a different physical CPU every run. The plan behind this repository was going to
justify buying a dedicated box partly on a "30–50% variance" figure taken from general reports
about shared runners — somebody else's number, about somebody else's workload. Asking for money to
fix a problem sized by a third party's blog post is precisely the epistemic failure `kno` refuses
to commit about your data, committed about our own spending instead.

So: this harness runs on `ubuntu-latest`, on a schedule, for at least four weeks, publishing
nothing about kno. At the end there is a measured coefficient of variation, in this repository,
with an `n` and a spread, and the hardware question gets decided on it — **in either direction**.
If the measured spread is at or under 5%, no machine is bought and this repository ships on hosted
runners with its spread published.

## What this repository explicitly does not claim yet

- **No performance claim about kno.** Not "fast", not "scales linearly", not a throughput number.
- **No scaling exponent.** The N axis has three points in the bootstrap; the fourth (N=10 000) is
  deferred and marked so in `matrix.yaml`.
- **No end-to-end time.** Every figure uses `fake:`, kno's local deterministic agent. It excludes
  provider latency, which dominates every real run. Every figure's `claim` string says so, and a
  schema check fails the build if one does not.
- **No cost figure.** No real-provider run has happened. None can happen without a human approving
  it, a billing project that does not exist, and a fix to the blocker in `METHODOLOGY.md` §6.
- **No comparison to any other tool.** Not now, not later. We do not run their code on our
  machine, and a competitor benchmark written by us is marketing.
- **No memory verdict.** The 1M-Case probe is measured and published; whether it confirms or
  refutes `CLAUDE.md`'s streaming claim is for the reader and for an issue upstream, not for a
  headline here.
- **No claim to have repaid `docs/debt.md#3` in `uknoAI/kno`.** That entry is about an in-repo
  micro-benchmark regression gate. This repository is a macro longitudinal instrument at a
  different altitude and does not replace it. `make bench-diff` there is untouched — this
  repository contains no `func Benchmark`, by construction.
- **Not "protected by a ruleset".** The GitHub Ruleset with an empty bypass list and required
  signed commits is the intended enforcement of the append-only rule and is **not applied yet**.
  See `METHODOLOGY.md` §4, which also states plainly what that guarantee is and is not.

## How it works

```
matrix.yaml          the configurations, committed before any data exists
harness/             drives the released kno binary as a subprocess
summary/             derives SUMMARY.md and results/latest.json; refuses dishonest figures
results/             append-only: every run ever taken, including the failed ones
METHODOLOGY.md       how it measures, what it excludes, and when each rule was committed
CITATION.md          what a figure from here may and may not be used to say
RUNNER.md            the machine, and the four decisions blocking a better one
SUMMARY.md           generated; CI regenerates and fails on a diff
```

```bash
# Measure. Requires a released kno binary; --archive/--checksums tie it to a published artifact.
go run ./cmd/knobench run \
  --bin ./kno --runner "local:$(uname -m)" --track local \
  --archive kno_0.1.2_linux_amd64.tar.gz --checksums checksums.txt

# Regenerate the derived artifacts (CI runs this with --check).
go run ./cmd/knobench summarize

# Check the matrix and the workflows against this repository's own rules.
go run ./cmd/knobench lint
```

## Why a separate repository

`uknoAI/kno`'s `make bench-diff` is a per-PR tripwire on hot paths inside the module, comparing
against `main`, blocking merges. This is a per-release measurement of a shipped binary, comparing
against its own history, blocking nothing. Both should exist; neither replaces the other. Keeping
them apart is also what lets this repository ever host a dedicated runner without giving `uknoAI/kno`
a fork-PR execution surface it must never have.

## Deferred

Listed rather than assumed. Each of these is in the plan and is **not** in this repository yet:

| Deferred | Blocked on |
|---|---|
| Any citable figure about kno | the bootstrap concluding, and a committed measurement machine in `RUNNER.md` |
| Dedicated runner, `RUNNER.md` hardware fields | four human decisions (see `RUNNER.md`) *and* the bootstrap data |
| Live-provider cost track (n=3, median and range, input/output split) | the human decisions, a billing project, an environment gate, and the `kno value --json` spend blocker |
| GitHub Ruleset JSON + the CI check that diffs it against the live API | repository admin action |
| N=10 000 cell | a machine that can afford it; see `matrix.yaml` |
| Release polling (measure each new tag automatically) | the bootstrap concluding |
| Release-over-release regression detection and the upstream issue it files | a second measured release |
| `kno-www` citation workstream | a citable figure to cite, and an owner |

## License

Apache-2.0. See `LICENSE`. Contributions are DCO-signed; see `CONTRIBUTING.md`.
