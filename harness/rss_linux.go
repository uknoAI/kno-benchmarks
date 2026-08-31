//go:build linux

package harness

// maxRSSBytes converts getrusage(2)'s ru_maxrss to bytes. Linux reports
// kilobytes; Darwin reports bytes. Getting this wrong is a silent factor of
// 1024 in the one number that tests a documented claim, so it lives in
// build-tagged files rather than a runtime switch.
func maxRSSBytes(maxrss int64) int64 { return maxrss * 1024 }
