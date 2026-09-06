// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"math/big"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	yaml "github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/testintegration/grammar"
	"github.com/go-openapi/go-yaml/parser"
)

// Shapes the generator found that still diverge.
//
// A case here pins today's behavior rather than the correct behavior, so that a
// fix breaks the test that says it was broken, and the matching entry in
// [yamlgen.Ledger] is what keeps the property tests from failing on it
// meanwhile. When both go the case moves to fixed_test.go with its assertions
// inverted, which is where all of them are now.
//
// Write the next one here. The two helpers below are what a case needs, and
// they are kept for it rather than moved.

// wellFormed asserts src is a YAML 1.2 document before anything is asked of the
// library, so that a case here is a claim about the library and not about a
// document nobody has to read.
func wellFormed(t *testing.T, src string) {
	t.Helper()

	require.True(t, grammar.NewRecognizer(1024).Stream([]byte(src)).OK,
		"the document is not YAML 1.2, so there is nothing to hold the library to")
}

// renderOnce reads a document with comments kept and writes it back.
func renderOnce(t *testing.T, src string) string {
	t.Helper()

	file, err := parser.ParseBytes([]byte(src), parser.WithComments())
	require.NoError(t, err)

	return file.String()
}

// TestDefectAMappingKeyWrittenEmptyIsRefused: `: 2` rather than `k: 2` is
// refused in several positions, and YAML 1.2 accepts every one of them.
//
// Three read and three do not, with two different messages, so this is more
// than one fault behind one shape. Both failing messages name the *earlier*
// line, and in each case that line's value carries properties or is written
// empty.
func TestDefectAMappingKeyWrittenEmptyIsRefused(t *testing.T) {
	t.Run("these read", func(t *testing.T) {
		for _, src := range []string{
			": a\n",
			"a: 1\n: 2\n",
			"- k: 1\n  : 2\n",
			"a: 1\n: &a1 !!null\n",
		} {
			wellFormed(t, src)

			var got any
			assert.NoError(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
		}
	})

	for _, tc := range []struct{ name, src, says string }{
		{
			name: "after an entry with no value",
			src:  "a:\n: 2\n",
			says: "unexpected scalar value",
		},
		{
			name: "after an entry whose value is only an anchor",
			src:  "k: &a1\n: 1\n",
			says: "mapping value is not allowed in this context",
		},
		{
			name: "after an entry whose value carries a tag",
			src:  "false: !!bool false\n: &a1 !!null\n",
			says: "mapping value is not allowed in this context",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wellFormed(t, tc.src)

			var got any
			err := yaml.Unmarshal([]byte(tc.src), &got)
			require.Error(t, err, "today: %q is refused", tc.src)
			assert.Contains(t, err.Error(), tc.says)
		})
	}
}

// TestDefectATaggedBlockMappingDoesNotResolveItsKeys: a tag on a block mapping
// leaves every key as the text that was written.
//
// Narrower than it was, now that a key is named by the canonical spelling of
// its type: `!foo` over `1.0: a` reads "1.0" correctly, because that is the
// text as well as the name. It shows on the booleans and the nulls, where the
// two differ.
func TestDefectATaggedBlockMappingDoesNotResolveItsKeys(t *testing.T) {
	for _, tag := range []string{"!foo", "!", "!<tag:yaml.org,2002:map>"} {
		src := tag + "\nFalse: 1\n"
		wellFormed(t, src)

		var got any
		require.NoError(t, yaml.Unmarshal([]byte(src), &got))
		assert.Equal(t, map[string]any{"False": uint64(1)}, got,
			"today: %q keeps the key's text, and False is named false", src)
	}

	t.Run("the same mapping resolves under !!map, in flow, and untagged", func(t *testing.T) {
		for _, src := range []string{"!!map\nFalse: 1\n", "!foo {False: 1}\n", "False: 1\n"} {
			wellFormed(t, src)

			var got any
			require.NoError(t, yaml.Unmarshal([]byte(src), &got))
			assert.Equal(t, map[string]any{"false": uint64(1)}, got, "%q", src)
		}
	})

	t.Run("and a key whose text is already its name is unaffected", func(t *testing.T) {
		var got any
		require.NoError(t, yaml.Unmarshal([]byte("!foo\n1.0: a\n"), &got))
		assert.Equal(t, map[string]any{"1.0": "a"}, got)
	})
}

// TestDefectAFloatTagOnAWideNumberIsNotRead: `!!float` on a number no float64
// holds fails two ways, where the same number untagged reads correctly.
//
// The wide types are built — untagged, both forms come back as a big.Float, and
// `!!int` on an integer past a machine word comes back as a big.Int. It is the
// float tag alone that does not know about them.
func TestDefectAFloatTagOnAWideNumberIsNotRead(t *testing.T) {
	t.Run("large, the parse stops", func(t *testing.T) {
		const src = "a: !!float 1e+310\n"
		wellFormed(t, src)

		var got any
		err := yaml.Unmarshal([]byte(src), &got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `cannot read "1e+310" as !!float`)
	})

	t.Run("small, it comes back as zero", func(t *testing.T) {
		const src = "a: !!float 1e-400\n"
		wellFormed(t, src)

		var got any
		require.NoError(t, yaml.Unmarshal([]byte(src), &got))
		assert.Equal(t, map[string]any{"a": float64(0)}, got,
			"today: the value is gone and nothing reported it")
	})

	t.Run("untagged, both read as a big.Float", func(t *testing.T) {
		for _, src := range []string{"a: 1e+310\n", "a: 1e-400\n"} {
			var got any
			require.NoError(t, yaml.Unmarshal([]byte(src), &got))
			assert.IsType(t, new(big.Float), got.(map[string]any)["a"], "%q", src)
		}
	})

	t.Run("and an int tag on a wide integer is read", func(t *testing.T) {
		var got any
		require.NoError(t, yaml.Unmarshal([]byte("a: !!int 123456789012345678901\n"), &got))
		assert.IsType(t, new(big.Int), got.(map[string]any)["a"])
	})
}
