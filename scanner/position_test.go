package scanner_test

import (
	"fmt"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	yamltestsuite "github.com/go-openapi/go-yaml/internal/testdata/yaml-test-suite"
)

// positionDefects counts the two ways a token position can be malformed.
type positionDefects struct {
	// zeroColumns counts tokens whose Column is below 1. Columns are 1-based
	// everywhere else, so 0 is out of range rather than merely surprising.
	zeroColumns int
	// backwards counts tokens positioned before the token that precedes them.
	backwards int
}

func (d positionDefects) empty() bool { return d == positionDefects{} }

func (d positionDefects) String() string {
	return fmt.Sprintf("{zeroColumns:%d, backwards:%d}", d.zeroColumns, d.backwards)
}

// positionLedger records the exact extent of two known position defects, per
// YAML Test Suite case.
//
//   - Content tokens of block scalars (| and >) are reported at column 0. Every
//     entry below with a zeroColumns count is a literal or folded scalar.
//   - A tab that looks like indentation can push a token's position backwards,
//     so positions stop advancing monotonically through the stream.
//
// Neither is a consumer's problem to work around: a column of 0 cannot be used
// to address the source at all. Both are scanner defects and both are expected
// to disappear when the scanner moves to byte offsets.
//
// The ledger is a ratchet in both directions. A case that starts violating an
// invariant fails as a regression; a case that stops violating fails too, and
// the fix is recorded by deleting the entry.
var positionLedger = map[string]positionDefects{
	"construct-binary": {zeroColumns: 1},
	"more-indented-lines-at-the-beginning-of-folded-block-scalars": {zeroColumns: 1},
	"spec-example-2-16-indentation-determines-scope":               {zeroColumns: 1},
	"spec-example-2-27-invoice":                                    {zeroColumns: 1},
	"spec-example-5-7-block-scalar-indicators":                     {zeroColumns: 1},
	"spec-example-6-1-indentation-spaces":                          {zeroColumns: 1},
	"spec-example-8-10-folded-lines-8-13-final-empty-lines":        {zeroColumns: 1},
	"spec-example-8-2-block-indentation-indicator":                 {zeroColumns: 1},
	"spec-example-8-2-block-indentation-indicator-1-3":             {zeroColumns: 1},
	"spec-example-8-5-chomping-trailing-lines":                     {zeroColumns: 2},
	"spec-example-8-8-literal-content":                             {zeroColumns: 1},
	"spec-example-8-8-literal-content-1-3":                         {zeroColumns: 1},
	"tabs-that-look-like-indentation/06":                           {backwards: 1},
}

// TestPositionLedger measures token positions over the whole YAML Test Suite
// and holds the known defects to their recorded extent.
func TestPositionLedger(t *testing.T) {
	tests, err := yamltestsuite.TestSuites()
	require.NoError(t, err)
	require.NotEmpty(t, tests)

	measured := make(map[string]positionDefects)

	for _, test := range tests {
		var (
			defects           positionDefects
			prevLine, prevCol int
		)

		for _, tk := range scanAll(t, string(test.InYAML)) {
			pos := tk.Position
			require.NotNil(t, pos)

			if pos.Column < 1 {
				defects.zeroColumns++
			}
			if pos.Line < prevLine || (pos.Line == prevLine && pos.Column < prevCol) {
				defects.backwards++
			}
			prevLine, prevCol = pos.Line, pos.Column
		}

		if !defects.empty() {
			measured[test.Name] = defects
		}
	}

	for name, got := range measured {
		want, known := positionLedger[name]
		if !assert.Truef(t, known, "%s: new position defect %s, not in the ledger", name, got) {
			continue
		}
		assert.Equalf(t, want, got, "%s: position defects changed", name)
	}

	for name, want := range positionLedger {
		_, still := measured[name]
		assert.Truef(t, still,
			"%s: no longer shows %s -- if that is a fix, delete the ledger entry", name, want)
	}
}
