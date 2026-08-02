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

// TestDefectStripChompingEatsTrailingSpaces: `|-` removes trailing spaces from
// the last line as well as the line break.
//
// YAML 1.2 defines chomping over line breaks (b-chomped-last, l-chomped-empty).
// A trailing space is content: l-nb-literal-text matches nb-char+, and nb-char
// includes a space. Reading the clipped form of the same document keeps it,
// which is what shows the two paths disagree rather than the space being
// unrepresentable.
func TestDefectStripChompingEatsTrailingSpaces(t *testing.T) {
	var strip any
	require.NoError(t, yaml.Unmarshal([]byte("|-\n  trailing \n"), &strip))
	assert.Equal(t, "trailing", strip, "expected \"trailing \" -- if this now holds, the defect is fixed")

	// The same content, clipped rather than stripped, keeps the space.
	var clip any
	require.NoError(t, yaml.Unmarshal([]byte("|\n  trailing \n"), &clip))
	assert.Equal(t, "trailing \n", clip)

	// A trailing space on a line that is not the last one also survives, so the
	// defect is in chomping and not in reading block scalars generally.
	var inner any
	require.NoError(t, yaml.Unmarshal([]byte("|-\n  a \n  b\n"), &inner))
	assert.Equal(t, "a \nb", inner)
}

// TestDefectKeepChompingLosesTheNewlinesItKeeps: rendering writes the `|+`
// indicator without the blank lines it exists to preserve.
//
// Reading is correct; only rendering loses them. It is silent data loss in the
// round trip the library exists for.
func TestDefectKeepChompingLosesTheNewlinesItKeeps(t *testing.T) {
	const src = "k: |+\n  trail\n\n"

	var before any
	require.NoError(t, yaml.Unmarshal([]byte(src), &before))
	assert.Equal(t, map[string]any{"k": "trail\n\n"}, before, "reading is correct")

	file, err := parser.ParseBytes([]byte(src), parser.ParseComments)
	require.NoError(t, err)
	rendered := file.String()

	assert.Equal(t, "k: |+\n  trail\n", rendered,
		"the kept blank line is dropped -- if this now round trips, the defect is fixed")

	var after any
	require.NoError(t, yaml.Unmarshal([]byte(rendered), &after))
	assert.Equal(t, map[string]any{"k": "trail\n"}, after, "and the value changed")
}

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

// TestDefectSingleQuotedKeyLosesItsEscaping: rendering a mapping key read from
// a single-quoted scalar writes its quote unescaped, and the result does not
// parse.
//
// The same string in a value position survives, which places the defect in how
// keys are written rather than in single-quoted scalars.
func TestDefectSingleQuotedKeyLosesItsEscaping(t *testing.T) {
	file, err := parser.ParseBytes([]byte("'a''b': false\n"), parser.ParseComments)
	require.NoError(t, err)

	rendered := file.String()
	assert.Equal(t, "'a'b': false\n", rendered,
		"the escaped quote is written bare -- if this now round trips, the defect is fixed")

	_, err = parser.ParseBytes([]byte(rendered), parser.ParseComments)
	assert.Error(t, err, "and the rendered document no longer parses")

	// The same string as a value is written back correctly.
	value, err := parser.ParseBytes([]byte("k: 'a''b'\n"), parser.ParseComments)
	require.NoError(t, err)
	assert.Equal(t, "k: 'a''b'\n", value.String())
}

// render parses a document and writes it back out.
func render(t *testing.T, src string) string {
	t.Helper()

	file, err := parser.ParseBytes([]byte(src), parser.ParseComments)
	require.NoError(t, err)

	return file.String()
}
