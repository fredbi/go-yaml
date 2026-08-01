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

func TestReadNode(t *testing.T) {
	t.Run("utf-8", func(t *testing.T) {
		const value = "éɛทᛞ⠻チ▓🦄"
		node := &StringNode{
			BaseNode: &BaseNode{},
			Token:    &token.Token{},
			Value:    value,
		}

		buffer := make([]byte, len(value))
		size, err := readNode(buffer, node)
		require.NoError(t, err)

		assert.Equal(t, len(value), size)
		assert.Equal(t, value, string(buffer))
	})
}
