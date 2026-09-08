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

// keyOf is the first key of the document's root mapping.
func keyOf(t *testing.T, src string) ast.Node {
	t.Helper()

	f, err := parser.ParseBytes([]byte(src), parser.WithAllowDuplicateMapKey())
	require.NoErrorf(t, err, "%q", src)
	require.NotEmptyf(t, f.Docs, "%q", src)

	body, ok := f.Docs[len(f.Docs)-1].Body.(*ast.MappingNode)
	if !ok {
		entry, single := f.Docs[len(f.Docs)-1].Body.(*ast.MappingValueNode)
		require.Truef(t, single, "%q holds no mapping", src)

		return entry.Key
	}
	require.NotEmptyf(t, body.Values, "%q", src)

	return body.Values[0].Key
}

// TestKeyIdentityReadsWhatAKeyResolvesTo holds spellings of one key to one
// answer, and keeps different keys apart.
//
// 3.2.1.1 makes two keys equal when they resolve to the same node. The text a
// document wrote is not that: "[a]", "[ a ]", "[a,]" and '["a"]' are four
// spellings of one sequence holding one string. Naming a key by its first token
// is worse -- every sequence key becomes "[", every block scalar key "|-".
func TestKeyIdentityReadsWhatAKeyResolvesTo(t *testing.T) {
	t.Run("spellings of one key agree", func(t *testing.T) {
		for _, group := range [][]string{
			{"[a]: 1\n", "[ a ]: 2\n", "[a,]: 3\n", "[\"a\"]: 4\n", "['a']: 5\n", "[ a , ]: 6\n"},
			{"{a: 1}: x\n", "{ a : 1 }: y\n", "{\"a\": 1}: z\n"},
			{"? - a\n: 1\n", "? [a]\n: 2\n", "?\n  - a\n: 3\n"},
			{"a: 1\n", "\"a\": 2\n", "'a': 3\n"},
			{"1: x\n", "01: y\n", "0x1: z\n"},
			{"? |-\n  a\n: 1\n", "? >-\n  a\n: 2\n", "a: 3\n"},
		} {
			want := ast.KeyIdentity(keyOf(t, group[0]))
			require.NotEmptyf(t, want, "%q has no identity", group[0])
			for _, src := range group[1:] {
				assert.Equalf(t, want, ast.KeyIdentity(keyOf(t, src)),
					"%q and %q spell one key", group[0], src)
			}
		}
	})

	t.Run("different keys stay apart", func(t *testing.T) {
		for _, pair := range [][2]string{
			{"[a]: 1\n", "[b]: 2\n"},
			{"[a]: 1\n", "[a, b]: 2\n"},
			{"[a]: 1\n", "{a: 1}: 2\n"},
			{"[[a]]: 1\n", "[a]: 2\n"},
			{"{a: 1}: x\n", "{a: 2}: y\n"},
			{"{a: 1}: x\n", "{b: 1}: y\n"},
			{"1: x\n", "\"1\": y\n"},
			{"1: x\n", "1.0: y\n"},
			// A tag names the type, so it changes the identity only where the
			// plain spelling resolves to another type: "a" is a string either
			// way, and "1" is an integer plain and a string tagged.
			{"1: x\n", "!!str 1: y\n"},
			// Chomping is part of the string a block scalar holds.
			{"? |-\n  a\n: 1\n", "? |\n  a\n: 2\n"},
			{"? - a\n: 1\n", "? - b\n: 2\n"},
		} {
			left, right := ast.KeyIdentity(keyOf(t, pair[0])), ast.KeyIdentity(keyOf(t, pair[1]))
			require.NotEmptyf(t, left, "%q", pair[0])
			assert.NotEqualf(t, left, right, "%q and %q are two keys", pair[0], pair[1])
		}
	})

	t.Run("an alias resolves to what it names", func(t *testing.T) {
		// The parser fills AliasNode.Target as it reads, so this needs no
		// anchor table of its own.
		f, err := parser.ParseBytes([]byte("x: &a [1]\n? *a\n: 1\n"))
		require.NoError(t, err)
		body, ok := f.Docs[0].Body.(*ast.MappingNode)
		require.True(t, ok)
		require.Len(t, body.Values, 2)

		assert.Equal(t, ast.KeyIdentity(keyOf(t, "[1]: 1\n")),
			ast.KeyIdentity(body.Values[1].Key),
			"an alias names the sequence its anchor named")
	})

	t.Run("a node with nothing to say has no identity", func(t *testing.T) {
		assert.True(t, ast.Unnamed(ast.KeyIdentity(nil)))
		assert.True(t, ast.Unnamed(ast.KeyIdentity(&ast.AliasNode{})))
	})
}
