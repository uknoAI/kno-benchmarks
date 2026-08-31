# Methodology

This document is the committed record of how this repository measures, what it excludes, and
why. It is an artifact, not prose: the summarizer reads the exclusion-rule table below and
**refuses any exclusion whose rule was committed to git after the observation it would exclude**.
You may exclude on a principle. You may never exclude on an outcome.

Its diff history is part of the published record. If you want to know whether a rule was written
before or after the number it affects, `git log METHODOLOGY.md` is the answer, and the summarizer
consults exactly that.

---

## 1. What is measured

The **released `kno` binary**, driven as a subprocess from the outside. This repository contains
no `func Benchmark` and is not a Go benchmark suite. Two consequences, both deliberate:

- It cannot trip `make bench-diff` in `uknoAI/kno`, which is a tripwire that greps `^func
  Benchmark` in `*_test.go` and hard-fails the moment one appears. That tripwire is a per-PR
  **micro** regression gate against `main`. This is a per-release **macro** longitudinal
  measurement of a shipped artifact against its own history. Different instrument, different
  altitude. **This repository does not repay `docs/debt.md#3` in `uknoAI/kno`, and must not be
  read as doing so.**
- What is measured is what a user downloads. Every result records `kno --version` and, where the
  binary came from a release archive, the SHA-256 of that archive checked against the release's
  published `checksums.txt`. A result whose digest was not matched is recorded with
  `checksums_verified: false` and is not citable.

### The agent is `fake:`

`fake:` is kno's default agent: local, deterministic, and free. With it there is no network, no
provider variance and no money, so what is left is engine orchestration — routing, injection,
scoring, checkpointing, SQLite writes, event emission, iterator plumbing. That is the only thing
kno controls and therefore the only thing kno may make a throughput claim about.

**Every `fake:` figure excludes provider latency, which dominates every real run.** This is the
single most likely way this repository becomes dishonest: not by fabricating a number, but by
quoting a true one in a context that implies an end-to-end time. It is enforced mechanically —
every `fake:` figure's `claim` string must contain the substring "excludes provider latency", and
the schema check fails the build if one does not.

### The stages

`baseline`, `value`, `select`, `export`. **`validate` is excluded because `kno validate` does not
exist** in v0.1.2 — `kno --help` says "validate arrives next". A benchmark of an unimplemented
stage would be fiction.

The four stages are driven as one chained pipeline per repetition, because they depend on each
other: `value` needs a baseline run id, `select` needs a value run id, `export` needs a select run
id. Each stage is timed separately, so **a stage's figure excludes the stages before it**. It does
not exclude the cost of the store those stages populated, which is a real dependency and is not
subtracted.

### The matrix

`matrix.yaml`, committed before any data exists, with its SHA-256 recorded in every result. It is
an **OFAT sweep**: a base cell (N=100 Assets, M=500 Cases, concurrency 8) measured for every
stage, with each axis varied one at a time from that base.

A full cross would be 1008 runs. It is not one, so **interaction effects between N, M and
concurrency are unmeasured**, and any statement derived from this file must name its base —
"varying N at M=500, concurrency 8" — and must never say "across the matrix". The latter would be
false, and it is the kind of false nobody would ever catch.

Deferred cells are carried in `matrix.yaml` with `deferred: true` and a stated reason rather than
deleted, so that a gap in an axis is visible instead of silent.

### Inputs

Cases and Assets are synthetic and are a deterministic function of `(N, M)` alone. Two repetitions
of one configuration therefore feed the binary identical bytes: an input that varied between
repetitions would put its own variance inside the spread this repository exists to characterize.
Inputs are generated once per configuration; the SQLite store is deleted and recreated **per
repetition**, so that repetition 2 does not measure a warm store that repetition 1 built.

---

## 2. Statistics

Every published figure carries **n, median, p25, p75, min, max and the coefficient of variation**.
There is no code path that emits a bare mean. A mean with no spread does not ship.

- **Percentiles** are linear interpolation between order statistics — the definition R calls type
  7 and NumPy uses by default. For sorted `x` of length `n` and quantile `q`: `h = q(n-1)`,
  `lo = floor(h)`, result `x[lo] + (h - lo)(x[lo+1] - x[lo])`. It is named here because an
  off-by-one in a percentile silently changes every published interval, and a reader must be able
  to reproduce the number. Unit tests check it against hand-computed values.
- **CV** is the sample standard deviation (n−1 denominator) over the mean. When the mean is not
  positive, CV is reported as **undefined** and the figure is flagged `unstable` — a meaningless
  CV is not a convenient zero.
- **Two scopes, and the difference is the point of the bootstrap.**
  - *Within-run*: one host measuring one configuration `n` times in a row. The smallest spread
    this setup can produce.
  - *Between-run*: the distribution of per-run **medians** for a configuration. On GitHub-hosted
    runners each run lands on a different physical machine, so this spread is the runner fleet's
    heterogeneity, not the software's. Every between-run figure records how many distinct machine
    fingerprints it spans. **This is the number the hardware question in `RUNNER.md` turns on.**

### Thresholds, committed before any data

| Threshold | Value | Behaviour |
|---|---|---|
| `unstable` | CV > **5%** | The figure is published **flagged**, never dropped. Dropping a noisy cell is cherry-picking with extra steps. |
| `insufficient-n` | fewer than **5** successful measured repetitions, or fewer than all of them for a configuration committed with fewer than 5 | No summary figure is produced. The observations are still published. |
| between-run spread | fewer than **2** runs | No between-run figure: a spread needs at least two observations. |

---

## 3. Exclusion rules

The summarizer looks up each rule ID's **first appearance in this file's git history** and refuses
to apply the rule to any observation recorded before that commit. A rule that is not committed at
all excludes nothing. Both refusals fail in the same direction — they **widen** the published
spread rather than narrowing it — which is the only direction this check is allowed to fail in.

When an exclusion is refused, `SUMMARY.md` says so, per rule, with a count, and the affected
figure carries an `exclusion-refused:<ID>` flag.

| ID | Rule | Applies to |
|---|---|---|
| EX-1 | The first repetition of every configuration is a warm-up. It is excluded from summary statistics and is still published, labelled `warmup`. | every configuration with `warmups > 0` |
| EX-2 | A repetition that did not complete the stage being measured is excluded from that stage's timing statistics. It is still published, with its error text. | every configuration |

There is no third exclusion rule, and adding one is a reviewable PR that changes **nothing
retroactively**, because a rule added today cannot reach data recorded yesterday.

---

## 4. Append-only results — what that guarantee actually is

Every run is committed to `results/`, including failed, partial, budget-aborted and
`unstable`-flagged ones. CI diffs the `results/` tree against the previous commit and fails on any
modification or deletion; only additions pass. The intended enforcement is a GitHub **Ruleset**
scoped to this repository with an **empty bypass list**, required status checks, and **required
signed commits** — not classic branch protection, which is a setting a repository admin edits at
will, so "we do not force-push" would be a promise made by exactly the person most motivated to
break it. A ruleset with no bypass actors applies to admins too, and changes to it land in the
organization audit log, which moves an evasion from *invisible* to *visible*.

**And it is still not a cryptographic guarantee.** An admin can loosen a ruleset. The audit log
records that they did, but a record is a deterrent, not a lock. Signed commits mean a rewritten
history cannot be silently re-attributed. An append-only tree means every result ever pushed
exists in every clone and every fork.

So the honest statement, in plain words:

> **Append-only here is an auditable norm, not a technical impossibility.** It is a rule the
> community can check from git history and from the organization audit log. It is not enforced by
> cryptography and it cannot be. If you do not trust us, clone this repository and diff it
> yourself — that is the actual guarantee on offer, and it is the only kind any self-published
> benchmark can honestly make.

The same caveat applies to the exclusion-rule dating in §3: it reads git commit timestamps, and
git commit timestamps are only as trustworthy as the history they sit in. Signed commits and an
empty-bypass ruleset are what make that history worth reading.

**Status:** the ruleset is **not yet applied**. This repository is at its bootstrap commit; the
ruleset JSON and the CI check that diffs it against the live API are listed in `README.md` as
deferred work. Saying "protected by a ruleset" before the ruleset exists would be the exact
category of error this document is about.

---

## 5. Generated artifacts

`SUMMARY.md` and `results/latest.json` are **derived, never written**. CI regenerates both and
fails on a diff, so a hand-edited summary fails the build. Neither contains a generation
timestamp: a wall-clock stamp would make the regenerate-and-diff check fail on every run and train
reviewers to ignore it.

`results/latest.json` lives under `results/` because that is where the site looks for it, but it is
a derived artifact and is **carved out of the append-only check by name** — it is regenerated on
every run and would otherwise fail the check immediately. The carve-out is stated here rather than
left implicit in a shell script: it is the one path under `results/` that is allowed to change, and
a reader is entitled to know that. Every actual *result* file remains append-only.

---

## 6. Cost control

No real-provider run has happened, and none can happen without a human. Four independent layers
are designed; the ones this repository has code for are layers 1 and 2.

1. **Per-invocation.** Every invocation naming an agent other than `fake:` carries both
   `--max-cost-usd` and `--max-calls` — two ceilings in different units, because a wrong price in
   the table makes the dollar cap wrong while the call cap stays right. This is enforced in the
   command builder, which *refuses to construct* such a command line, rather than in a lint that
   could be bypassed by a second code path.
2. **Per-run aggregate.** The harness sums the spend each `--json` document reports and aborts the
   remaining matrix the moment the total crosses the committed budget. This catches what layer 1
   structurally cannot: twenty individually-capped runs that together cost twenty times the cap.
3. **Provider-side hard limit.** Not configured — there is no billing project, because there is no
   live run.
4. **Human approval** via a GitHub environment with required reviewers. Not configured, for the
   same reason.

**A blocker, recorded because it is load-bearing:** as of kno v0.1.2, `kno value --json` reports
**no spend field** — only `kno baseline --json` carries `spent_usd`. Layer 2 therefore cannot see
value-stage spend at all. The harness treats a missing spend field on a non-`fake:` agent as a
**hard error that stops the run**, rather than as zero, because "free" and "unaccounted" are not
the same number. Until that is resolved upstream, no live-provider run can be reconciled against
an aggregate cap, and this repository will not perform one.

---

## 7. Machines

Every result records a machine fingerprint: OS, architecture, CPU model, core count, total memory,
kernel, CPU governor where the host exposes it, and a `machine_id` — a truncated SHA-256 of the
host's machine identifier rather than the identifier itself, which is stable and comparable
without putting a host identifier in a public repository.

Results carrying different `machine_id` values are **never merged into one within-run series**.
The between-run scope aggregates across machines deliberately and says how many it spanned.

`machine.citable` is `false` in every result this repository can currently produce, and the
summarizer's citability gate depends on it. See `RUNNER.md`.

---

## 8. Ambient conditions

Load average and available memory are recorded before and after every repetition. A repetition run
under load is **not** discarded — there is no exclusion rule for it, and adding one after seeing
data would be refused by §3 — it is published with the load recorded, so a reader can see whether
a wide spread has an explanation.

Ambient temperature and per-core frequency are **not** recorded: GitHub-hosted runners do not
expose them. A field that cannot be filled is not promised.
