// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package ast_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/codec"
	"github.com/go-openapi/go-yaml/parser"
	"github.com/go-openapi/go-yaml/token"
)

// TestABlockScalarBelowItsKeyStatesTheWidthFromTheMapping checks the width a
// block scalar header states when File.String writes it on a line below its key.
//
// Section 8.1.1.1 counts the indentation indicator from the node enclosing the
// scalar, not from the line the header stands on. "k: # c" over "  |2-" over
// "   x" reads " x": the mapping sits in column 0, so the content starts at
// column 2 and the third space is part of the value. Written back with the
// header two columns in and the content four, "|2-" dropped two more spaces
// into the value, and each rendering added two again.
func TestABlockScalarBelowItsKeyStatesTheWidthFromTheMapping(t *testing.T) {
	for name, src := range map[string]string{
		"a comment on the key":            "k: # c\n  |2-\n   x\n",
		"a comment above the value":       "k:\n  # c\n  |2-\n   x\n",
		"an explicit key":                 "? k\n: # c\n  |2-\n   x\n",
		"a nested mapping":                "a:\n  k: # c\n    |2-\n     x\n",
		"an anchor in front":              "k: # c\n  &a |2-\n   x\n",
		"an anchor on the key's line":     "k: &a\n  # c\n  |2-\n   x\n",
		"a tag in front":                  "k: # c\n  !!str |1\n  x\n",
		"a folded scalar":                 "k: # c\n  >2\n    x\n   y\n",
		"no indicator, for contrast":      "k: # c\n  |\n   x\n",
		"the header on the key's line":    "k: |1\n  x\n",
		"the document's own block scalar": "--- &a |1\n x\n",
	} {
		t.Run(name, func(t *testing.T) {
			f, err := parser.ParseBytes([]byte(src), parser.WithComments())
			require.NoError(t, err)

			assertRenderingKeepsTheValue(t, src, f.String())
		})
	}

	t.Run("a comment set on the key", func(t *testing.T) {
		// The same shape reached by an edit: the key's comment claims the rest of
		// its line, so the block scalar opens the next one.
		const src = "k: |2-\n  x\n"
		f, err := parser.ParseBytes([]byte(src), parser.WithComments())
		require.NoError(t, err)

		mapping, ok := f.Docs[0].Body.(*ast.MappingNode)
		require.True(t, ok)
		comment := ast.CommentGroup([]*token.Token{{Type: token.CommentType, Value: "c"}})
		require.NoError(t, mapping.Values[0].Key.SetComment(comment))

		assertRenderingKeepsTheValue(t, src, f.String())
	})
}

// assertRenderingKeepsTheValue checks that rendered reads as src does and
// renders to itself.
func assertRenderingKeepsTheValue(t *testing.T, src, rendered string) {
	t.Helper()

	var want, got any
	require.NoError(t, codec.Unmarshal([]byte(src), &want))
	require.NoErrorf(t, codec.Unmarshal([]byte(rendered), &got), "%q renders to %q", src, rendered)
	assert.Equalf(t, want, got, "%q renders to %q", src, rendered)

	again, err := parser.ParseBytes([]byte(rendered), parser.WithComments())
	require.NoError(t, err)
	assert.Equalf(t, rendered, again.String(), "%q renders to %q, which does not settle", src, rendered)
}
