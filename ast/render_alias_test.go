// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package ast_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/parser"
)

// TestRenderResolvesAnAliasFromTheTree covers an alias the caller's table does
// not hold.
//
// The renderer took every target from the map it was given. The decoder builds
// that map from a whole file and then renders one node out of it, so an alias
// whose anchor stood elsewhere rendered as "*x" -- the reference a custom
// UnmarshalYAML has no way to look up, which is the thing the option exists to
// avoid. The parser fills AliasNode.Target where the alias stands, so the node
// carries the answer.
func TestRenderResolvesAnAliasFromTheTree(t *testing.T) {
	f, err := parser.ParseBytes([]byte("anchored: &x\n  a: 1\nelsewhere: *x\n"))
	require.NoError(t, err)

	body, ok := f.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)
	require.Len(t, body.Values, 2)

	// An empty table, which is what a caller holding one node has.
	renderer := ast.NewRenderer(ast.WithAliasTargets(map[string]ast.Node{}))
	assert.Equal(t, "a: 1", renderer.String(body.Values[1].Value))

	// Without the option an alias still renders as it was written.
	assert.Equal(t, "*x", ast.NewRenderer().String(body.Values[1].Value))
}

// TestRenderStopsOnACycle covers a target that reaches back to the node holding
// it, which the parser now builds.
//
// Resolution expands the alias once and then falls back to the reference,
// because the name is already being rendered. So "&x [ *x ]" comes out as
// "&x [[*x]]" and not as an exhausted stack. It is not the document that went
// in: resolving an alias never round-trips, since the alias is written as the
// value it stands for.
func TestRenderStopsOnACycle(t *testing.T) {
	f, err := parser.ParseBytes([]byte("a: &x [ *x ]\n"))
	require.NoError(t, err)

	anchored := f.Docs[0].Body.(*ast.MappingNode).Values[0].Value

	renderer := ast.NewRenderer(ast.WithAliasTargets(map[string]ast.Node{}))
	assert.Equal(t, "&x [[*x]]", renderer.String(anchored))

	// Left alone it is the document that went in.
	assert.Equal(t, "&x [*x]", ast.NewRenderer().String(anchored))
}
