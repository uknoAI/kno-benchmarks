package summary

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Rule is one exclusion rule from METHODOLOGY.md, together with the commit
// timestamp at which it entered the repository's history.
//
// The timestamp is the teeth. You may exclude an observation on a principle
// committed before it was measured; you may never exclude one on an outcome.
type Rule struct {
	ID          string
	Description string
	// EffectiveAt is the author timestamp of the first commit in which this
	// rule's ID appears in METHODOLOGY.md.
	EffectiveAt time.Time
	// Committed is false when the rule cannot be located in git history: the
	// working tree is dirty, the file is not committed yet, or git is not
	// available. An uncommitted rule excludes nothing, which is the safe
	// direction: it widens the published spread rather than narrowing it.
	Committed bool
}

// Applies reports whether the rule may be used to exclude an observation
// recorded at observedAt.
func (r Rule) Applies(observedAt time.Time) bool {
	if !r.Committed {
		return false
	}
	return !r.EffectiveAt.After(observedAt)
}

var ruleRow = regexp.MustCompile(`^\|\s*(EX-\d+)\s*\|\s*(.*?)\s*\|`)

// LoadRules parses the exclusion-rule table out of METHODOLOGY.md and dates
// each rule from git.
func LoadRules(repoRoot, methodologyPath string) (map[string]Rule, error) {
	body, err := gitShowOrRead(repoRoot, methodologyPath)
	if err != nil {
		return nil, err
	}
	rules := map[string]Rule{}
	sc := bufio.NewScanner(strings.NewReader(body))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		m := ruleRow.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		r := Rule{ID: m[1], Description: m[2]}
		if ts, ok := firstCommitContaining(repoRoot, methodologyPath, m[1]); ok {
			r.EffectiveAt = ts
			r.Committed = true
		}
		rules[r.ID] = r
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", methodologyPath, err)
	}
	return rules, nil
}

func gitShowOrRead(repoRoot, path string) (string, error) {
	out, err := exec.Command("git", "-C", repoRoot, "show", "HEAD:"+path).Output() //nolint:gosec // fixed arguments.
	if err == nil {
		return string(out), nil
	}
	b, rerr := exec.Command("cat", repoRoot+"/"+path).Output() //nolint:gosec // fixed arguments.
	if rerr != nil {
		return "", fmt.Errorf("read %s: %w", path, rerr)
	}
	return string(b), nil
}

// firstCommitContaining returns the author timestamp of the earliest commit
// whose change to path introduced needle.
func firstCommitContaining(repoRoot, path, needle string) (time.Time, bool) {
	cmd := exec.Command("git", "-C", repoRoot, "log", "--reverse", "--format=%aI", "-S"+needle, "--", path) //nolint:gosec // fixed arguments.
	out, err := cmd.Output()
	if err != nil {
		return time.Time{}, false
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		ts, err := time.Parse(time.RFC3339, line)
		if err != nil {
			return time.Time{}, false
		}
		return ts, true
	}
	return time.Time{}, false
}
