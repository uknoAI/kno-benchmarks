//go:build darwin

package harness

// maxRSSBytes converts getrusage(2)'s ru_maxrss to bytes. Darwin reports
// bytes already.
func maxRSSBytes(maxrss int64) int64 { return maxrss }
