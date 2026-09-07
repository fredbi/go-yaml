// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	yaml "github.com/go-openapi/go-yaml"
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

// The three defects Style.ExplicitKeys found on its first deep run.
//
// A mapping entry has two spellings -- "key: value" and "? key" over
// ": value" -- and the generator had only ever written the short one. All three
// of these are the long form and nothing else: the same document written short
// reads, libfyaml 1.0.0b1 reads every one, the reference parser passes them,
// and grammar.NewRecognizer accepts them.

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

// TestDefectABlockScalarInASequenceSwallowsAnEmptyKey: a block scalar written
// as a nested sequence entry, with an empty key after it, renders to text the
// parser refuses.
//
// "a:" over " - |1-" over "   " over ":" renders to "a:" over "- |2-    :",
// which reports `invalid header option: "2-    :"` on the way back in. The
// content and the empty key's ":" are both written onto the header line, so a
// valid document renders to one that is not YAML at all.
//
// The empty key is what does it: "b: 1" in its place renders correctly, at the
// same column. Nothing else is needed -- the anchor and the whitespace-only
// content each drop out and it still happens, and "|1" chomping the break goes
// the same way as "|1-".
//
// Found on 2026-09-12 at 30,000 draws of TestRenderWritesValidYAML, on the
// document "&a3 !!map" over `"": &a2` over " - !!null" over " - &a1 |1-" over
// "   " over ":".
func TestDefectABlockScalarInASequenceSwallowsAnEmptyKey(t *testing.T) {
	for src, renders := range map[string]string{
		"a:\n - &a1 |1-\n   \n:\n":  "a:\n- &a1 |2-    :\n",
		"a:\n - |1-\n   \n:\n":      "a:\n- |2-    :\n",
		"a:\n - &a1 |1-\n   x\n:\n": "a:\n- &a1 |2-    x:\n",
		"a:\n - &a1 |1\n   \n:\n":   "a:\n- &a1 |2    :\n",
	} {
		wellFormed(t, src)

		once := renderOnce(t, src)
		assert.Equal(t, renders, once, "today: %q writes its content onto the header line", src)

		_, err := parser.ParseBytes([]byte(once), parser.WithComments())
		require.Errorf(t, err, "today: %q renders to text the parser refuses", src)
		assert.Contains(t, err.Error(), "invalid header option")
	}

	t.Run("an ordinary key after it renders correctly", func(t *testing.T) {
		const src = "a:\n - &a1 |1-\n   \nb: 1\n"
		wellFormed(t, src)

		once := renderOnce(t, src)
		assert.Equal(t, "a:\n- &a1 |2-\n   \nb: 1\n", once)

		_, err := parser.ParseBytes([]byte(once), parser.WithComments())
		require.NoError(t, err)
	})
}

// TestDefectASecondCommentOnAnExplicitKeysColonLineIsDropped: a comment on the
// ":" line of the long form and a head comment under it, and only the first
// survives.
//
// What is left of the entry closed on 2026-09-12. The ":" line comment goes on
// the value now, and a head comment written under it has nowhere left to go:
// "? a" over ": # c4" over "  # c5" over "  - 1" keeps c4 and loses c5. A
// nested mapping keeps both, which is the shape that says the head comment can
// be carried at all.
func TestDefectASecondCommentOnAnExplicitKeysColonLineIsDropped(t *testing.T) {
	for src, renders := range map[string]string{
		"? a\n: # c4\n  # c5\n  - 1\n": "? a\n:\n# c4\n- 1\n",
		"? a\n: # c4\n  # c5\n  v\n":   "? a\n: v # c4\n",
	} {
		wellFormed(t, src)
		assert.Equal(t, renders, renderOnce(t, src), "today: %q loses the second comment", src)
	}

	t.Run("a nested mapping keeps both", func(t *testing.T) {
		const src = "? a\n: # c4\n  # c5\n  b: 1\n"
		wellFormed(t, src)
		assert.Equal(t, "? a\n:\n  # c4\n  # c5\n  b: 1\n", renderOnce(t, src))
	})

	t.Run("one comment on the ':' line is kept", func(t *testing.T) {
		assert.Equal(t, "? a\n: v # c3\n", renderOnce(t, "? a\n: # c3\n  v\n"))
	})
}

// TestDefectADocumentSuffixMishandlesAPropertiedBlockScalar: a "..." suffix
// followed by a bare document whose root is a block scalar carrying an anchor
// or a tag reads differently from the same stream spelled with "---".
//
// Two symptoms, one suffix. With an indentation indicator a column of the
// content is dropped; without one a valid stream is refused outright.
//
// The reference parser settles which side is right and it is "---": it emits
// the same events for both spellings. Do not reach for libfyaml or
// go.yaml.in/yaml/v3 on the first symptom -- both strip a column from every
// root block scalar with an indicator, so they agree with each other and with
// neither the grammar nor the specification. It is a content question, and the
// reference parser is the source that answers one.
func TestDefectADocumentSuffixMishandlesAPropertiedBlockScalar(t *testing.T) {
	t.Run("today a column goes missing", func(t *testing.T) {
		for _, tc := range []struct{ suffix, marker, reads, correct string }{
			{
				suffix:  "a: 1\n...\n&a1 |2-\n  \n",
				marker:  "a: 1\n---\n&a1 |2-\n  \n",
				reads:   "",
				correct: " ",
			},
			{
				suffix:  "a: 1\n...\n!!str |2-\n  x\n",
				marker:  "a: 1\n---\n!!str |2-\n  x\n",
				reads:   "x",
				correct: " x",
			},
		} {
			wellFormed(t, tc.suffix)

			assert.Equalf(t, tc.reads, secondDocument(t, tc.suffix), "today: %q", tc.suffix)
			assert.Equalf(t, tc.correct, secondDocument(t, tc.marker), "the marker spelling: %q", tc.marker)
		}
	})

	t.Run("today a valid stream is refused", func(t *testing.T) {
		const refused = "&a3 a: 1\n...\n&a1 >-\n -\n"

		wellFormed(t, refused)

		_, err := parser.ParseBytes([]byte(refused), parser.WithComments())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "value is not allowed in this context")

		// The marker spelling of the same stream reads.
		assert.Equal(t, "-", secondDocument(t, "&a3 a: 1\n---\n&a1 >-\n -\n"))
	})

	t.Run("it takes an anchor on each side, and a block scalar", func(t *testing.T) {
		for _, src := range []string{
			// No anchor on the first document.
			"a: 1\n...\n&a1 >-\n -\n",
			// None on the second.
			"&a3 a: 1\n...\n>-\n -\n",
			// A plain scalar rather than a block one.
			"&a3 a: 1\n...\n&a1 x\n",
		} {
			_, err := parser.ParseBytes([]byte(src), parser.WithComments())
			assert.NoErrorf(t, err, "%q", src)
		}
	})
}

// secondDocument reads a two-document stream and returns what the second one
// holds, which is where this defect shows.
func secondDocument(t *testing.T, src string) string {
	t.Helper()

	dec := codec.NewDecoder(bytes.NewReader([]byte(src)))

	var last any

	for i := 0; ; i++ {
		var v any

		err := dec.Decode(&v)
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoErrorf(t, err, "document %d of %q", i, src)

		last = v
	}

	text, isText := last.(string)
	require.Truef(t, isText, "the last document of %q is %#v", src, last)

	return text
}

// TestDefectAVersionDirectiveSpansTheWholeStream: a "%YAML" directive is
// applied to every document of the stream rather than to the one it precedes.
//
// The library disagrees with itself: an anchor and a %TAG handle are both
// scoped to the document that declares them, and enforced, while a version
// directive is not. Documents are independent -- Fred, 2026-09-13 -- so the
// second document here should be read under the core schema, which reads "yes"
// as the string.
func TestDefectAVersionDirectiveSpansTheWholeStream(t *testing.T) {
	t.Run("today the directive reaches the second document", func(t *testing.T) {
		const src = "%YAML 1.1\n---\na: yes\n---\nb: yes\n"

		wellFormed(t, src)

		got := readTheStream(t, src)
		require.Len(t, got, 2)
		assert.Equal(t, map[string]any{"a": true}, got[0], "the first document declares 1.1")
		assert.Equal(t, map[string]any{"b": true}, got[1],
			"today: the second declares nothing and is read under 1.1 anyway")
	})

	t.Run("what each document should mean on its own", func(t *testing.T) {
		// The same two documents, written apart.
		assert.Equal(t, []any{map[string]any{"a": true}}, readTheStream(t, "%YAML 1.1\n---\na: yes\n"))
		assert.Equal(t, []any{map[string]any{"b": "yes"}}, readTheStream(t, "b: yes\n"))
	})

	t.Run("an anchor and a tag handle are scoped, which is the inconsistency", func(t *testing.T) {
		var v any
		err := yaml.Unmarshal([]byte("a: &x 1\n---\nb: *x\n"), &v)
		require.Error(t, err, "an anchor does not reach the next document")
		assert.Contains(t, err.Error(), `could not find alias "x"`)

		_, herr := parser.ParseBytes(
			[]byte("%TAG !e! tag:yaml.org,2002:\n---\na: !e!str 1\n---\nb: !e!str 2\n"),
			parser.WithComments())
		require.Error(t, herr, "a handle does not reach the next document")
		assert.Contains(t, herr.Error(), "tag handle !e! is not defined")
	})

	t.Run("each document declaring its own reads under it", func(t *testing.T) {
		const both = "%YAML 1.1\n---\na: yes\n...\n%YAML 1.1\n---\nb: yes\n"

		wellFormed(t, both)
		assert.Equal(t, []any{map[string]any{"a": true}, map[string]any{"b": true}}, readTheStream(t, both))
	})
}

// readTheStream reads every document of src, which is what a caller looping
// over codec.Decoder gets.
func readTheStream(t *testing.T, src string) []any {
	t.Helper()

	dec := codec.NewDecoder(bytes.NewReader([]byte(src)))

	var out []any

	for {
		var v any

		err := dec.Decode(&v)
		if errors.Is(err, io.EOF) {
			return out
		}
		require.NoErrorf(t, err, "%q", src)

		out = append(out, v)
	}
}
