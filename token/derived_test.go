// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package token_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/lexer"
	"github.com/go-openapi/go-yaml/token"
)

// TestIndicatorFollowsFromType checks that a token's indicator and character
// type can be read off its type, over every token the YAML Test Suite produces.
//
// They used to be recorded on every token, 16 bytes each carried for the whole
// life of a document to say something its type already said.
func TestIndicatorFollowsFromType(t *testing.T) {
	sources := suiteSources(t)
	require.NotEmpty(t, sources)

	var checked int
	for _, src := range sources {
		for _, tk := range lexer.Tokenize(src) {
			checked++
			assert.Equalf(t, tk.Type.Indicator(), tk.Indicator,
				"%s %q: indicator does not follow from the type", tk.Type, tk.Value)
			assert.Equalf(t, tk.Type.CharacterType(), tk.CharacterType,
				"%s %q: character type does not follow from the type", tk.Type, tk.Value)
		}
	}
	t.Logf("checked %d tokens", checked)
}

// TestEveryTypeHasAnIndicator checks the derivation covers every type, so a
// type added later cannot quietly fall through to NotIndicator.
func TestEveryTypeHasAnIndicator(t *testing.T) {
	indicators := map[token.Type]token.Indicator{
		token.SequenceEntryType: token.BlockStructureIndicator,
		token.MappingKeyType:    token.BlockStructureIndicator,
		token.MappingValueType:  token.BlockStructureIndicator,
		token.CollectEntryType:  token.FlowCollectionIndicator,
		token.SequenceStartType: token.FlowCollectionIndicator,
		token.SequenceEndType:   token.FlowCollectionIndicator,
		token.MappingStartType:  token.FlowCollectionIndicator,
		token.MappingEndType:    token.FlowCollectionIndicator,
		token.CommentType:       token.CommentIndicator,
		token.AnchorType:        token.NodePropertyIndicator,
		token.AliasType:         token.NodePropertyIndicator,
		token.TagType:           token.NodePropertyIndicator,
		token.LiteralType:       token.BlockScalarIndicator,
		token.FoldedType:        token.BlockScalarIndicator,
		token.SingleQuoteType:   token.QuotedScalarIndicator,
		token.DoubleQuoteType:   token.QuotedScalarIndicator,
		token.DirectiveType:     token.DirectiveIndicator,
	}

	for typ, want := range indicators {
		assert.Equalf(t, want, typ.Indicator(), "%s", typ)
		assert.Equalf(t, token.CharacterTypeIndicator, typ.CharacterType(), "%s", typ)
	}

	assert.Equal(t, token.CharacterTypeWhiteSpace, token.SpaceType.CharacterType())
	assert.Equal(t, token.CharacterTypeInvalid, token.InvalidType.CharacterType())
	assert.Equal(t, token.CharacterTypeMiscellaneous, token.StringType.CharacterType())
	assert.Equal(t, token.NotIndicator, token.StringType.Indicator())
}
