// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser"
)

// The generator finds these shapes; this file pins them to one document each.
//
// A shape is what the ledger can express and it is the right thing to measure,
// but it is not something anyone can sit down and fix. These are the same
// defects reduced to the smallest input that shows them, with the behavior as
// it is today rather than as it should be.
//
// They therefore fail when the defect is fixed. That is the point: the fix
// arrives together with the deletion of its ledger entry and the correction of
// the expectation here, and none of the three can be forgotten.

// TestDefectCommentOnASequenceEntryMovesOrIsLost: a comment on a sequence entry
// with nothing else on its line is not kept where it was.
//
// Two symptoms, one cause. The comment has nothing on its own line to attach to,
// so the renderer moves it -- and where it cannot, drops it. A mapping entry in
// the same position keeps its comment, which places the defect in how sequence
// entries carry comments rather than in comments generally.
func TestDefectCommentOnASequenceEntryMovesOrIsLost(t *testing.T) {
	t.Run("it moves twice, so rendering takes two passes to settle", func(t *testing.T) {
		// The comment starts on its own line between the entry and the nested
		// sequence it introduces.
		once := render(t, "-\n# c\n - x\n")
		assert.Equal(t, "- # c\n  - x\n", once, "first it moves onto the entry's line")

		twice := render(t, once)
		assert.Equal(t, "# c\n- - x\n", twice, "then out to the head of the document")

		assert.Equal(t, twice, render(t, twice), "only then does it settle")
	})

	t.Run("it is lost when the head is already taken", func(t *testing.T) {
		file, err := parser.ParseBytes([]byte("# c1\n-  # c2\n"), parser.ParseComments)
		require.NoError(t, err)

		rendered := file.String()
		assert.NotContains(t, rendered, "# c2",
			"the line comment is dropped -- if it survives now, the defect is fixed")
		assert.Contains(t, rendered, "# c1", "the head comment survives")
	})

	t.Run("a mapping entry keeps its comment", func(t *testing.T) {
		file, err := parser.ParseBytes([]byte("k: # c\n  j: 1\n"), parser.ParseComments)
		require.NoError(t, err)
		assert.Equal(t, "k: # c\n  j: 1\n", file.String())
	})
}

// render parses a document and writes it back out.
func render(t *testing.T, src string) string {
	t.Helper()

	file, err := parser.ParseBytes([]byte(src), parser.ParseComments)
	require.NoError(t, err)

	return file.String()
}
