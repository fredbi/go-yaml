// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"
	"github.com/go-openapi/go-yaml/parser"
)

// The generator finds shapes; this file pins them to one document each.
//
// A shape is what the ledger can express and it is the right thing to measure,
// but it is not something anyone can sit down and fix. These are the same
// defects reduced to the smallest input that shows them, with the behavior as
// it is today rather than as it should be.
//
// They therefore fail when the defect is fixed. That is the point: the fix
// arrives together with the deletion of its ledger entry and the correction of
// the expectation here, and none of the three can be forgotten.

// TestDefectKeptBlankLineUnderAStatedIndentIsRejected: a block scalar that
// states its indentation and keeps its trailing blank lines is not read at all.
//
// The blank line carries no indentation of its own, and YAML allows that --
// l-empty admits a line indented less than the header states. Every neighboring
// document is accepted, which is what says the two features are only rejected
// together rather than either being unsupported.
func TestDefectKeptBlankLineUnderAStatedIndentIsRejected(t *testing.T) {
	const src = "k: |2+\n  one\n\n"

	// The emitter writes exactly this, so the document is ours and the
	// rejection is not a matter of taste about how to spell it.
	assert.Equal(t, src, yamlgen.Emit(
		yamlgen.Map{Pairs: []yamlgen.Pair{{Key: "k", Val: yamlgen.Str{V: "one\n\n"}}}},
		yamlgen.Style{Indent: 2, Quoting: yamlgen.QuotePlain, Literal: true, BlockIndicator: true, NullSpelling: "null"},
	))

	var got any
	err := yaml.Unmarshal([]byte(src), &got)
	require.Error(t, err, "if this now parses, the defect is fixed")
	assert.Contains(t, err.Error(), "invalid number of indent")

	// One difference each, all accepted.
	for name, ok := range map[string]string{
		"without the indicator":     "k: |+\n  one\n\n",
		"blank line padded out":     "k: |2+\n  one\n  \n",
		"blank line in the middle":  "k: |2\n  one\n\n  two\n",
		"indicator without keeping": "k: |2\n  one\n",
	} {
		t.Run(name, func(t *testing.T) {
			var v any
			assert.NoError(t, yaml.Unmarshal([]byte(ok), &v))
		})
	}
}

// TestDefectStatedIndentIsNotUpdatedWhenContentIsReIndented: a block scalar
// that states its indentation is re-indented without the header being changed
// to match, so the value gains a leading space every cycle.
//
// A scalar with no indicator round trips: the renderer picks the width and the
// content says what it is. Stating a width the content then contradicts is what
// puts the difference into the value.
func TestDefectStatedIndentIsNotUpdatedWhenContentIsReIndented(t *testing.T) {
	const src = "|2\n a\n"

	var before any
	require.NoError(t, yaml.Unmarshal([]byte(src), &before))
	assert.Equal(t, "a\n", before, "reading is correct: at the root, |2 means column 1")

	rendered := render(t, src)
	assert.Equal(t, "|2\n  a\n", rendered,
		"the content moved a column and the header did not follow")

	var after any
	require.NoError(t, yaml.Unmarshal([]byte(rendered), &after))
	assert.Equal(t, " a\n", after,
		"so a space moved into the value -- if this now round trips, the defect is fixed")

	t.Run("without an indicator it round trips", func(t *testing.T) {
		var v any
		require.NoError(t, yaml.Unmarshal([]byte(render(t, "|\n a\n")), &v))
		assert.Equal(t, "a\n", v)
	})
}

// render parses a document and writes it back out.
func render(t *testing.T, src string) string {
	t.Helper()

	file, err := parser.ParseBytes([]byte(src), parser.ParseComments)
	require.NoError(t, err)

	return file.String()
}
