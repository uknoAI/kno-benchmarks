package harness

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Generated inputs are deterministic functions of (n, m) alone. Two runs of
// the same configuration therefore measure the same bytes, on purpose: an
// input that varied between repetitions would put its own variance inside the
// spread this repository exists to characterize.

type genCase struct {
	ID       string   `json:"id"`
	Input    string   `json:"input"`
	Expected string   `json:"expected"`
	Tags     []string `json:"tags"`
}

type genAsset struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Kind    string `json:"kind"`
}

// assetBody is a fixed ~200-byte body. Asset size is not a matrix axis, so it
// is held constant rather than left to drift with the index.
var assetBody = strings.Repeat("Reference material for the support agent. ", 5)

// GenerateCases writes m synthetic Cases as JSONL.
func GenerateCases(path string, m int) error {
	return writeJSONL(path, m, func(i int) any {
		return genCase{
			ID:       fmt.Sprintf("case-%08d", i),
			Input:    fmt.Sprintf("Synthetic question %d: what is the documented answer?", i),
			Expected: fmt.Sprintf("Documented answer %d.", i),
			Tags:     []string{fmt.Sprintf("topic-%d", i%8)},
		}
	})
}

// GenerateAssets writes n synthetic Assets as JSONL.
func GenerateAssets(path string, n int) error {
	return writeJSONL(path, n, func(i int) any {
		return genAsset{
			ID:      fmt.Sprintf("asset-%08d", i),
			Content: fmt.Sprintf("Asset %d. %s", i, assetBody),
			Kind:    "knowledge",
		}
	})
}

func writeJSONL(path string, n int, row func(int) any) error {
	f, err := os.Create(path) //nolint:gosec // path is a harness-owned temp file.
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck // the flush and close below carry the error.

	w := bufio.NewWriterSize(f, 1<<20)
	enc := json.NewEncoder(w)
	for i := range n {
		if err := enc.Encode(row(i)); err != nil {
			return fmt.Errorf("encode row %d of %s: %w", i, path, err)
		}
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush %s: %w", path, err)
	}
	return f.Close()
}
