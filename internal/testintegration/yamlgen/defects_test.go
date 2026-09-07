// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"fmt"
	"math/big"
	"testing"
	"time"

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

// TestDefectABinaryTagCannotBeReadIntoAGoByteSlice: `!!binary` cannot be read
// into a Go []byte, which is the type the tag names.
//
// The conversion is there -- the same document reads into an `any` as
// []uint8{'h','e','l','l','o'} and into a string field as "hello", the decoded
// bytes rather than the base64 text. Only []byte is refused.
//
// Sibling of TestDefectAnIntTagCannotBeReadIntoAGoInteger and the same shape,
// with one difference: go.yaml.in/yaml/v3 v3.0.5 refuses it too, so this is an
// inconsistency inside the library rather than a departure from the field.
// encoding/json reads a base64 string into a []byte.
func TestDefectABinaryTagCannotBeReadIntoAGoByteSlice(t *testing.T) {
	const src = "a: !!binary aGVsbG8=\n"

	wellFormed(t, src)

	t.Run("today a []byte destination is refused", func(t *testing.T) {
		var into struct {
			A []byte `yaml:"a"`
		}
		err := yaml.Unmarshal([]byte(src), &into)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "string was used where sequence is expected")

		var byName map[string][]byte
		assert.Error(t, yaml.Unmarshal([]byte(src), &byName))

		var items [][]byte
		assert.Error(t, yaml.Unmarshal([]byte("- !!binary aGVsbG8=\n"), &items))
	})

	t.Run("an any and a string field read it", func(t *testing.T) {
		var loose any
		require.NoError(t, yaml.Unmarshal([]byte(src), &loose))
		assert.Equal(t, map[string]any{"a": []byte("hello")}, loose)

		var into struct {
			A string `yaml:"a"`
		}
		require.NoError(t, yaml.Unmarshal([]byte(src), &into))
		assert.Equal(t, "hello", into.A)
	})

	t.Run("the timestamp tag reads into the type it names", func(t *testing.T) {
		var into struct {
			T time.Time `yaml:"t"`
		}
		require.NoError(t, yaml.Unmarshal([]byte("t: !!timestamp 2001-12-14\n"), &into))
		assert.Equal(t, time.Date(2001, time.December, 14, 0, 0, 0, 0, time.UTC), into.T)
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

// TestDefectABlankLineBeforeACommentDoesNotSettle: the first rendering keeps a
// blank line written before a comment and the second drops it.
//
// Only the rendering wobbles -- the value is the same every time and no comment
// is lost. Found on 2026-09-07 by Style.Chomping's padding, which writes the
// blank lines a "-" or a clip indicator then discards.
func TestDefectABlankLineBeforeACommentDoesNotSettle(t *testing.T) {
	const src = "a:\n - x\n\n# c\nb: 1\n"
	wellFormed(t, src)

	once := renderOnce(t, src)
	assert.Equal(t, "a:\n- x\n\n# c\nb: 1\n", once, "the first rendering keeps the blank line")
	assert.Equal(t, "a:\n- x\n# c\nb: 1\n", renderOnce(t, once), "and the second drops it")

	t.Run("three things are needed", func(t *testing.T) {
		for _, tc := range []struct{ name, src string }{
			// Already at the renderer's own indentation.
			{name: "an indentation the renderer does not use", src: "a:\n- x\n\n# c\nb: 1\n"},
			// A mapping rather than a sequence.
			{name: "a nested sequence", src: "a:\n b: 1\n\n# c\nc: 2\n"},
			// Nothing after the comment.
			{name: "an entry after the comment", src: "a:\n - x\n\n# c\n"},
		} {
			once := renderOnce(t, tc.src)
			assert.Equalf(t, once, renderOnce(t, once), "without %s it settles: %q", tc.name, tc.src)
		}
	})

	t.Run("the value survives every rendering", func(t *testing.T) {
		want := map[string]any{"a": []any{"x"}, "b": uint64(1)}

		text := src
		for range 3 {
			var got any
			require.NoError(t, yaml.Unmarshal([]byte(text), &got))
			assert.Equal(t, want, got)
			text = renderOnce(t, text)
		}
	})
}

// TestDefectABlankLineBeforeASequenceEntryDoesNotSettle is the same wobble with
// no comment in it, which is what widens the entry above.
//
// ": &1" over a blank line over "-" over "? \"\"" renders to ": &1" over "- "
// over a blank over "? \"\"" over ":", moving the blank line past the "-", and
// renders again without it. yamlgen.Ledger's predicate for this asks for
// Style.Chomping's padding and a comment, and reaches neither shape here, so
// the property test met it as a plain failure. Found on 2026-09-11 at 30,000
// draws; 200,000 draws of TestRenderReachesAFixedPoint alone did not draw it
// again.
func TestDefectABlankLineBeforeASequenceEntryDoesNotSettle(t *testing.T) {
	const src = ": &1\n\n-\n? \"\"\n"
	wellFormed(t, src)

	once := renderOnce(t, src)
	assert.Equal(t, ": &1\n- \n\n? \"\"\n:\n", once, "the first rendering moves the blank line past the \"-\"")
	assert.Equal(t, ": &1\n- \n? \"\"\n:\n", renderOnce(t, once), "and the second drops it")

	t.Run("the value survives every rendering", func(t *testing.T) {
		want := map[string]any{"": nil, "null": []any{nil}}

		text := src
		for range 3 {
			var got any
			require.NoError(t, yaml.Unmarshal([]byte(text), &got))
			assert.Equal(t, want, got)
			text = renderOnce(t, text)
		}
	})
}

// TestDefectAVersionDirectiveResolvesTheRootBlockScalarItOpens: a "%YAML" line
// over a document whose body is a block scalar makes the parse fail when the
// scalar's content is a word the schema would resolve.
//
// A block scalar is a string under every schema, so there is nothing here to
// resolve. internal/lab's resolvesDifferentlyOnPurpose describes the machinery:
// the grouping reads one token past the directive to know the directive's own
// document has ended, and for a document whose body is a bare scalar that token
// is the body.
func TestDefectAVersionDirectiveResolvesTheRootBlockScalarItOpens(t *testing.T) {
	t.Run("the content decides, and only when it is one whole token", func(t *testing.T) {
		for _, text := range []string{"null", "~", "True", "yes", "5", "1.5"} {
			src := "%YAML 1.1\n---\n>-\n " + text + "\n"
			wellFormed(t, src)

			var got any
			err := yaml.Unmarshal([]byte(src), &got)
			require.Errorf(t, err, "today: %q is refused", src)
			assert.Contains(t, err.Error(), "unexpected token. required string token")
		}

		for _, text := range []string{"x", "x y", "null x"} {
			src := "%YAML 1.1\n---\n>-\n " + text + "\n"

			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equal(t, text, got)
		}
	})

	t.Run("the version does not, and no other directive does it", func(t *testing.T) {
		var refused any
		assert.Error(t, yaml.Unmarshal([]byte("%YAML 1.2\n---\n>-\n null\n"), &refused))

		for _, src := range []string{
			">-\n null\n",
			"---\n>-\n null\n",
			"%TAG !e! tag:yaml.org,2002:\n---\n>-\n null\n",
			// The body has to be the root: as a mapping value it reads.
			"%YAML 1.1\n---\nk: >-\n  null\n",
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.NotNil(t, got, "%q", src)
		}
	})
}

// TestDefectALocalTagBeforeAnAnchorDoesNotTypeItsScalar: a local or
// non-specific tag written before an anchor stops typing its scalar, and the
// scalar resolves by the schema as though it carried no tag.
//
// The order decides, and so does the kind of tag: a `!!` shorthand is
// unaffected. Pins the whole matrix, since a fix that traded one spelling for
// another would otherwise look like a fix.
func TestDefectALocalTagBeforeAnAnchorDoesNotTypeItsScalar(t *testing.T) {
	t.Run("today the anchor cancels the tag", func(t *testing.T) {
		for _, src := range []string{
			"!foo &a1 true\n",
			"! &a1 true\n",
			"!<!foo> &a1 true\n",
			"a: !foo &a1 true\n",
			"- !foo &a1 true\n",
			"{a: !foo &a1 true}\n",
		} {
			wellFormed(t, src)

			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.NotContainsf(t, fmt.Sprintf("%#v", got), `"true"`,
				"today: %q resolves the scalar the tag stands on", src)
		}
	})

	t.Run("the order and the spelling each undo it", func(t *testing.T) {
		for _, src := range []string{
			// The anchor first.
			"&a1 !foo true\n",
			// A secondary tag rather than a local one.
			"!!str &a1 true\n",
			// No anchor at all.
			"!foo true\n",
			"! true\n",
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equalf(t, "true", got, "%q", src)
		}
	})
}

// TestDefectAKeyAfterALongTagOnAnEmptyValueIsNotResolved: an entry whose value
// is a tag written in full with nothing after it stops the key on the next line
// from resolving.
//
// Only the long spellings do it, which is what ties this to
// TestDefectATagNotWrittenAsAShorthandDoesNotTypeItsScalar.
func TestDefectAKeyAfterALongTagOnAnEmptyValueIsNotResolved(t *testing.T) {
	t.Run("today the key keeps its text", func(t *testing.T) {
		for _, src := range []string{
			"a: !<tag:yaml.org,2002:null>\nFalse: 1\n",
			"%TAG !e! tag:yaml.org,2002:\n---\na: !e!null\nFalse: 1\n",
		} {
			wellFormed(t, src)

			var got map[string]any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Containsf(t, got, "False", "today: %q leaves the key unresolved", src)
		}
	})

	t.Run("the shorthand resolves it", func(t *testing.T) {
		var got map[string]any
		require.NoError(t, yaml.Unmarshal([]byte("a: !!null\nFalse: 1\n"), &got))
		assert.Contains(t, got, "false")
	})
}
