package harness

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Budget is cost-control layer 2: the per-workflow-run aggregate.
//
// Layer 1 (per-invocation --max-cost-usd / --max-calls) structurally cannot
// catch twenty individually-capped runs that together cost twenty times the
// cap. This can, and it aborts the rest of the matrix rather than warning.
type Budget struct {
	LimitUSD float64
	spent    float64
	aborted  bool
}

// Add books spend and reports whether the run must stop. A zero limit means
// unlimited, which is only ever correct for a `fake:` matrix that spends
// nothing; the runner refuses a paid agent with a zero limit.
func (b *Budget) Add(usd float64) bool {
	b.spent += usd
	if b.LimitUSD > 0 && b.spent > b.LimitUSD {
		b.aborted = true
	}
	return b.aborted
}

// Spent returns the accumulated total.
func (b *Budget) Spent() float64 { return b.spent }

// Aborted reports whether the cap has fired.
func (b *Budget) Aborted() bool { return b.aborted }

// parseSpend extracts the spend a stage's --json document reports.
//
// The second return value distinguishes "reported zero" from "reported
// nothing". As of kno v0.1.2 only `baseline` carries `spent_usd`; `value`,
// `select` and `export` do not. Collapsing the two into 0.0 would let a paid
// value run accumulate invisibly against the aggregate cap, which is the exact
// failure layer 2 exists to prevent.
func parseSpend(doc []byte) (float64, bool, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(doc, &m); err != nil {
		return 0, false, fmt.Errorf("parse --json document: %w", err)
	}
	raw, ok := m["spent_usd"]
	if !ok {
		return 0, false, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, false, fmt.Errorf("parse spent_usd: %w", err)
	}
	v, err := parseUSD(s)
	if err != nil {
		return 0, false, err
	}
	return v, true, nil
}

func parseUSD(s string) (float64, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "$")
	s = strings.ReplaceAll(s, ",", "")
	if s == "" {
		return 0, fmt.Errorf("empty dollar figure")
	}
	// kno reports sub-cent amounts as "<$0.01"; treat the bound as the value
	// rather than as zero, because the aggregate must never round spend down.
	s = strings.TrimPrefix(s, "<")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse dollar figure %q: %w", s, err)
	}
	return v, nil
}

func parseRunID(doc []byte) string {
	var m struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(doc, &m); err != nil {
		return ""
	}
	return m.RunID
}
