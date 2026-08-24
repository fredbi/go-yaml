// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/parser"
)

// paths returns every node's path, in walk order.
func paths(t *testing.T, src string) []string {
	t.Helper()

	f, err := parser.ParseBytes([]byte(src), parser.ParseComments)
	require.NoError(t, err)

	var got []string
	for _, doc := range f.Docs {
		ast.Walk(pathCollector(func(n ast.Node) { got = append(got, n.GetPath()) }), doc)
	}

	return got
}

type pathCollector func(ast.Node)

func (v pathCollector) Visit(n ast.Node) ast.Visitor { v(n); return v }

// TestNodePathsAreRendered checks the path every node reports.
//
// The paths are a trie of steps rather than a string per node, so the check
// that matters is the rendering: a step into a mapping, a step into a sequence,
// and a key holding a character YAMLPath reads as syntax.
//
// The first entry of each want is the *ast.DocumentNode, which carries no path
// and never has.
func TestNodePathsAreRendered(t *testing.T) {
	tests := map[string]struct {
		src  string
		want []string
	}{
		"a mapping": {
			src:  "foo:\n  bar: 1\n",
			want: []string{"", "$", "$.foo", "$.foo", "$.foo", "$.foo.bar", "$.foo.bar", "$.foo.bar"},
		},
		"a sequence": {
			src:  "foo:\n  - a\n  - b\n",
			want: []string{"", "$", "$.foo", "$.foo", "$.foo", "$.foo[0]", "$.foo[1]"},
		},
		"a sequence of mappings": {
			src:  "- a: 1\n- a: 2\n",
			want: []string{"", "$", "$[0]", "$[0].a", "$[0].a", "$[0].a", "$[1]", "$[1].a", "$[1].a", "$[1].a"},
		},
		"a key holding a dot": {
			src:  "a.b: 1\n",
			want: []string{"", "$", "$.'a.b'", "$.'a.b'", "$.'a.b'"},
		},
		"a key holding a bracket": {
			src:  "a[0]: 1\n",
			want: []string{"", "$", "$.'a[0]'", "$.'a[0]'", "$.'a[0]'"},
		},
		"a key holding a dollar": {
			src:  "$ref: 1\n",
			want: []string{"", "$", "$.'$ref'", "$.'$ref'", "$.'$ref'"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.want, paths(t, test.src))
		})
	}
}

// TestNodePathRendersIndexesPastOneDigit checks the width the renderer reserves
// for an index, which is counted digit by digit.
func TestNodePathRendersIndexesPastOneDigit(t *testing.T) {
	got := paths(t, "foo: [0,1,2,3,4,5,6,7,8,9,10,11]\n")

	assert.Contains(t, got, "$.foo[0]")
	assert.Contains(t, got, "$.foo[9]")
	assert.Contains(t, got, "$.foo[10]")
	assert.Contains(t, got, "$.foo[11]")
}

// TestSetPathOverridesTheRecordedPath checks that a path handed in by a caller
// reads back exactly, rather than being folded into the trie.
func TestSetPathOverridesTheRecordedPath(t *testing.T) {
	f, err := parser.ParseBytes([]byte("foo: 1\n"), 0)
	require.NoError(t, err)

	node := f.Docs[0].Body
	require.Equal(t, "$", node.GetPath())

	node.SetPath("$.anywhere['at all']")
	assert.Equal(t, "$.anywhere['at all']", node.GetPath())
}
