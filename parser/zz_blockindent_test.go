// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/parser"
)

// TestABlockScalarUnderAnEmptyKeyIsMeasuredFromItsColon covers where a block
// scalar's indentation indicator counts from when the entry holding it has no
// key.
//
// 8.1.1.1 makes the indicator count from the parent node's indentation, and
// "- : |1" over "   x" is a sequence entry holding a mapping whose key is the
// empty node. The mapping sits at the ':' in column 3, so the content is
// measured from there and reads "x".
//
// scanMapValue took the column from the last content token on the line, and a
// '-' is not one, so the entry kept the sequence's own column 1 and the content
// read as "  x" -- two spaces that were the entry's indentation. Every
// rendering then wrote them out and the next read gained two more, so the value
// grew without bound: "x" to "  x" to "    x". libfyaml 1.0.0b1 reads "x".
//
// A key of any kind hides it, because the key is a content token that says
// where the entry sits: "- \"\": |1" and "- a: |1" were both correct.
//
// The same empty key under a mapping rather than a sequence entry -- "k:" over
// "  : |1" -- is not here on purpose. It was refused outright, with "unexpected
// scalar value", until keyWindow.hasNoKey stopped keying a ':' on the entry
// above it, and it belongs to that fix and its tests. Asserting it here would
// make this test pass only with that commit already applied.
func TestABlockScalarUnderAnEmptyKeyIsMeasuredFromItsColon(t *testing.T) {
	t.Run("the content is measured from the colon", func(t *testing.T) {
		for _, tc := range []struct{ name, src, want string }{
			{name: "an empty key in a sequence entry", src: "- : |1\n   x\n", want: "x\n"},
			{name: "folded reads the same way", src: "- : >1\n   x\n", want: "x\n"},
			// The rule is the same when the block scalar stands on its own
			// line -- the ':' at column 3, so content at column 4 -- and the
			// content is written one column further right there, so one space
			// of it survives. libfyaml 1.0.0b1 reads " x" too.
			{name: "the block scalar on its own line", src: "- :\n   |1\n    x\n", want: " x\n"},

			// The shapes that were already right, kept so a fix to the one
			// above cannot move them.
			{name: "a quoted empty key", src: "- \"\": |1\n   x\n", want: "x\n"},
			{name: "a named key", src: "- a: |1\n   x\n", want: "x\n"},
			{name: "an empty key at the root", src: ": |1\n  x\n", want: " x\n"},
			{name: "no indicator at all", src: "- : |\n   x\n", want: "x\n"},
			{name: "a named key with the scalar below it", src: "k:\n  |1\n   x\n", want: "  x\n"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				assert.Equal(t, tc.want, blockScalarOf(t, tc.src), "%q", tc.src)
			})
		}
	})

	t.Run("rendering settles, and the value with it", func(t *testing.T) {
		// The compounding is what made this worth fixing rather than filing: a
		// document that grows by two spaces every rendering has no fixed point,
		// so nothing that writes a document back can be trusted with it.
		for _, src := range []string{
			"- : |1\n   x\n",
			"- : |1\n   \n",
			"- : >1\n   x\n",
			"- : |2\n    x\n",
			"- :\n   |1\n    x\n",
		} {
			once := renderDocument(t, src)
			twice := renderDocument(t, once)
			assert.Equal(t, once, twice, "%q renders to %q and then to %q", src, once, twice)

			assert.Equal(t, blockScalarOf(t, src), blockScalarOf(t, once),
				"%q: rendering keeps the value", src)
		}
	})

	t.Run("a mapping nested straight in a sequence entry is refused", func(t *testing.T) {
		// "- - : |1" is not YAML 1.2 -- the grammar refuses it and so does
		// libfyaml 1.0.0b1 -- and it was read here while the entry took the
		// sequence's column. Closing that took the laxity with it.
		_, err := parser.ParseBytes([]byte("- - : |1\n    x\n"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "non-map value is specified")
	})
}

// blockScalarOf is the block scalar the first entry of the document holds,
// whatever depth of sequence and mapping stands around it.
func blockScalarOf(t *testing.T, src string) string {
	t.Helper()

	f, err := parser.ParseBytes([]byte(src))
	require.NoErrorf(t, err, "%q", src)
	require.NotEmptyf(t, f.Docs, "%q holds no document", src)

	var found *ast.LiteralNode
	var visit visitorFunc
	visit = func(n ast.Node) ast.Visitor {
		if lit, ok := n.(*ast.LiteralNode); ok && found == nil {
			found = lit
		}

		return visit
	}
	ast.Walk(visit, f.Docs[0])

	require.NotNilf(t, found, "%q holds no block scalar", src)
	require.NotNilf(t, found.Value, "%q: the block scalar holds nothing", src)

	return found.Value.Value
}

// renderDocument writes the first document of src back out.
func renderDocument(t *testing.T, src string) string {
	t.Helper()

	f, err := parser.ParseBytes([]byte(src), parser.WithComments())
	require.NoErrorf(t, err, "%q", src)

	var b strings.Builder
	require.NoErrorf(t, ast.NewRenderer().Render(&b, f.Docs[0]), "%q", src)

	return b.String() + "\n"
}
