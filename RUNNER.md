# The measuring machine

Every published figure is a statement about software *and* about the machine that ran it. This
file is the machine's record. Its history is the machine's history.

## Current state: there is no measurement machine

**No dedicated hardware has been ordered, rented, or budgeted. No hosting contract exists.**
Nothing in this repository implies a purchase, and nothing in it should be read as a request for
one.

Measurements run on **GitHub-hosted `ubuntu-latest`**, which is a different physical machine every
time. That is not a machine you can characterize — it is a fleet — so every result produced on it
is recorded with `machine.citable: false`, and the summarizer's citability gate reads that field.
This is why `results/latest.json` contains no citable figure and will contain none until this file
says otherwise.

| Field | Value |
|---|---|
| Runner | `github-hosted:ubuntu-latest` (ephemeral, shared, CPU model varies per run) |
| `machine_id` | recorded per run; **expected to differ every run**, which is the finding, not a bug |
| CPU model / cores / RAM / kernel | recorded per run from the host, whatever it happens to be |
| CPU governor | recorded when `/sys` exposes it |
| Ambient temperature, per-core frequency | **not recorded** — hosted runners do not expose them, and a field that cannot be filled is not promised |
| Citable | **no** |

## The bootstrap gate

The dedicated-machine question is **not** open for a decision yet. It is gated on data this
repository does not have:

1. This harness runs on `ubuntu-latest`, on a schedule, for **at least four weeks**.
2. Results accumulate under `results/bootstrap/`. Nothing about kno is published from them.
3. The **between-run** coefficient of variation for the base cell and the N-sweep is then a
   measured number in this repository, with an `n` and a spread.
4. That number decides it, **in either direction**:
   - **CV at or under 5%** → no machine is bought. This repository ships on hosted runners with
     its measured spread published, and the sections below are simply unnecessary.
   - **CV above 5%** → the bootstrap data is the justification, and any provisioning request must
     cite it **by number**. A hardware request with no committed bootstrap data is rejected on
     process, not on merit.

The reason the gate exists: the "30–50% shared-runner variance" figure that would otherwise
justify the spend comes from general third-party reports, not from one measurement of *this*
workload on *this* fleet.

## Four decisions that must be made by a human before anything is spent

None of these has an owner today. They are not fields with placeholder values — a placeholder here
would be a decision nobody made, wearing the clothes of one. **A lint will require all four to be
filled with real values before any provisioning happens; that lint is deferred until there is
something for it to check.**

| Decision | What it requires | Status |
|---|---|---|
| **Provisioner** | A named person who holds the hosting account and the runner registration token. Not "the maintainers". | **not decided** |
| **Monthly cost** | A dollar figure for a specific machine at a specific provider, plus who pays it and out of which budget. A plan that cannot state the number is not ready to ask for it. | **not decided** |
| **Pager** | A named person notified when the box drifts: a missed scheduled run, a mismatch against this file, a kernel or microcode update, or a CV that climbs across releases. Drift here is silent by construction — nobody notices a nightly that did not happen. | **not decided** |
| **Decommission trigger** | A condition that fires *without the hardware dying*. The plan's committed form: two consecutive quarters in which no figure from this repository is cited by `kno-www` or the kno README, **or** the arrival of a hosted option whose measured spread sits under the 5% threshold. | **not decided** |

A machine that nobody named, nobody budgeted, and nobody agreed on a condition for switching off
is how a research rig becomes a permanent unowned bill that outlives its own justification.

## If a machine is ever provisioned

Recorded here for the reader's benefit, and binding on whoever does it — not a schedule, and not a
commitment that it will happen:

- Repository-scoped self-hosted runner, on this repository only, never organization-scoped, so a
  compromise cannot pivot into `uknoAI/kno`.
- The measurement workflow triggers on `schedule`, `workflow_dispatch` and `repository_dispatch`
  **only** — never `pull_request` or `pull_request_target`. `knobench lint` asserts this
  mechanically over every workflow file.
- No provider credential ever reaches the machine. Live-cost runs, if they ever happen, run on
  hosted runners behind an environment approval, because their output is dollars and does not need
  timing stability.
- This file records CPU model, core count, RAM, kernel, distro, disk type and `machine_id`, and a
  dated line for every kernel or firmware change. Results before and after such a change carry a
  host-config epoch the summarizer surfaces on any series spanning the boundary.
- A replacement machine gets a **new `machine_id` and starts a new series**. Old results are never
  re-labelled or rescaled. Where both machines can run one release, the cross-calibration factor is
  reported and never silently applied.

## Machine log

| Date | Change | Recorded by |
|---|---|---|
| 2026-08-31 | Repository bootstrapped. No measurement machine exists. Hosted `ubuntu-latest` only, nothing citable. | bootstrap commit |
