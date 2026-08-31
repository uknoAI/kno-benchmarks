package summary

import (
	"testing"
	"time"
)

func TestRuleRefusesExclusionThatPostdatesTheData(t *testing.T) {
	t.Parallel()
	measured := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		rule Rule
		want bool
	}{
		{
			name: "a rule committed before the data may exclude it",
			rule: Rule{ID: "EX-1", Committed: true, EffectiveAt: measured.Add(-time.Hour)},
			want: true,
		},
		{
			name: "a rule committed after the data may not",
			rule: Rule{ID: "EX-1", Committed: true, EffectiveAt: measured.Add(time.Hour)},
			want: false,
		},
		{
			name: "a rule committed at the same instant may",
			rule: Rule{ID: "EX-1", Committed: true, EffectiveAt: measured},
			want: true,
		},
		{
			name: "an uncommitted rule excludes nothing",
			rule: Rule{ID: "EX-1", Committed: false},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.rule.Applies(measured); got != tc.want {
				t.Fatalf("Applies = %v, want %v", got, tc.want)
			}
		})
	}
}
