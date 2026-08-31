//go:build !linux && !darwin

package harness

// maxRSSBytes returns 0 on platforms whose ru_maxrss unit this harness has
// not verified. A zero is recorded as "not measured" rather than as a small
// number, because a wrong memory figure is worse than a missing one.
func maxRSSBytes(_ int64) int64 { return 0 }
