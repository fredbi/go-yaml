package parser_test

import (
	"runtime"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/corpus"
	"github.com/go-openapi/go-yaml/parser"
)

// The sizes to compare. Sixteen times as many entries is enough separation to
// tell a constant from a growing one, and small enough to stay fast.
const (
	scalingSmall = 500
	scalingLarge = 8000

	// How much the per-entry cost may grow between those two sizes. A linear
	// parser holds it flat; measured, it moves by about a tenth, which is the
	// allocator's own size classes rather than the algorithm. The recursive
	// parseMap moved it by a factor of nine over the same range, so anything
	// near this bound is a real regression rather than noise.
	scalingTolerance = 2.0
)

// TestParseScalesLinearlyInWidth guards the property that a mapping's parse
// cost is proportional to the number of entries, not to their square.
//
// parseMap used to recurse once per sibling entry, building a whole
// MappingNode at every level and discarding it to keep only its values, so a
// mapping of N keys cost N recursions and slice concatenations summing to
// O(N^2). It was invisible on small fixtures and severe on the wide, shallow
// mappings a large OpenAPI paths: section produces.
//
// The measurement is bytes allocated per entry rather than time. Allocation is
// what the defect actually multiplied, and it is deterministic -- the same
// numbers on a busy CI runner as on an idle workstation, which a timing
// threshold could not promise.
func TestParseScalesLinearlyInWidth(t *testing.T) {
	for name, generate := range map[string]func(int) string{
		"mapping":  corpus.FlatMap,
		"sequence": corpus.FlatSequence,
	} {
		t.Run(name, func(t *testing.T) {
			small := bytesPerEntry(t, generate(scalingSmall), scalingSmall)
			large := bytesPerEntry(t, generate(scalingLarge), scalingLarge)

			require.Positive(t, small)
			growth := large / small

			assert.LessOrEqualf(t, growth, scalingTolerance,
				"per-entry cost grew %.1fx between %d and %d entries (%.0f B to %.0f B): "+
					"parsing is super-linear in the number of sibling entries",
				growth, scalingSmall, scalingLarge, small, large)
		})
	}
}

// bytesPerEntry reports how many bytes parsing src allocates per entry.
func bytesPerEntry(t *testing.T, src string, entries int) float64 {
	t.Helper()

	source := []byte(src)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	_, err := parser.ParseBytes(source, 0)
	require.NoError(t, err)

	runtime.ReadMemStats(&after)

	return float64(after.TotalAlloc-before.TotalAlloc) / float64(entries)
}
