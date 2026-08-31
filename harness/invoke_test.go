package harness

import (
	"slices"
	"strings"
	"testing"
)

func TestPaidInvocationRequiresBothCeilings(t *testing.T) {
	t.Parallel()
	p := NewPaths(t.TempDir())
	ids := RunIDs{Baseline: "b", Value: "v", Select: "s"}

	cases := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name:    "no ceilings at all",
			cfg:     Config{ID: "x", Agent: "openai:gpt-not-real", Concurrency: 1},
			wantErr: "--max-cost-usd",
		},
		{
			name:    "dollars but no calls",
			cfg:     Config{ID: "x", Agent: "openai:gpt-not-real", Concurrency: 1, MaxCostUSD: 1},
			wantErr: "--max-calls",
		},
	}
	for _, tc := range cases {
		for _, stage := range []string{"baseline", "value"} {
			t.Run(tc.name+"/"+stage, func(t *testing.T) {
				t.Parallel()
				_, err := BuildArgs(stage, tc.cfg, p, ids)
				if err == nil {
					t.Fatalf("BuildArgs built a paid %s command line with a missing ceiling", stage)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not name %q", err, tc.wantErr)
				}
			})
		}
	}
}

func TestPaidInvocationCarriesBothCeilings(t *testing.T) {
	t.Parallel()
	p := NewPaths(t.TempDir())
	cfg := Config{ID: "x", Agent: "openai:gpt-not-real", Concurrency: 4, MaxCostUSD: 2.5, MaxCalls: 40}
	for _, stage := range []string{"baseline", "value"} {
		sa, err := BuildArgs(stage, cfg, p, RunIDs{Baseline: "b", Value: "v"})
		if err != nil {
			t.Fatalf("%s: %v", stage, err)
		}
		if !slices.Contains(sa.Args, "--max-cost-usd") || !slices.Contains(sa.Args, "--max-calls") {
			t.Fatalf("%s: %v carries neither ceiling", stage, sa.Args)
		}
	}
}

func TestLaterStagesNeedTheirPredecessor(t *testing.T) {
	t.Parallel()
	p := NewPaths(t.TempDir())
	cfg := Config{ID: "x", Agent: FakeAgent, Concurrency: 1}
	for _, stage := range []string{"value", "select", "export"} {
		if _, err := BuildArgs(stage, cfg, p, RunIDs{}); err == nil {
			t.Fatalf("%s built a command line with no upstream run id", stage)
		}
	}
}
