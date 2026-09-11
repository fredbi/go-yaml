// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package ast_test

import (
	"fmt"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/token"
)

// TestArenaResetHandsTheSameCellsOutAgain checks that Arena.Reset hands out the cells of the nodes built before it,
// in the same order and across several blocks, each one zeroed.
func TestArenaResetHandsTheSameCellsOutAgain(t *testing.T) {
	t.Parallel()

	const nodes = 40 // more than two blocks of the smallest size
	a := ast.NewArena(0)

	before := make([]*ast.StringNode, 0, nodes)
	for i := range nodes {
		n := a.String(&token.Token{Value: fmt.Sprintf("old%d", i)})
		require.NoError(t, n.SetComment(ast.CommentGroup([]*token.Token{{Value: "# stale"}})))
		before = append(before, n)
	}

	a.Reset(0)

	for i := range nodes {
		n := a.String(&token.Token{Value: fmt.Sprintf("new%d", i)})
		require.Samef(t, before[i], n, "cell %d is not handed out again", i)
		assert.Equal(t, fmt.Sprintf("new%d", i), n.Value)
		assert.Nilf(t, n.GetComment(), "cell %d keeps the comment of the node built before Reset", i)
	}
}
