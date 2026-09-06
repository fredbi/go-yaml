// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/testintegration/grammar"
	"github.com/go-openapi/go-yaml/parser"
)

// Shapes the generator found that still diverge.
//
// Each one pins today's behavior rather than the correct behavior, so that a
// fix breaks the test that says it was broken. The corresponding entry in
// [yamlgen.Ledger] is what keeps the property tests from failing on it
// meanwhile; when both go, the case moves to fixed_test.go.
//
// The first two below came from [yamlgen.Style.Break] on 2026-09-03, the axis that
// writes one document with LF, CRLF and a lone CR. Neither shape is exotic and
// neither was reachable before it. The third came from [yamlgen.DeepDocument]
// the same day, and is the one no verdict could have found: the documents parse
// correctly and cost quadratic time doing it. The rest came from [Tagged]
// and [yamlgen.Style.PropertyOrder] on the same day again.

// wellFormed asserts src is a YAML 1.2 document before anything is asked of the
// library, so that a case here is a claim about the library and not about a
// document nobody has to read.
func wellFormed(t *testing.T, src string) {
	t.Helper()

	require.True(t, grammar.NewRecognizer(1024).Stream([]byte(src)).OK,
		"the document is not YAML 1.2, so there is nothing to hold the library to")
}

// renderOnce reads a document with comments kept and writes it back.
func renderOnce(t *testing.T, src string) string {
	t.Helper()

	file, err := parser.ParseBytes([]byte(src), parser.WithComments())
	require.NoError(t, err)

	return file.String()
}

// TestDefectTagBeforeAnchorIsDropped: YAML 1.2 lets a node's tag and anchor
// appear in either order. Written second the tag holds; written first it is
// dropped from the node the anchor names.
//
// Three shapes, three failures, and the middle one is the reason this is worth
// more than a curiosity: a document loses entries and nobody is told.
func TestDefectTagBeforeAnchorIsDropped(t *testing.T) {
	t.Run("a collection tag stops the parse", func(t *testing.T) {
		for _, src := range []string{"a: !!seq &a1 [1]\n", "a: !!map &a1 {b: 1}\n"} {
			wellFormed(t, src)
			assert.Error(t, yaml.Unmarshal([]byte(src), new(any)), "today: %q is refused", src)
		}

		// The same documents with the anchor first are read.
		for _, src := range []string{"a: &a1 !!seq [1]\n", "a: &a1 !!map {b: 1}\n"} {
			wellFormed(t, src)
			assert.NoError(t, yaml.Unmarshal([]byte(src), new(any)))
		}
	})

	t.Run("on an empty node it swallows what follows", func(t *testing.T) {
		wellFormed(t, "- !!null &a1\n- x\n")

		var got any
		require.NoError(t, yaml.Unmarshal([]byte("- !!null &a1\n- x\n"), &got))
		assert.Equal(t, []any{nil}, got, "today: the second entry is gone, with no error")

		got = nil
		require.NoError(t, yaml.Unmarshal([]byte("- !!str &a1\n- x\n"), &got))
		assert.Equal(t, []any{"[x]"}, got, "today: the second entry is swallowed as text")

		got = nil
		require.NoError(t, yaml.Unmarshal([]byte("k: !!null &a1\nj: x\n"), &got))
		assert.Equal(t, map[string]any{"k": nil}, got, "today: j is gone")

		// Anchor first, all three read correctly.
		got = nil
		require.NoError(t, yaml.Unmarshal([]byte("- &a1 !!null\n- x\n"), &got))
		assert.Equal(t, []any{nil, "x"}, got)
	})

	t.Run("otherwise the anchor names the untagged value", func(t *testing.T) {
		wellFormed(t, "a: !!int &a1 \"5\"\nb: *a1\n")

		var got any
		require.NoError(t, yaml.Unmarshal([]byte("a: !!int &a1 \"5\"\nb: *a1\n"), &got))
		assert.Equal(t, map[string]any{"a": int(5), "b": "5"}, got,
			"today: one node, read as a number where it stands and as a string through the alias")

		got = nil
		require.NoError(t, yaml.Unmarshal([]byte("a: &a1 !!int \"5\"\nb: *a1\n"), &got))
		assert.Equal(t, map[string]any{"a": int(5), "b": int(5)}, got)

		// "!!str" is the one tag that survives the alias: the decoder replaces
		// what the anchor recorded with the string, since that is the value
		// the tag names. See TestFixedStrTagKeepsTheSpelling.
		got = nil
		require.NoError(t, yaml.Unmarshal([]byte("a: !!str &a1 5\nb: *a1\n"), &got))
		assert.Equal(t, map[string]any{"a": "5", "b": "5"}, got)
	})
}

// TestDefectCommentAfterALineEndingTagIsDropped: a comment sitting after a tag
// that is the last thing on its line does not survive a render.
//
// The anchor on its own keeps it, which is what says this is about the tag and
// not about comments at the end of a property line in general.
func TestDefectCommentAfterALineEndingTagIsDropped(t *testing.T) {
	for _, src := range []string{
		"!!null # c1\n",
		"&a1 !!null # c1\n",
		"k: !!null # c1\n",
		"- !!null # c1\n",
		"- &a2 !!seq # c2\n  - 1\n",
	} {
		wellFormed(t, src)
		assert.NotContains(t, renderOnce(t, src), "#", "today: %q loses its comment", src)
	}

	t.Run("kept without the tag, and kept once anything follows it", func(t *testing.T) {
		for _, src := range []string{"&a1 # c1\n", "!!null null # c1\n", "&a1 !!str x # c1\n"} {
			wellFormed(t, src)
			assert.Contains(t, renderOnce(t, src), "# c1", "%q", src)
		}
	})
}

// TestDefectCommentAboveAPropertyLineMoves: a comment on the line that
// introduces a node whose properties are written on the next line comes back
// attached to that node's last entry.
//
// Harmless where the last entry is a plain scalar. Where it is a block scalar
// the comment lands inside the content, and the value changes without anything
// reporting it.
func TestDefectCommentAboveAPropertyLineMoves(t *testing.T) {
	const moved = "k: # c1\n  &a2\n  - 1\n"
	wellFormed(t, moved)
	assert.Equal(t, "k: &a2\n- 1 # c1\n", renderOnce(t, moved), "today: the comment is on the entry")

	const corrupts = "k: # c1\n  &a2\n  - |-\n    trailing \n"
	wellFormed(t, corrupts)

	var before, after any
	require.NoError(t, yaml.Unmarshal([]byte(corrupts), &before))
	require.NoError(t, yaml.Unmarshal([]byte(renderOnce(t, corrupts)), &after))

	assert.Equal(t, []any{"trailing "}, before.(map[string]any)["k"])
	assert.Equal(t, []any{"trailing  # c1"}, after.(map[string]any)["k"],
		"today: the comment became part of the block scalar")
}

// TestDefectAStrTaggedNullSpellingIsLowercased: the renderer writes a
// `!!str`-tagged null spelling back in lower case, and under that tag `null` is
// the four letters, so the value is gone.
//
// The decode is right and only the render is wrong. Reading `!!str Null` as
// "Null" was the half of this fixed on the parser branch; writing it back is
// the half that was not. `!!str True` renders unchanged, so this is the
// renderer resolving the scalar it was told not to resolve and not a general
// case-folding.
func TestDefectAStrTaggedNullSpellingIsLowercased(t *testing.T) {
	for _, src := range []string{
		"k: !!str Null\n",
		"k: !!str NULL\n",
		"!!str Null\n",
		"&a1 !!str Null\n",
	} {
		wellFormed(t, src)
		assert.Contains(t, renderOnce(t, src), "!!str null",
			"today: %q comes back lower-cased", src)
	}

	t.Run("the value survives the read and dies on the write", func(t *testing.T) {
		const src = "k: !!str Null\n"

		var got any
		require.NoError(t, yaml.Unmarshal([]byte(src), &got))
		assert.Equal(t, map[string]any{"k": "Null"}, got, "the decode is right")

		var again any
		require.NoError(t, yaml.Unmarshal([]byte(renderOnce(t, src)), &again))
		assert.Equal(t, map[string]any{"k": "null"}, again, "today: the render loses it")
	})

	t.Run("the boolean spellings are untouched", func(t *testing.T) {
		for _, src := range []string{"k: !!str True\n", "k: !!str FALSE\n"} {
			wellFormed(t, src)
			assert.Equal(t, src, renderOnce(t, src), "%q", src)
		}
	})
}
