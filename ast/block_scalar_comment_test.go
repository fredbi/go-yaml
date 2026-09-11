// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package ast_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/codec"
	"github.com/go-openapi/go-yaml/parser"
	"github.com/go-openapi/go-yaml/token"
)

// TestACommentGroupOnABlockScalarGoesAboveItsEntry checks where both renderers
// write a comment of more than one line set on a block scalar.
//
// The header's line takes one comment, and the line after it is content. Written
// there, "k: |2- #c" over "#d" over "  x" ended the scalar at "#d" and did not
// parse, and the verbatim copy wrote "k: |2- #c #d", which reads back as one
// comment. The group goes above the line the header stands on, each comment on a
// line of its own, and the value stays as it was.
func TestACommentGroupOnABlockScalarGoesAboveItsEntry(t *testing.T) {
	for name, src := range map[string]string{
		"a mapping value":                    "k: |2-\n  x\n",
		"between two entries":                "a: 1\nk: |2-\n  x\nb: 2\n",
		"a sequence entry":                   "- |2-\n   x\n- y\n",
		"the document's own node":            "|1-\n x\n",
		"after the document marker":          "--- |1-\n x\n",
		"an anchor in front":                 "k: &a |2-\n  x\n",
		"a tag in front":                     "k: !!str |2-\n  x\n",
		"an explicit key":                    "? |-\n  k\n: v\n",
		"an explicit key's value":            "? k\n: |-\n  v\n",
		"a sequence under a key":             "k:\n- |-\n  x\n",
		"a mapping in a sequence entry":      "- k: |-\n    x\n",
		"below a comment the document wrote": "# head\nk: |-\n  x\n",
		"a folded scalar":                    "k: >-\n  x\n  y\n",
	} {
		t.Run(name, func(t *testing.T) {
			var want any
			require.NoError(t, codec.Unmarshal([]byte(src), &want))

			for _, rendering := range []struct {
				name   string
				render func(*ast.File) (string, error)
			}{
				{"File.String", func(f *ast.File) (string, error) { return f.String(), nil }},
				{"VerbatimFile", func(f *ast.File) (string, error) {
					var out bytes.Buffer
					err := ast.NewRenderer(ast.WithSource([]byte(src))).VerbatimFile(&out, f)

					return out.String(), err
				}},
			} {
				t.Run(rendering.name, func(t *testing.T) {
					f, err := parser.ParseBytes([]byte(src), parser.WithComments())
					require.NoError(t, err)
					require.NoError(t, firstBlockScalar(t, f).SetComment(ast.CommentGroup([]*token.Token{
						{Type: token.CommentType, Value: "c"},
						{Type: token.CommentType, Value: "d"},
					})))

					out, err := rendering.render(f)
					require.NoError(t, err)

					var got any
					require.NoErrorf(t, codec.Unmarshal([]byte(out), &got), "%q renders to %q", src, out)
					assert.Equalf(t, want, got, "%q renders to %q", src, out)

					assert.Truef(t, commentsOnLinesOfTheirOwn(out, "#c", "#d"),
						"%q renders to %q, where #c and #d do not stand on consecutive lines of their own", src, out)
				})
			}
		})
	}
}

// commentsOnLinesOfTheirOwn reports whether first and second stand on two
// consecutive lines of out with nothing in front of them but indentation or the
// "- " of the sequence entry they open.
func commentsOnLinesOfTheirOwn(out, first, second string) bool {
	lines := strings.Split(out, "\n")
	for i := 0; i+1 < len(lines); i++ {
		if strings.TrimLeft(lines[i], " -") == first && strings.TrimLeft(lines[i+1], " ") == second {
			return true
		}
	}

	return false
}

// firstBlockScalar returns the first block scalar in the file's first document.
func firstBlockScalar(t *testing.T, f *ast.File) *ast.LiteralNode {
	t.Helper()

	var found *ast.LiteralNode
	ast.Walk(visitEach(func(n ast.Node) {
		if lit, ok := n.(*ast.LiteralNode); ok && found == nil {
			found = lit
		}
	}), f.Docs[0])
	require.NotNil(t, found)

	return found
}

// visitEach is an [ast.Visitor] that calls itself on every node.
type visitEach func(ast.Node)

func (fn visitEach) Visit(n ast.Node) ast.Visitor {
	fn(n)

	return fn
}
