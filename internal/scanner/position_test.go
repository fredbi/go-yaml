// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner_test

import (
	"iter"
	"slices"
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/scanner/internal/testscanner"
	yamltestsuite "github.com/go-openapi/go-yaml/internal/yamltestsuite"
)

// TestPositionLedger measures token positions over the whole YAML Test Suite.
//
// It holds the known defects to their recorded extent.
func TestPositionLedger(t *testing.T) {
	measured := testscanner.NewPositionLedger()

	for test := range yamlTests(t) {
		var (
			defects           testscanner.PositionDefects
			prevLine, prevCol int
		)

		for _, tk := range scanAll(t, test.InYAML) {
			pos := tk.Position
			require.NotNil(t, pos)

			if pos.Column < 1 {
				defects.ZeroColumns++
			}
			if line, col := int(pos.Line), int(pos.Column); line < prevLine || (line == prevLine && col < prevCol) { // TODO(fred): type mismatch that requires conversion - TO BE FIXED
				defects.Backwards++
			}
			prevLine, prevCol = int(pos.Line), int(pos.Column)
		}

		measured.Record(test.Name, defects)
	}

	testscanner.ComparePositionLedgers(t, measured, positionLedger)
}

func yamlTests(t *testing.T) iter.Seq[*yamltestsuite.TestSuite] {
	tests, err := yamltestsuite.TestSuites()
	require.NoError(t, err)
	require.NotEmpty(t, tests)

	return slices.Values(tests)
}

// positionLedger records the exact extent of two known position defects, per YAML Test Suite case.
//
//   - Content tokens of block scalars (| and >) are reported at column 0. Every
//     entry below with a ZeroColumns count is a literal or folded scalar.
//   - A tab that looks like indentation can push a token's position backwards,
//     so positions stop advancing monotonically through the stream.
//
// Neither is a consumer's problem to work around: a column of 0 cannot be used to address the source at all.
// Both are scanner defects and both are expected to disappear when the scanner moves to byte offsets.
//
// The ledger is a ratchet in both directions.
// A case that starts violating an invariant fails as a regression; a case that stops violating fails too, and the fix
// is recorded by deleting the entry.
//
// TODO: the ledger should hold a justification/explanation for an excused defect.
var positionLedger = testscanner.PositionLedger{ //nolint:gochecknoglobals // ok to store and immutable map as a global
	"construct-binary": {ZeroColumns: 1},
	"more-indented-lines-at-the-beginning-of-folded-block-scalars": {ZeroColumns: 1},
	"spec-example-2-16-indentation-determines-scope":               {ZeroColumns: 1},
	"spec-example-2-27-invoice":                                    {ZeroColumns: 1},
	"spec-example-5-7-block-scalar-indicators":                     {ZeroColumns: 1},
	"spec-example-6-1-indentation-spaces":                          {ZeroColumns: 1},
	"spec-example-8-10-folded-lines-8-13-final-empty-lines":        {ZeroColumns: 1},
	"spec-example-8-2-block-indentation-indicator":                 {ZeroColumns: 1},
	"spec-example-8-2-block-indentation-indicator-1-3":             {ZeroColumns: 1},
	"spec-example-8-5-chomping-trailing-lines":                     {ZeroColumns: 2},
	"spec-example-8-8-literal-content":                             {ZeroColumns: 1},
	"spec-example-8-8-literal-content-1-3":                         {ZeroColumns: 1},
}
