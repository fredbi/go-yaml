// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
)

// TestTwoCollectionKeysAreTwoKeys is the key-identity rule where the key is not
// a scalar.
//
// Parser.mapKeyIdentity names a key by its text and its type, and unwraps the
// wrappers it knows -- an explicit key, an anchor, a tag. An alias hands back
// nothing on purpose, since the load resolves what it names. A sequence or a
// mapping fell past all of that to the node's own first token, so every
// sequence key was named "[" and every mapping key "{": "{[a]: 1, [b]: 2}" was
// refused as `mapping key "[" already defined`, two keys sharing not one
// character between them.
//
// The block spelling was always read, so the two disagreed as well.
//
// A collection has no name to be had -- two of them repeat a key when their
// contents match, which is a comparison of trees and not of text -- so it hands
// back nothing, as the alias does, and recordKeyOnce records neither.
//
// grammar.NewRecognizer reads every document here. go.yaml.in/yaml/v3 v3.0.5
// and libfyaml 1.0.0b1 both parse them and then refuse to hold a collection as
// a map key, which is a value model declining rather than a syntax verdict.
func TestTwoCollectionKeysAreTwoKeys(t *testing.T) {
	t.Run("two collection keys read, in flow and in block", func(t *testing.T) {
		for _, src := range []string{
			"{[a]: 1, [b]: 2}\n",
			"{{a: 1}: x, {b: 2}: y}\n",
			"? [a]\n: 1\n? [b]\n: 2\n",
			"{[a]: 1, a: 2}\n",
		} {
			var got any
			require.NoErrorf(t,
				codec.UnmarshalWithOptions([]byte(src), &got, codec.UseOrderedMap()), "%q", src)
			assert.Lenf(t, got, 2, "%q read %v", src, got)
		}
	})

	t.Run("one collection key still reads", func(t *testing.T) {
		var got any
		require.NoError(t, codec.UnmarshalWithOptions([]byte("{[a]: 1}\n"), &got, codec.UseOrderedMap()))
		assert.Len(t, got, 1)
	})

	t.Run("a collection key repeated is refused, in every spelling", func(t *testing.T) {
		// 913fb19 stopped naming a collection key and so stopped checking it,
		// and the walking reader then folded two entries into one and dropped
		// the first value without a word. The check that commit ran to prove it
		// had not over-reached -- "<<: {a: 1, a: 2}" still refused -- was a
		// repeat INSIDE the key rather than a repeat OF the key, which is why
		// it passed while this went out.
		for _, src := range []string{
			"{{a: 0}: 1, {a: 0}: 2}\n",
			"{[a]: 1, [a]: 2}\n",
			"{[\"\"]: 1, [\"\"]: 2}\n",
			"{[a, b]: 1, [a, b]: 2}\n",
			"? [a]\n: 1\n? [a]\n: 2\n",
			"? {a: 0}\n: 1\n? {a: 0}\n: 2\n",
			"[a]: 1\n[a]: 2\n",
			"? [a]\n: 1\n[a]: 2\n",
		} {
			var got any
			err := codec.UnmarshalWithOptions([]byte(src), &got, codec.UseOrderedMap())
			require.Errorf(t, err, "%q read %v", src, got)
			assert.ErrorIsf(t, err, yamlerrors.ErrDuplicateKey, "%q", src)
		}
	})

	t.Run("and two collections that differ are still two keys", func(t *testing.T) {
		// The name is the document's own spelling, taken from the source rather
		// than from the node: mapKeyIdentity runs before a collection's
		// children are hung on it, so String() renders "[]" and "{}" and every
		// sequence key collided with every other. The suite caught that --
		// spec-example-2-11-mapping-between-sequences has two sequence keys.
		for _, src := range []string{
			"{{a: 0}: 1, {a: 1}: 2}\n",
			"{[a]: 1, [b]: 2}\n",
			"{[a]: 1, [a, b]: 2}\n",
			"{{\"\": 0}: a, {\"\": 1}: b}\n",
			"? [a]\n: 1\n? [b]\n: 2\n",
			"? - Detroit Tigers\n  - Chicago cubs\n: 1\n? [ New York Yankees ]\n: 2\n",
		} {
			var got any
			require.NoErrorf(t,
				codec.UnmarshalWithOptions([]byte(src), &got, codec.UseOrderedMap()), "%q", src)
			assert.Lenf(t, got, 2, "%q read %v", src, got)
		}
	})

	t.Run("and a scalar key repeated is still refused", func(t *testing.T) {
		// The skip is for keys mapKeyIdentity gave up on. Everything it can
		// name is recorded, and a key written empty is one of those: a quoted
		// "" is the string, the empty node is "null", and neither is the
		// nothing a collection hands back.
		for _, src := range []string{
			"{a: 1, a: 2}\n",
			"a: 1\na: 2\n",
			"{\"\": 1, \"\": 2}\n",
			"? a\n: 1\n? a\n: 2\n",
		} {
			var got any
			err := codec.UnmarshalWithOptions([]byte(src), &got, codec.UseOrderedMap())
			require.Errorf(t, err, "%q read %v", src, got)
			assert.ErrorIsf(t, err, yamlerrors.ErrDuplicateKey, "%q", src)
		}
	})

	t.Run("an empty key of either spelling still reads", func(t *testing.T) {
		for _, src := range []string{"{\"\": 1}\n", ": 1\n"} {
			var got any
			require.NoErrorf(t,
				codec.UnmarshalWithOptions([]byte(src), &got, codec.UseOrderedMap()), "%q", src)
			assert.Lenf(t, got, 1, "%q", src)
		}
	})
}
