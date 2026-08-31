package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowLintCatchesASelfHostedForkPRJob(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		body    string
		wantHit string
	}{
		{
			name: "self-hosted reachable from pull_request",
			body: `
on: [pull_request]
jobs:
  measure:
    runs-on: [self-hosted, kno-bench]
    steps: [{run: "true"}]
`,
			wantHit: "self-hosted",
		},
		{
			name: "self-hosted reachable from pull_request_target",
			body: `
on:
  pull_request_target:
jobs:
  measure:
    runs-on: [self-hosted, kno-bench]
    steps: [{run: "true"}]
`,
			wantHit: "self-hosted",
		},
		{
			name: "provider secret with no environment gate",
			body: `
on:
  schedule: [{cron: "0 0 * * *"}]
jobs:
  live:
    runs-on: ubuntu-latest
    steps:
      - run: kno baseline
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
`,
			wantHit: "environment",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			findings := LintWorkflow("test.yml", []byte(tc.body))
			if len(findings) == 0 {
				t.Fatalf("lint found nothing in:\n%s", tc.body)
			}
			joined := strings.Join(findings, "\n")
			if !strings.Contains(joined, tc.wantHit) {
				t.Fatalf("findings %q do not mention %q", joined, tc.wantHit)
			}
		})
	}
}

func TestWorkflowLintAcceptsTheSafeShapes(t *testing.T) {
	t.Parallel()
	ok := []string{
		// Self-hosted, but only on schedule and dispatch.
		`
on:
  schedule: [{cron: "0 0 * * *"}]
  workflow_dispatch:
jobs:
  measure:
    runs-on: [self-hosted, kno-bench]
    steps: [{run: "true"}]
`,
		// A provider secret behind an environment with required reviewers.
		`
on:
  workflow_dispatch:
jobs:
  live:
    runs-on: ubuntu-latest
    environment: bench-live
    steps:
      - run: kno baseline
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
`,
		// Hosted PR validation with no secret at all.
		`
on: [pull_request]
jobs:
  validate:
    runs-on: ubuntu-latest
    steps: [{run: "go test ./..."}]
`,
	}
	for i, body := range ok {
		if findings := LintWorkflow("test.yml", []byte(body)); len(findings) > 0 {
			t.Fatalf("case %d wrongly flagged: %v", i, findings)
		}
	}
}

func TestCommittedWorkflowsPassTheirOwnLint(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "..", ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	if len(entries) == 0 {
		t.Fatal("no workflows to lint")
	}
	for _, e := range entries {
		blob, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			t.Fatal(rerr)
		}
		if findings := LintWorkflow(e.Name(), blob); len(findings) > 0 {
			t.Fatalf("%s: %v", e.Name(), findings)
		}
	}
}
