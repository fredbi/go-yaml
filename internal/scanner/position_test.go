// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner_test

import (
	"iter"
	"slices"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/scanner/internal/testscanner"
	yamltestsuite "github.com/go-openapi/go-yaml/internal/yamltestsuite"
	"github.com/go-openapi/go-yaml/token"
)

// TestPositionLedger measures token positions over the whole YAML Test Suite.
//
// It holds the known defects to their recorded extent.
func TestPositionLedger(t *testing.T) {
	measured := testscanner.NewPositionLedger()

	for test := range yamlTests(t) {
		var (
			defects           testscanner.PositionDefects
			prevLine, prevCol int32
		)

		for _, tk := range scanAll(t, test.InYAML) {
			pos := tk.Position

			if pos.Column < 1 {
				defects.ZeroColumns++
			}
			if pos.Line < prevLine || (pos.Line == prevLine && pos.Column < prevCol) {
				defects.Backwards++
			}
			prevLine, prevCol = pos.Line, pos.Column
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

// positionLedger records position defects the scanner is known to produce, per YAML Test Suite case.
//
// It is empty, and the two invariants it guards hold over the whole suite:
//
//   - No token reports a column below 1. Columns count from 1, so 0 addresses nothing.
//   - No token is positioned before the token in front of it.
//
// It held twelve entries until 2026-09-06, every one a block scalar whose content token carried column 0. They were
// one defect at one site, not twelve cases: Scanner.bufferedToken worked the position out again for a block a dedent
// had ended, where the block had already recorded it. See the comment there.
//
// The ledger is a ratchet in both directions. A case that starts violating an invariant fails as a regression, and a
// case that stops violating fails too, so the fix is recorded by deleting the entry. An entry added here has to carry
// an Explanation saying why the defect stands, and "not fixed yet" is not one.
var positionLedger = testscanner.PositionLedger{} //nolint:gochecknoglobals // an immutable map is fine as a global

// TestBlockScalarPositionDoesNotDependOnWhatFollows holds the two paths that end a block scalar to the same position.
//
// A block ends either at the end of the source, through emitMultiLine, or at a dedent, through
// Scanner.bufferedToken. Both report where the content began. The second used to work the position out again and get
// it wrong for a folded scalar: "a: >\n  fold\n  more\n" reported line 2 column 3, and the same scalar with "b: 1"
// after it reported line 3 column 0.
func TestBlockScalarPositionDoesNotDependOnWhatFollows(t *testing.T) {
	for _, tc := range []struct {
		name  string
		alone string
		then  string
	}{
		{"folded", "a: >\n  fold\n  more\n", "a: >\n  fold\n  more\nb: 1\n"},
		{"literal", "a: |\n  one\n  two\n", "a: |\n  one\n  two\nb: 1\n"},
		{"folded, indentation indicator", "a: >2\n   x\n   y\n", "a: >2\n   x\n   y\nb: 1\n"},
		{"literal, kept breaks", "a: |+\n  x\n\n", "a: |+\n  x\n\nb: 1\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			alone := blockContentToken(t, tc.alone)
			then := blockContentToken(t, tc.then)

			assert.Equalf(t, alone.Value, then.Value, "the value must not depend on what follows the block")
			assert.Equalf(t, alone.Position.Line, then.Position.Line, "line moved when %q was added after the block", "b: 1")
			assert.Equalf(t, alone.Position.Column, then.Position.Column, "column moved when a key was added after the block")
			assert.Equalf(t, alone.Position.Offset(), then.Position.Offset(), "offset moved when a key was added after the block")
			assert.GreaterOrEqualf(t, then.Position.Column, int32(1), "a column addresses the source and counts from 1")
		})
	}
}

// blockContentToken returns the string token holding a block scalar's content, which follows its header.
func blockContentToken(t *testing.T, src string) token.Token {
	t.Helper()

	tokens := tokenize(t, src)
	for i, tk := range tokens {
		switch tk.Type {
		case token.LiteralType, token.FoldedType:
			require.Greaterf(t, len(tokens), i+1, "%q: the block scalar header is the last token", src)

			return tokens[i+1]
		default:
		}
	}
	t.Fatalf("%q: holds no block scalar header", src)

	return token.Token{}
}
