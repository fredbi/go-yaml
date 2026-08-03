// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml"
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

// TestDefectCommentAfterAnAnchoredEmptyEntrySwallowsTheRest: an entry whose
// line ends on an anchor that named nothing, followed by a comment, takes every
// later entry down into itself.
//
// This is the worst class of defect the harness looks for: the document still
// parses, still settles, and means something else. The sub-cases below are what
// says the cause is an anchor with no value after it, rather than anchors or
// comments in general.
func TestDefectCommentAfterAnAnchoredEmptyEntrySwallowsTheRest(t *testing.T) {
	t.Run("a sequence entry takes its siblings with it", func(t *testing.T) {
		const src = "- &a1\n#\n- x\n"

		var before any
		require.NoError(t, yaml.Unmarshal([]byte(src), &before))
		assert.Equal(t, []any{nil, "x"}, before, "reading is correct")

		rendered := render(t, src)
		assert.Equal(t, "- &a1\n  #\n  - x\n", rendered,
			"the comment and the entry after it are indented under the anchor")

		var after any
		require.NoError(t, yaml.Unmarshal([]byte(rendered), &after))
		assert.Equal(t, []any{[]any{"x"}}, after,
			"two entries became one nested in the other -- if this now round trips, the defect is fixed")
	})

	t.Run("a mapping entry does the same", func(t *testing.T) {
		rendered := render(t, "k: &a1\n#\nj: x\n")
		assert.Equal(t, "k: &a1\n  #\n  j: x\n", rendered)
	})

	// Each of these differs from the failing document in one respect, and each
	// round trips. Together they are the argument that the cause is an anchor
	// with nothing after it and something after that.
	t.Run("or the end of a nested collection swallows an outer entry", func(t *testing.T) {
		// No comment here: what follows the anchor is the next mapping key,
		// written level with the entry that ended on the anchor because a block
		// sequence under a key sits at the key's own column.
		const src = "\"\": &a2\n - &a1\n\" \":\n"

		var before any
		require.NoError(t, yaml.Unmarshal([]byte(src), &before))
		assert.Equal(t, map[string]any{"": []any{nil}, " ": nil}, before, "reading is correct")

		rendered := render(t, src)

		var after any
		require.NoError(t, yaml.Unmarshal([]byte(rendered), &after))
		assert.Equal(t, map[string]any{"": []any{map[string]any{" ": nil}}}, after,
			"the second key was absorbed -- if this now round trips, the defect is fixed")

		assert.NotEqual(t, rendered, render(t, rendered),
			"and unlike the comment trigger, this one does not settle either")
	})

	t.Run("what does not trigger it", func(t *testing.T) {
		for name, src := range map[string]string{
			"no anchor":                   "-\n#\n- x\n",
			"no comment":                  "- &a1\n- x\n",
			"the anchor names a sequence": "- &a1\n  - 1\n#\n- x\n",
			"the anchor names a mapping":  "k: &a1\n  a: 1\n#\nj: x\n",
			"nothing follows the comment": "- x\n- &a1\n#\n",
		} {
			t.Run(name, func(t *testing.T) {
				var before, after any
				require.NoError(t, yaml.Unmarshal([]byte(src), &before))
				require.NoError(t, yaml.Unmarshal([]byte(render(t, src)), &after))
				assert.Equal(t, before, after)
			})
		}
	})
}

// render parses a document and writes it back out.
func render(t *testing.T, src string) string {
	t.Helper()

	file, err := parser.ParseBytes([]byte(src), parser.ParseComments)
	require.NoError(t, err)

	return file.String()
}
