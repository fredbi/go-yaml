// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"math"
	"math/big"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	yaml "github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/codec"
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

// TestDefectATagNotWrittenAsAShorthandDoesNotTypeItsScalar: the scanner types
// the scalar under a `!!` tag and leaves the one under the same tag written any
// other way as a string.
//
// The tag's URI is identical in every spelling, so what changes is the tree:
// `!!float 7` puts an *ast.IntegerNode under the *ast.TagNode and
// `!<tag:yaml.org,2002:float> 7` an *ast.StringNode. The string is read back
// against the tag afterwards, which is why almost everything still decodes
// correctly and the three specials do not.
func TestDefectATagNotWrittenAsAShorthandDoesNotTypeItsScalar(t *testing.T) {
	t.Run("the tree depends on how the tag was spelled", func(t *testing.T) {
		for _, tc := range []struct {
			src  string
			node ast.Node
		}{
			{src: "!!float 7\n", node: &ast.IntegerNode{}},
			{src: "!!float 1e3\n", node: &ast.FloatNode{}},
			{src: "!!float .inf\n", node: &ast.InfinityNode{}},
			{src: "!<tag:yaml.org,2002:float> 7\n", node: &ast.StringNode{}},
			{src: "!<tag:yaml.org,2002:float> 1e3\n", node: &ast.StringNode{}},
			{src: "!<tag:yaml.org,2002:float> .inf\n", node: &ast.StringNode{}},
			{src: "%TAG !e! tag:yaml.org,2002:\n---\n!e!float 1e3\n", node: &ast.StringNode{}},
		} {
			wellFormed(t, tc.src)

			file, err := parser.ParseBytes([]byte(tc.src), parser.WithComments())
			require.NoError(t, err, "%q", tc.src)

			tag, ok := file.Docs[len(file.Docs)-1].Body.(*ast.TagNode)
			require.True(t, ok, "%q: the body is not a tag node", tc.src)
			assert.Equal(t, "tag:yaml.org,2002:float", tag.URI, "%q", tc.src)
			assert.IsType(t, tc.node, tag.Value, "%q", tc.src)
		}
	})

	t.Run("and the three specials lose their value", func(t *testing.T) {
		for _, text := range []string{".inf", "-.inf", ".nan"} {
			long := "!<tag:yaml.org,2002:float> " + text + "\n"
			wellFormed(t, long)

			var got any
			require.NoError(t, yaml.Unmarshal([]byte(long), &got))
			assert.Equal(t, float64(0), got, "today: %q reads zero and reports nothing", long)

			out, err := codec.ToJSON([]byte(long))
			require.NoError(t, err)
			assert.Equal(t, "0.0", string(out), "today: %q converts to zero", long)
		}
	})

	t.Run("where the shorthand reads them, and ToJSON refuses them", func(t *testing.T) {
		var got any
		require.NoError(t, yaml.Unmarshal([]byte("!!float .inf\n"), &got))
		assert.Equal(t, math.Inf(1), got)

		_, err := codec.ToJSON([]byte("!!float .inf\n"))
		require.Error(t, err, "JSON has no infinity, so refusing is right")
		assert.Contains(t, err.Error(), "JSON has no number for .inf")
	})

	t.Run("every other value survives every spelling", func(t *testing.T) {
		for _, tc := range []struct {
			short, long string
			want        any
		}{
			{short: "!!int 0x1f\n", long: "!<tag:yaml.org,2002:int> 0x1f\n", want: 31},
			{short: "!!float 1e3\n", long: "!<tag:yaml.org,2002:float> 1e3\n", want: float64(1000)},
			{short: "!!bool True\n", long: "!<tag:yaml.org,2002:bool> True\n", want: true},
			{short: "!!str null\n", long: "!<tag:yaml.org,2002:str> null\n", want: "null"},
			{short: "!!null ~\n", long: "!<tag:yaml.org,2002:null> ~\n", want: nil},
		} {
			for _, src := range []string{tc.short, tc.long} {
				var got any
				require.NoError(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
				assert.Equal(t, tc.want, got, "%q", src)
			}
		}
	})

	t.Run("a collection tag is unaffected, since the scalar under it carries none", func(t *testing.T) {
		var got any
		require.NoError(t, yaml.Unmarshal([]byte("!<tag:yaml.org,2002:seq> [1, .inf]\n"), &got))
		assert.Equal(t, []any{uint64(1), math.Inf(1)}, got)
	})
}

// TestDefectAnIntTagCannotBeReadIntoAGoInteger: `!!int` on a value cannot be
// read into any Go integer, where the same value untagged reads into all of
// them.
//
// [yamlgen.Tagged.Decoded] says where to look: an untagged non-negative integer
// comes back as a uint64 and a negative one as an int64, and `!!int` overrides
// both with a plain int. The reflection path has a case for the first two.
func TestDefectAnIntTagCannotBeReadIntoAGoInteger(t *testing.T) {
	type box struct {
		N int64   `yaml:"n"`
		I int     `yaml:"i"`
		U uint64  `yaml:"u"`
		S string  `yaml:"s"`
		B bool    `yaml:"b"`
		F float64 `yaml:"f"`
		A any     `yaml:"a"`
	}

	t.Run("into an integer field, whatever its width", func(t *testing.T) {
		for _, tc := range []struct{ src, says string }{
			{src: "n: !!int 5\n", says: "of type int64"},
			{src: "i: !!int 5\n", says: "of type int"},
			{src: "u: !!int 5\n", says: "of type uint64"},
			{src: "n: !<tag:yaml.org,2002:int> -5\n", says: "of type int64"},
		} {
			wellFormed(t, tc.src)

			var got box
			err := yaml.Unmarshal([]byte(tc.src), &got)
			require.Error(t, err, "today: %q is refused", tc.src)
			assert.Contains(t, err.Error(), "cannot unmarshal int into Go struct field")
			assert.Contains(t, err.Error(), tc.says)
		}
	})

	t.Run("into a slice and into a typed map", func(t *testing.T) {
		var items []int64
		assert.Error(t, yaml.Unmarshal([]byte("- !!int 5\n"), &items))

		var byName map[string]int64
		assert.Error(t, yaml.Unmarshal([]byte("n: !!int 5\n"), &byName))
	})

	t.Run("untagged the same documents read", func(t *testing.T) {
		var got box
		require.NoError(t, yaml.Unmarshal([]byte("n: 5\ni: 5\nu: 5\n"), &got))
		assert.Equal(t, box{N: 5, I: 5, U: 5}, got)

		var items []int64
		require.NoError(t, yaml.Unmarshal([]byte("- 5\n"), &items))
		assert.Equal(t, []int64{5}, items)
	})

	t.Run("and the other tags read, as does !!int into an any", func(t *testing.T) {
		var got box
		require.NoError(t, yaml.Unmarshal([]byte("s: !!str x\nb: !!bool true\nf: !!float 1.5\na: !!int 5\n"), &got))
		assert.Equal(t, box{S: "x", B: true, F: 1.5, A: 5}, got)
	})
}

// TestOneNonStringKeyNoLongerZeroesAWholeStruct: a key no field can be named
// after is skipped, and the entries around it read.
//
// ✅ Closed 2026-09-07 in two steps. 7dc4075 inverted decodeStruct, so the
// decode walks the document's entries and looks each field up rather than
// walking the fields and reading the mapping into a map first -- the map came
// back nil at the first key that was not a string, with no error, and every
// field kept its zero. entryName then named a key by its type's own canonical
// spelling, so a key a field can be named after reaches it: "true: a" writes a
// field tagged "true".
func TestOneNonStringKeyNoLongerZeroesAWholeStruct(t *testing.T) {
	type named struct {
		Name string `yaml:"name"`
		True string `yaml:"true"`
	}

	for _, src := range []string{
		"1: a\nname: x\n",
		"name: x\n1: a\n",
		"1: a\nname: x\n2: b\n",
	} {
		wellFormed(t, src)

		var got named
		require.NoError(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
		assert.Equal(t, "x", got.Name, "%q: the entries around the key that cannot be named must read", src)
	}

	t.Run("a key a field can be named after reaches it", func(t *testing.T) {
		var got named
		require.NoError(t, yaml.Unmarshal([]byte("true: a\nname: x\n"), &got))
		assert.Equal(t, named{Name: "x", True: "a"}, got)
	})

	t.Run("the same documents read into an any", func(t *testing.T) {
		var got any
		require.NoError(t, yaml.Unmarshal([]byte("1: a\nname: x\n"), &got))
		assert.Equal(t, map[string]any{"1": "a", "name": "x"}, got)
	})

	t.Run("and a string-keyed document reads into the struct", func(t *testing.T) {
		var got named
		require.NoError(t, yaml.Unmarshal([]byte("name: x\n"), &got))
		assert.Equal(t, named{Name: "x"}, got)
	})
}

// The three defects Style.ExplicitKeys found on its first deep run.
//
// A mapping entry has two spellings -- "key: value" and "? key" over
// ": value" -- and the generator had only ever written the short one. All three
// of these are the long form and nothing else: the same document written short
// reads, libfyaml 1.0.0b1 reads every one, the reference parser passes them,
// and grammar.NewRecognizer accepts them.

// TestDefectAQuotedExplicitKeyRefusesABlockScalarValue: `? "a"` over `: >-` is
// refused where `? a` over the same two lines reads.
func TestDefectAQuotedExplicitKeyRefusesABlockScalarValue(t *testing.T) {
	for _, src := range []string{
		"? \"a\"\n: >-\n  x\n",
		"? 'a'\n: >-\n  x\n",
		"? \"a\"\n: |\n  x\n",
		"a:\n  ? \"b\"\n  : >-\n    x\n",
		"- ? \"a\"\n  : >-\n    x\n",
		"? \"a\"\n: &an >-\n  x\n",
		"? \"a\"\n: !!str >-\n  x\n",
	} {
		wellFormed(t, src)

		var got any
		err := yaml.Unmarshal([]byte(src), &got)
		require.Errorf(t, err, "today: %q is refused", src)
		assert.Contains(t, err.Error(), "value is not allowed in this context", "%q", src)
	}

	t.Run("a plain key over the same value reads", func(t *testing.T) {
		for _, src := range []string{"? a\n: >-\n  x\n", "? a\n: |\n  x\n", "? a\n: |3-\n   x\n"} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
		}
	})

	t.Run("and so does the short form of the quoted one", func(t *testing.T) {
		var got any
		require.NoError(t, yaml.Unmarshal([]byte("\"a\": >-\n  x\n"), &got))
		assert.Equal(t, map[string]any{"a": "x"}, got)
	})

	t.Run("a value that is not a block scalar reads under the quoted key", func(t *testing.T) {
		for _, src := range []string{"? \"a\"\n: \"y\"\n", "? \"a\"\n: [1]\n", "? \"\"\n: x\n"} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
		}
	})
}

// TestDefectAnAnchorAloneAfterAnExplicitKeySwallowsWhatFollows: `: &a1` with
// nothing after it takes the entries below into the node it anchors.
//
// The same shape as a tag on an empty value, one property over -- see
// yamlcorpus.Departures, "a local tag on an empty value, with the mapping
// carrying on".
func TestDefectAnAnchorAloneAfterAnExplicitKeySwallowsWhatFollows(t *testing.T) {
	const src = "? a\n: &a1\n? b\n: &a2\n"
	wellFormed(t, src)

	var got any
	require.NoError(t, yaml.Unmarshal([]byte(src), &got))
	assert.Equal(t, map[string]any{"a": map[string]any{"b": nil}}, got,
		"today: b is swallowed into the node a's anchor names")

	t.Run("the renderer writes the nesting back out", func(t *testing.T) {
		assert.Equal(t, "? a\n: &a1\n  ? b\n  : &a2\n", renderOnce(t, src))
	})

	t.Run("an alias to the anchor stops the parse instead", func(t *testing.T) {
		const aliased = "? a\n: &a1\n? b\n: *a1\n"
		wellFormed(t, aliased)

		var v any
		err := yaml.Unmarshal([]byte(aliased), &v)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `alias "a1" names an anchor that is not resolved yet`)
	})

	t.Run("the short form, the long form without anchors, and a value all read", func(t *testing.T) {
		for _, tc := range []struct {
			src  string
			want map[string]any
		}{
			{src: "a: &a1\nb: &a2\n", want: map[string]any{"a": nil, "b": nil}},
			{src: "? a\n:\n? b\n:\n", want: map[string]any{"a": nil, "b": nil}},
			{src: "? a\n: &a1 x\n? b\n: *a1\n", want: map[string]any{"a": "x", "b": "x"}},
		} {
			var v any
			require.NoErrorf(t, yaml.Unmarshal([]byte(tc.src), &v), "%q", tc.src)
			assert.Equal(t, tc.want, v, "%q", tc.src)
		}
	})
}

// TestDefectACommentOnAnExplicitKeysColonLineIsDropped: a comment on the ":"
// line of the long form, with the value below it, is lost by the renderer.
func TestDefectACommentOnAnExplicitKeysColonLineIsDropped(t *testing.T) {
	for _, tc := range []struct{ src, renders string }{
		{src: "? a\n: # c3\n  v\n", renders: "? a\n: v\n"},
		{src: "? a\n: # c3\n  - 1\n", renders: "? a\n:\n- 1\n"},
		{src: "?\n: #c1\n", renders: "?\n:\n"},
	} {
		wellFormed(t, tc.src)
		assert.Equal(t, tc.renders, renderOnce(t, tc.src), "today: %q loses its comment", tc.src)
	}

	t.Run("every other position keeps it", func(t *testing.T) {
		for _, tc := range []struct{ src, renders string }{
			{src: "a: # c3\n  v\n", renders: "a: v # c3\n"},
			{src: "a: # c3\n  - 1\n", renders: "a: # c3\n- 1\n"},
			{src: "? a\n: v # c3\n", renders: "? a\n: v # c3\n"},
			{src: "? a # c3\n: v\n", renders: "? a # c3\n: v\n"},
		} {
			assert.Equal(t, tc.renders, renderOnce(t, tc.src), "%q", tc.src)
		}
	})
}
