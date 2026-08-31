#!/usr/bin/env bash
# Fail on any modification or deletion under results/. Only additions pass.
#
# This is the CI half of the append-only rule. The other half is a GitHub
# Ruleset with an empty bypass list, which is not applied yet — see
# METHODOLOGY.md section 4, which states plainly what the guarantee is and
# is not.
set -euo pipefail

BASE="${1:-origin/main}"

if ! git rev-parse --verify --quiet "$BASE" >/dev/null; then
  echo "append-only: no base ref '$BASE' to compare against; skipping" >&2
  exit 0
fi

# M(odified), D(eleted), R(enamed), C(opied) under results/ are all violations.
# results/latest.json is a *derived* artifact, regenerated on every run and
# diffed by CI against a fresh generation. It is carved out by name, and the
# carve-out is stated in METHODOLOGY.md section 5 rather than left implicit.
violations="$(git diff --name-status --diff-filter=MDRC "$BASE"...HEAD -- results/ ':(exclude)results/latest.json' || true)"

if [ -n "$violations" ]; then
  echo "append-only: results/ may only gain files. These were modified or removed:" >&2
  echo "$violations" >&2
  echo >&2
  echo "If a result is wrong, publish a new result and a note. Never edit history." >&2
  exit 1
fi

echo "append-only: results/ additions only"
