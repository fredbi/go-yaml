// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/token"
)

func TestEscapeSingleQuote(t *testing.T) {
	assert.Equal(t, `'Victor''s victory'`, escapeSingleQuote("Victor's victory"))
}

func TestDocumentNodeGetTokenWithoutBody(t *testing.T) {
	// A document with no content has no token to point at. Reported by fuzzing
	// the AST walk: an empty source is enough to reach this, and the walk is
	// what a consumer looking for positions does.
	doc := &DocumentNode{BaseNode: BaseNode{}}

	require.NotPanics(t, func() {
		assert.Nil(t, doc.GetToken())
	})
}

func TestReadNode(t *testing.T) {
	t.Run("utf-8", func(t *testing.T) {
		const value = "éɛทᛞ⠻チ▓🦄"
		file := &File{Docs: []*DocumentNode{{Body: &StringNode{
			Token: &token.Token{Value: value},
			Value: value,
		}}}}

		buffer := make([]byte, len(value))
		size, err := file.Read(buffer)
		require.NoError(t, err)

		assert.Equal(t, len(value), size)
		assert.Equal(t, value, string(buffer))
	})
}
