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
// The first two came from [yamlgen.Style.Break] on 2026-09-03, the axis that
// writes one document with LF, CRLF and a lone CR. Neither shape is exotic and
// neither was reachable before it. The third came from [yamlgen.DeepDocument]
// the same day, and is the one no verdict could have found: the documents parse
// correctly and cost quadratic time doing it.

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

	file, err := parser.ParseBytes([]byte(src), parser.Comments())
	require.NoError(t, err)

	return file.String()
}

// TestDefectFoldedScalarCopiesTheSourceBreak: rendering a document written with
// CRLF or a lone CR copies that break into the folded block scalar it writes,
// instead of writing the renderer's own \n.
//
// Three consequences, and the third is the one no round trip could have found.
// The document never settles, because the second render writes \n where the
// first wrote \r\n. With a lone CR the content lines also drift one column
// right, so a fold stops folding and the value changes. And in a nested
// position the content lands at the parent's own column, which is not a block
// scalar at all -- the grammar refuses the result.
//
// `|` is unaffected: a literal scalar's content is written from the value,
// which has no breaks of the source's kind left in it.
func TestDefectFoldedScalarCopiesTheSourceBreak(t *testing.T) {
	t.Run("the break comes through and the document does not settle", func(t *testing.T) {
		const src = ">-\r\n x\r\n"
		wellFormed(t, src)

		once := renderOnce(t, src)
		assert.Equal(t, ">-\r\n  x\n", once, "today: the source's CRLF, then the renderer's LF")
		assert.Equal(t, ">-\n  x\n", renderOnce(t, once), "and a third document on the next pass")
	})

	t.Run("a lone CR reindents the lines after the first", func(t *testing.T) {
		const src = ">-\r x\r y\r"
		wellFormed(t, src)

		var before, after any
		require.NoError(t, yaml.Unmarshal([]byte(src), &before))
		assert.Equal(t, "x y", before)

		once := renderOnce(t, src)
		assert.Equal(t, ">-\r  x\r   y\n", once, "today: ` y` is indented one further than ` x`")

		require.NoError(t, yaml.Unmarshal([]byte(once), &after))
		assert.Equal(t, "x\n y", after, "today: the deeper line is more-indented, so it stops folding")
	})

	t.Run("nested, the rendering is not YAML at all", func(t *testing.T) {
		const src = "a:\r  b: >\r   x\rc: 1\r"
		wellFormed(t, src)

		once := renderOnce(t, src)
		assert.Equal(t, "a:\n  b: >\r  x\nc: 1\n", once,
			"today: the content sits at b's own column, so it is not the scalar's content")
		assert.False(t, grammar.NewRecognizer(1024).Stream([]byte(once)).OK,
			"today: the renderer wrote something no conforming parser reads")
	})
}

// TestDefectCRLFBlanksALineAboveAStandaloneComment: a CRLF source gains a blank
// line above a comment on its own line.
//
// It takes both halves. A document with standalone comments and no bare entry
// renders unchanged, and so does one with a bare entry and no comment; the
// blank line appears only when a `-` or a `key:` is the whole line somewhere in
// the document. A lone CR is unaffected.
func TestDefectCRLFBlanksALineAboveAStandaloneComment(t *testing.T) {
	const src = "# c1\r\n-\r\n# c2\r\n- 1\r\n"
	wellFormed(t, src)

	once := renderOnce(t, src)
	assert.Equal(t, "# c1\n- \n\n# c2\n- 1\n", once, "today: a blank line above # c2")
	assert.Equal(t, "# c1\n- \n# c2\n- 1\n", renderOnce(t, once), "and it is gone again on the next pass")

	t.Run("neither half alone", func(t *testing.T) {
		for name, only := range map[string]string{
			"no bare entry":       "# c1\r\n- 0\r\n# c2\r\n- 1\r\n",
			"no comment":          "-\r\n- 1\r\n",
			"a lone CR, not CRLF": "# c1\r-\r# c2\r- 1\r",
		} {
			t.Run(name, func(t *testing.T) {
				wellFormed(t, only)
				once := renderOnce(t, only)
				assert.Equal(t, once, renderOnce(t, once), "settles in one pass")
			})
		}
	})
}
