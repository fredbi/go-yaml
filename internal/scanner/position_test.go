// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/token"
)

// TestBlockScalarPositionDoesNotDependOnWhatFollows holds the two paths that end a block scalar to the same position.
//
// A block ends either at the end of the source, through emitMultiLine, or at a dedent, through
// Scanner.bufferedToken. Both report where the content began. The second used to work the position out again and get
// it wrong for a folded scalar: "a: >\n  fold\n  more\n" reported line 2 column 3, and the same scalar with "b: 1"
// after it reported line 3 column 0.
func TestBlockScalarPositionDoesNotDependOnWhatFollows(t *testing.T) {
	for _, tc := range []struct {
		name  string
		alone string
		then  string
	}{
		{"folded", "a: >\n  fold\n  more\n", "a: >\n  fold\n  more\nb: 1\n"},
		{"literal", "a: |\n  one\n  two\n", "a: |\n  one\n  two\nb: 1\n"},
		{"folded, indentation indicator", "a: >2\n   x\n   y\n", "a: >2\n   x\n   y\nb: 1\n"},
		{"literal, kept breaks", "a: |+\n  x\n\n", "a: |+\n  x\n\nb: 1\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			alone := blockContentToken(t, tc.alone)
			then := blockContentToken(t, tc.then)

			assert.Equalf(t, alone.Value, then.Value, "the value must not depend on what follows the block")
			assert.Equalf(t, alone.Position.Line, then.Position.Line, "line moved when %q was added after the block", "b: 1")
			assert.Equalf(t, alone.Position.Column, then.Position.Column, "column moved when a key was added after the block")
			assert.Equalf(t, alone.Position.Offset(), then.Position.Offset(), "offset moved when a key was added after the block")
			assert.GreaterOrEqualf(t, then.Position.Column, int32(1), "a column addresses the source and counts from 1")
		})
	}
}

// blockContentToken returns the string token holding a block scalar's content, which follows its header.
func blockContentToken(t *testing.T, src string) token.Token {
	t.Helper()

	tokens := tokenize(t, src)
	for i, tk := range tokens {
		switch tk.Type {
		case token.LiteralType, token.FoldedType:
			require.Greaterf(t, len(tokens), i+1, "%q: the block scalar header is the last token", src)

			return tokens[i+1]
		default:
		}
	}
	t.Fatalf("%q: holds no block scalar header", src)

	return token.Token{}
}
