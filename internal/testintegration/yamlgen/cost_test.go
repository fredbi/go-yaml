// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"testing"
	"time"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/testintegration/grammar"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"
	"github.com/go-openapi/go-yaml/parser"
)

// The cost of a parse, measured as a curve rather than as a number.
//
// Every other test in this package asks what a document means or whether it is
// one. None of them notices a parser that takes 67 seconds over 400,000
// brackets, because it reads them correctly -- so the parse can go quadratic
// with the whole corpus green and no verdict anywhere disagreeing. The shapes
// come from [yamlgen.DeepDocument]; the measurement is here.

// costSpan is how much bigger the large document is than the small one.
//
// Eight, not two. The two curves are told apart by a factor that compounds, so
// a wide span separates them far past the noise: over eight times the depth a
// linear parse costs about the same per byte and a quadratic one about eight
// times as much.
const costSpan = 8

// costCeiling is what a linear parse may cost per byte at costSpan times the
// depth.
//
// Measured on 2026-09-03, the five linear shapes land between 0.9 and 1.3 and
// the two quadratic ones at 5.4 and 5.1. Anything between 2 and 4 separates
// them; 2.5 leaves twice the observed spread for a slower or noisier machine.
const costCeiling = 2.5

// TestNestingCostStaysLinear: reading a document eight times as deep must not
// cost eight times as much per byte.
//
// These five shapes the parser reads linearly today and has to keep reading
// linearly. The two flow shapes do not, and are pinned in
// TestDefectFlowNestingIsQuadratic rather than excused here.
func TestNestingCostStaysLinear(t *testing.T) {
	for _, shape := range []yamlgen.Depth{
		yamlgen.BlockSeqCompact,
		yamlgen.BlockSeqIndented,
		yamlgen.BlockMapIndented,
		yamlgen.AliasChain,
		yamlgen.FlatSeq,
	} {
		t.Run(shape.String(), func(t *testing.T) {
			got := costGrowth(t, shape)
			t.Logf("%s costs %.2f times as much per byte at %d times the depth", shape, got, costSpan)

			assert.Less(t, got, costCeiling,
				"reading %s got %.2f times dearer per byte for %d times the depth, "+
					"which is a curve that bends", shape, got, costSpan)
		})
	}
}

// costGrowth returns how much more a parse costs per byte at costSpan times the
// depth.
//
// Per byte rather than per document, because two of the shapes indent one space
// further at every level and so hold n^2 bytes at depth n. Measured against
// depth they would look quadratic in a parser that is doing nothing wrong.
func costGrowth(t *testing.T, shape yamlgen.Depth) float64 {
	t.Helper()

	// The indented shapes reach eight megabytes at depth 4000 and the flow ones
	// stay under a hundred kilobytes at 32000, so the depth that makes a fair
	// measurement is not the same for both.
	small := 4000
	if shape.Quadratic() {
		small = 500
	}

	// A depth of eight is where the shape is checked against the grammar. It is
	// the same shape at every depth, and running the recognizer over eight
	// megabytes would be the slowest thing in this package by a wide margin.
	require.True(t, grammar.NewRecognizer(1024).Stream(yamlgen.DeepDocument(shape, 8)).OK,
		"%s is not YAML 1.2, so its cost says nothing about parsing one", shape)

	return parseCostPerByte(t, shape, small*costSpan) / parseCostPerByte(t, shape, small)
}

// parseCostPerByte times a parse three times over and keeps the fastest.
//
// A parse does no I/O and allocates from a warm heap, so the spread between
// runs is the scheduler's and the minimum is the closest thing to hand to the
// cost of the work itself.
func parseCostPerByte(t *testing.T, shape yamlgen.Depth, n int) float64 {
	t.Helper()

	src := yamlgen.DeepDocument(shape, n)
	best := time.Duration(1 << 62)

	for range 3 {
		start := time.Now()
		_, err := parser.ParseBytes(src, parser.Comments())
		elapsed := time.Since(start)

		require.NoError(t, err)

		best = min(best, elapsed)
	}

	require.NotZero(t, best, "%s at depth %d was too fast to time", shape, n)

	return float64(best) / float64(len(src))
}
