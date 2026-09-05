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
// correctly and cost quadratic time doing it. The last four came from [Tagged]
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
		wellFormed(t, "a: !!str &a1 5\nb: *a1\n")

		var got any
		require.NoError(t, yaml.Unmarshal([]byte("a: !!str &a1 5\nb: *a1\n"), &got))
		assert.Equal(t, map[string]any{"a": "5", "b": uint64(5)}, got,
			"today: one node, read as a string where it stands and as a number through the alias")

		got = nil
		require.NoError(t, yaml.Unmarshal([]byte("a: &a1 !!str 5\nb: *a1\n"), &got))
		assert.Equal(t, map[string]any{"a": "5", "b": "5"}, got)
	})
}

// TestDefectStrTagResolvesFirst: `!!str` is meant to settle what a plain scalar
// is. Instead the scalar is resolved and the result turned back into text, so
// the spelling that went in is not the one that comes out.
//
// The library disagrees with itself about it, which is what makes this a defect
// rather than a reading of the spec: the verbatim spelling of the very same tag
// gives the text.
func TestDefectStrTagResolvesFirst(t *testing.T) {
	respelt := map[string]string{
		"!!str null\n":  "",
		"!!str Null\n":  "",
		"!!str NULL\n":  "",
		"!!str ~\n":     "",
		"!!str True\n":  "true",
		"!!str TRUE\n":  "true",
		"!!str False\n": "false",
		"!!str FALSE\n": "false",
	}

	for src, today := range respelt {
		wellFormed(t, src)

		var got any
		require.NoError(t, yaml.Unmarshal([]byte(src), &got))
		assert.Equal(t, today, got, "today: %q reads as %q, not as the text", src, today)
	}

	t.Run("every other spelling of the same tag gives the text", func(t *testing.T) {
		for _, src := range []string{
			"!<tag:yaml.org,2002:str> null\n", "! null\n", "!foo null\n", "!!str \"null\"\n",
		} {
			var got any
			require.NoError(t, yaml.Unmarshal([]byte(src), &got))
			assert.Equal(t, "null", got, "%q", src)
		}
	})

	t.Run("and a scalar that does not resolve is untouched", func(t *testing.T) {
		for src, want := range map[string]string{
			"!!str 5\n": "5", "!!str on\n": "on", "!!str true\n": "true", "!!str x\n": "x",
		} {
			var got any
			require.NoError(t, yaml.Unmarshal([]byte(src), &got))
			assert.Equal(t, want, got, "%q", src)
		}
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
