// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
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

// TestDefectAMergeKeyWrittenTheLongWayDoesNotMerge pins the two spellings apart.
//
// The 1.1 merge type names the key `<<` and says nothing about how it is
// written, so `? <<` over `: *a` is the same key node as `<<: *a` and should
// merge the same way. It does not: the long form comes back as an ordinary key
// named "<<".
//
// go.yaml.in/yaml/v3 v3.0.5 merges both, in block and in flow, and is the oracle
// that answers here -- libfyaml 1.0.0b1 resolves no merge at all and hands "<<"
// back as a member name, so it cannot say which spelling is right.
func TestDefectAMergeKeyWrittenTheLongWayDoesNotMerge(t *testing.T) {
	const base = "b: &a {x: 1}\n"

	t.Run("the plain spelling merges", func(t *testing.T) {
		for _, src := range []string{
			base + "d:\n  <<: *a\n  y: 2\n",
			// A tag on the mapping changes nothing, which is what makes this
			// the key's presentation rather than the node's type.
			base + "d: !foo\n  <<: *a\n  y: 2\n",
			base + "d: !!map\n  <<: *a\n  y: 2\n",
		} {
			var got map[string]any
			require.NoErrorf(t, codec.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equalf(t, map[string]any{"x": uint64(1), "y": uint64(2)}, got["d"], "%q", src)
		}
	})

	t.Run("today the long spelling does not", func(t *testing.T) {
		for _, src := range []string{
			base + "d:\n  ? <<\n  : *a\n  y: 2\n",
			base + "d: {? <<\n  : *a, y: 2}\n",
			base + "d: !foo\n  ? <<\n  : *a\n  y: 2\n",
		} {
			var got map[string]any
			require.NoErrorf(t, codec.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equalf(t,
				map[string]any{"<<": map[string]any{"x": uint64(1)}, "y": uint64(2)},
				got["d"], "today: the long spelling is an ordinary key: %q", src)
		}
	})

	// The worse half: the answer depends on the destination. Written in flow
	// with the mapping in place, the walk hands "<<" back and the tree merges
	// it, so two callers reading the same document into different destinations
	// get different values.
	t.Run("today the two decode paths disagree in flow", func(t *testing.T) {
		const src = "{? <<: {x: 1}, y: 2}\n"

		var walked any
		require.NoError(t, codec.Unmarshal([]byte(src), &walked))
		assert.Equal(t,
			map[string]any{"<<": map[string]any{"x": uint64(1)}, "y": uint64(2)},
			walked, "today: the walk does not merge it")

		var typed map[string]any
		require.NoError(t, codec.Unmarshal([]byte(src), &typed))
		assert.Equal(t,
			map[string]any{"x": uint64(1), "y": uint64(2)},
			typed, "today: the tree does merge it")
	})

	t.Run("in block the two paths agree, and neither merges", func(t *testing.T) {
		const src = "d:\n  ? <<\n  : {x: 1}\n  y: 2\n"

		want := map[string]any{"d": map[string]any{"<<": map[string]any{"x": uint64(1)}, "y": uint64(2)}}

		var walked any
		require.NoError(t, codec.Unmarshal([]byte(src), &walked))
		assert.Equal(t, want, walked)

		var typed map[string]any
		require.NoError(t, codec.Unmarshal([]byte(src), &typed))
		assert.Equal(t, want, typed)
	})
}

// TestDefectAMergeKeyAloneInFlowEscapesTheDuplicateCheck pins the inconsistency.
//
// 3.2.1.1 makes two keys that resolve alike one key, and a flow entry written
// as a key alone is an entry like any other: `{a: 1, a}` is refused, and so is
// `{<<: {x: 1}, <<: {y: 2}}`. `{<<: {x: 1}, <<}` is read.
//
// What comes back is stranger than the acceptance: the first "<<" merges and
// the second becomes a literal key, so the mapping holds both the merged entry
// and a "<<" named nothing.
//
// Found on 2026-09-07 when yamlcorpus's duplicateAKey landed on a merge key --
// the merge axis made that reachable for the first time.
func TestDefectAMergeKeyAloneInFlowEscapesTheDuplicateCheck(t *testing.T) {
	t.Run("an ordinary key alone is refused, and so are two merge keys with values", func(t *testing.T) {
		for _, src := range []string{
			"{a: 1, a}\n",
			"{<<: {x: 1}, <<: {y: 2}}\n",
			"b: &r {x: 1}\nd:\n  <<: *r\n  <<: *r\n",
		} {
			var got any
			assert.Errorf(t, codec.Unmarshal([]byte(src), &got), "%q", src)
		}
	})

	t.Run("today a merge key alone is read", func(t *testing.T) {
		const src = "{<<: {x: 1}, <<}\n"

		var got any
		require.NoError(t, codec.Unmarshal([]byte(src), &got))
		assert.Equal(t, map[string]any{"<<": nil, "x": uint64(1)}, got,
			"today: the first merges and the second is a literal key")
	})
}

// TestDefectMergingNullIsReadByTheWalkAndRefusedByTheTree pins the split.
//
// `<<:` with no value asks to merge null, which is not a mapping and so not a
// merge at all. The two decode paths answer differently: the walk drops the
// entry and hands back an empty mapping, the tree refuses the document with
// "null was used where mapping is expected".
//
// Whichever answer is right, one document should not have two. yamlcorpus's
// MergeShapes holds "a merge key with no alias at all" under TagMergeNonMapping
// for the stance question of what merging a non-mapping means; this is the
// narrower fault of the two paths disagreeing about it.
func TestDefectMergingNullIsReadByTheWalkAndRefusedByTheTree(t *testing.T) {
	// A second shape, and a narrower one: the two paths read the element
	// differently rather than the merge. "<<: [{a: 1}, - {b: 2}]" merges both
	// mappings on the walk and is refused by the tree as "sequence was used
	// where mapping is expected", so the tree sees a sequence where the walk
	// sees the mapping. Every other non-mapping merge -- "<<: x", "<<: 1",
	// "<<: [x]", "<<: [[x]]", "<<: [1]" -- is refused by both.
	//
	// Reached on 2026-09-07 by a mutation that put a "-" inside a merge
	// sequence, once the corpus grew to 3,000 drawn documents.
	t.Run("a '-' inside a flow merge sequence", func(t *testing.T) {
		const src = "<<: [{a: 1}, - {b: 2}]\n"

		var walked any
		require.NoError(t, codec.Unmarshal([]byte(src), &walked))
		assert.Equal(t, map[string]any{"a": uint64(1), "b": uint64(2)}, walked,
			"today: the walk merges both")

		var typed map[string]any
		err := codec.Unmarshal([]byte(src), &typed)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "sequence was used where mapping is expected")
	})

	for _, src := range []string{"<<:\n", "<<: null\n", "a:\n  <<:\n"} {
		var walked any
		assert.NoErrorf(t, codec.Unmarshal([]byte(src), &walked), "today: the walk reads it: %q", src)

		var typed map[string]any
		err := codec.Unmarshal([]byte(src), &typed)
		require.Errorf(t, err, "today: the tree refuses it: %q", src)
		assert.Contains(t, err.Error(), "null was used where mapping is expected")
	}
}

// TestDefectAnExplicitKeyInsideAnExplicitKeyIsRefused pins the nesting.
//
// 8.2.2 puts an explicit entry's key at s-l+block-indented(n, block-out), which
// is any block node -- a mapping written the long way included. Nesting the two
// "?" is refused.
//
// The same key written any other way reads, which is what makes this the nesting
// and not the collection key.
//
// Reached on 2026-09-07, when Keys began drawing a collection. No generated
// document held a collection key before that, and the census reports the YAML
// Test Suite holds no nested explicit key either, so nothing on either side had
// provoked it.
func TestDefectAnExplicitKeyInsideAnExplicitKeyIsRefused(t *testing.T) {
	t.Run("today the nesting is refused", func(t *testing.T) {
		var got any
		err := codec.Unmarshal([]byte("?\n  ? a\n  : 0\n: v\n"), &got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected scalar value type")
	})

	t.Run("the same key written any other way reads", func(t *testing.T) {
		for _, tc := range []struct{ src, key string }{
			{"?\n  a: 0\n: v\n", "map[a:0]"},
			{"? {a: 0}\n: v\n", "map[a:0]"},
			{"?\n  - a\n  - b\n: v\n", "[a b]"},
		} {
			// Into an `any`, which names the key by walking it. A typed map
			// cannot hold one: `cannot use map[string]interface {} as a map
			// key: Go cannot hash it`.
			var got any
			require.NoErrorf(t, codec.Unmarshal([]byte(tc.src), &got), "%q", tc.src)
			assert.Equalf(t, map[string]any{tc.key: "v"}, got, "%q", tc.src)
		}
	})
}

// TestDefectAKeyBelowItsIndicatorLosesItsIndentation pins the render.
//
// A collection key cannot go on its "?"s own line -- 8.2.2 puts it at
// s-l+block-indented(n, block-out) and a block collection has nowhere to be
// indented against there -- so it is written below. Rendering brings it back at
// column 1, where it is no longer the key.
//
// The blank line is the trigger, and a head comment is what puts one there.
// Without it the renderer moves the key up onto the "?"s line, which is a
// different spelling of the same document and settles.
//
// Reached on 2026-09-07, when Keys began drawing a collection.
func TestDefectAKeyBelowItsIndicatorLosesItsIndentation(t *testing.T) {
	t.Run("today a blank line above the key loses its indentation", func(t *testing.T) {
		const src = "?\n\n \"\": 0\n: v\n"

		f, err := parser.ParseBytes([]byte(src), parser.WithComments())
		require.NoError(t, err)
		assert.Equal(t, "? \n\"\": 0\n: v\n", f.String(),
			"today: the key comes back at column 1")
	})

	t.Run("without the blank line the render settles", func(t *testing.T) {
		for _, src := range []string{"?\n a: 0\n: v\n", "?\n - a\n: v\n"} {
			f, err := parser.ParseBytes([]byte(src), parser.WithComments())
			require.NoErrorf(t, err, "%q", src)

			once := f.String()
			g, err := parser.ParseBytes([]byte(once), parser.WithComments())
			require.NoErrorf(t, err, "%q", once)
			assert.Equalf(t, once, g.String(), "%q renders to %q and then moves", src, once)
		}
	})
}

// TestDefectACollectionKeyWrittenAloneInFlowIsRefused pins it.
//
// 7.4.2 lets a flow mapping entry be a key with no value, and lets that key be
// any flow node. `{{"": 0}}` is one entry whose key is the mapping {"": 0}.
//
// The same key with a value parses, so it is the missing value and not the
// collection key. The whole measurement, including why libfyaml cannot answer,
// is on yamlgen.Strict's entry of the same name.
func TestDefectACollectionKeyWrittenAloneInFlowIsRefused(t *testing.T) {
	t.Run("today a collection key alone is refused", func(t *testing.T) {
		for _, src := range []string{"{{\"\": 0}}\n", "{[a]}\n"} {
			var got any
			err := codec.Unmarshal([]byte(src), &got)
			require.Errorf(t, err, "%q", src)
			assert.Containsf(t, err.Error(), "could not find flow map content", "%q", src)
		}
	})

	t.Run("the same key with a value parses", func(t *testing.T) {
		var got any
		require.NoError(t, codec.Unmarshal([]byte("{{a: 0}: v}\n"), &got))
		assert.Equal(t, map[string]any{"map[a:0]": "v"}, got)
	})
}

// TestDefectTwoBareColonLinesInARowAreRefused pins it.
//
// 8.2.2 lets an explicit entry's value be the empty node and lets an entry's key
// be the empty node, so `? a` over `:` over `: v` is two entries and two bare
// ":" lines. It is refused with `found an invalid key for this map`.
//
// grammar.NewRecognizer accepts it, the reference parser passes it, and libfyaml
// 1.0.0b1 reads {a: null, null: "v"}. go.yaml.in/yaml/v3 v3.0.5 refuses it,
// which is its own refusal of a bare ":" as an empty key rather than agreement.
//
// Reached on 2026-09-08 by Keys drawing a collection: a collection key forces
// the explicit form, which put a bare ":" where nothing had put one before. The
// collection is not needed to reproduce it, which the neighbors below show.
func TestDefectTwoBareColonLinesInARowAreRefused(t *testing.T) {
	t.Run("today the pair of bare colon lines is refused", func(t *testing.T) {
		for _, src := range []string{"? a\n:\n: v\n", "?\n \"\": 0\n:\n: v\n"} {
			var got any
			err := codec.Unmarshal([]byte(src), &got)
			require.Errorf(t, err, "%q", src)
			assert.Containsf(t, err.Error(), "found an invalid key for this map", "%q", src)
		}
	})

	// Change any one thing and it reads, which is what makes the pair of lines
	// the shape rather than the explicit key, the empty value or the empty key.
	t.Run("each neighbor reads", func(t *testing.T) {
		for _, tc := range []struct {
			src  string
			want map[string]any
		}{
			{"a:\n: v\n", map[string]any{"a": nil, "null": "v"}},
			{"? a\n: 1\n: v\n", map[string]any{"a": uint64(1), "null": "v"}},
			{"? a\n:\nk: v\n", map[string]any{"a": nil, "k": "v"}},
			{"? a\n:\n", map[string]any{"a": nil}},
		} {
			var got any
			require.NoErrorf(t, codec.Unmarshal([]byte(tc.src), &got), "%q", tc.src)
			assert.Equalf(t, tc.want, got, "%q", tc.src)
		}
	})
}

// TestDefectACommentAboveABlankLineAddsALeadingBreak pins it.
//
// A sequence entry carrying a comment, with a blank line before its content,
// renders with a blank line at the head of the document. Rendering again drops
// it, so the first pass does not settle and the second does.
//
// The value survives every pass, and the tab is not part of the shape: a space
// does it too, and without the blank line it settles on the first pass.
func TestDefectACommentAboveABlankLineAddsALeadingBreak(t *testing.T) {
	t.Run("today the first render adds a leading break", func(t *testing.T) {
		for _, src := range []string{"-\t#\n\n e\n", "- #\n\n e\n", "-\t# c\n\n e\n"} {
			f, err := parser.ParseBytes([]byte(src), parser.WithComments())
			require.NoErrorf(t, err, "%q", src)

			once := f.String()
			assert.Truef(t, strings.HasPrefix(once, "\n"), "today: %q renders to %q", src, once)

			g, err := parser.ParseBytes([]byte(once), parser.WithComments())
			require.NoErrorf(t, err, "%q", once)
			assert.Equalf(t, strings.TrimPrefix(once, "\n"), g.String(),
				"the second render settles: %q", once)
		}
	})

	t.Run("the value survives every pass", func(t *testing.T) {
		const src = "-\t#\n\n e\n"

		var got any
		require.NoError(t, codec.Unmarshal([]byte(src), &got))
		assert.Equal(t, []any{"e"}, got)
	})

	t.Run("without the blank line it settles on the first pass", func(t *testing.T) {
		f, err := parser.ParseBytes([]byte("-\t#\n e\n"), parser.WithComments())
		require.NoError(t, err)
		assert.Equal(t, "- e #\n", f.String())
	})
}

// TestDefectACollectionKeySpelledTwoWaysLosesAnEntry pins it.
//
// 3.2.1.1 makes two keys equal when they resolve to the same node. Two
// collection keys that resolve alike and are written differently are one key,
// and the document holds a duplicate. `[a]: 1` over `[ a ]: 2` reads as
// {"[a]": 2}: no error, and the 1 is gone.
//
// The duplicate check names a collection key by the source text between the
// node's first and last token, so two spellings are two names and the repeat is
// missed. What the walk then builds names the key from what it resolved to --
// KeyText's Go %v of the sequence -- and those two names are the same, so the
// map keeps the last entry and the first disappears.
//
// So the two namings disagree, and the gap between them is where the value
// goes. Naming by spelling was chosen in `e842b79` to miss a repeat rather than
// invent one, which is the safe direction for a *refusal*; this is the other
// half of that choice, and a space inside a flow sequence is enough to reach
// it.
func TestDefectACollectionKeySpelledTwoWaysLosesAnEntry(t *testing.T) {
	t.Run("today the first entry is dropped and nothing is reported", func(t *testing.T) {
		for _, tc := range []struct {
			src  string
			want map[string]any
		}{
			// Whitespace alone, in flow and in block.
			{"{[a]: 1, [ a ]: 2}\n", map[string]any{"[a]": uint64(2)}},
			{"[a]: 1\n[ a ]: 2\n", map[string]any{"[a]": uint64(2)}},
			{"? [a]\n: 1\n? [ a ]\n: 2\n", map[string]any{"[a]": uint64(2)}},
			// A trailing comma, which 7.4 allows and which changes nothing.
			{"{[a]: 1, [a,]: 2}\n", map[string]any{"[a]": uint64(2)}},
			// Quoting, which settles presentation and not the node.
			{`{[a]: 1, ["a"]: 2}` + "\n", map[string]any{"[a]": uint64(2)}},
			{`{['a']: 1, ["a"]: 2}` + "\n", map[string]any{"[a]": uint64(2)}},
			// A tag that agrees with what the scalar already resolves to.
			{"{[1]: x, [!!int 1]: y}\n", map[string]any{"[1]": "y"}},
			// A mapping key, where the space is inside the entry.
			{"{{a: 0}: 1, {a: 0 }: 2}\n", map[string]any{"map[a:0]": uint64(2)}},
		} {
			var got any
			require.NoErrorf(t, codec.Unmarshal([]byte(tc.src), &got), "%q", tc.src)
			assert.Equalf(t, tc.want, got, "today: one entry short, and no error: %q", tc.src)
		}
	})

	// The same two documents spelled alike are refused, which is what places
	// this in the naming rather than in the check.
	t.Run("spelled alike they are refused", func(t *testing.T) {
		for _, src := range []string{"{[a]: 1, [a]: 2}\n", "[a]: 1\n[a]: 2\n"} {
			var got any
			err := codec.Unmarshal([]byte(src), &got)
			require.Errorf(t, err, "%q", src)
			assert.Containsf(t, err.Error(), "already defined", "%q", src)
		}
	})
}

// TestDefectTwoBlockCollectionKeysCollide pins it.
//
// A collection key written in block is named by its first indicator -- "-" for
// a sequence and ":" for a mapping -- so every block collection key in one
// mapping is the same key as every other. `? - a` over `: 1` over `? - b` over
// `: 2` is refused with `mapping key "-" already defined`, where 3.2.1.1 makes
// [a] and [b] two keys.
//
// The residue of `e842b79`, which fixed the flow half by naming a collection
// key from the source text between the node's first and last token. A block
// collection's span reaches only its first indicator, so the name comes back as
// one character. Before that commit every collection key collided in both
// spellings, so this is half a fix rather than a new fault.
func TestDefectTwoBlockCollectionKeysCollide(t *testing.T) {
	t.Run("today two block collection keys are one key", func(t *testing.T) {
		for _, tc := range []struct{ src, named string }{
			{"? - a\n: 1\n? - b\n: 2\n", "-"},
			{"?\n  a: 0\n: 1\n?\n  b: 0\n: 2\n", ":"},
		} {
			var got any
			err := codec.Unmarshal([]byte(tc.src), &got)
			require.Errorf(t, err, "%q", tc.src)
			assert.Containsf(t, err.Error(), `mapping key "`+tc.named+`" already defined`, "%q", tc.src)
		}
	})

	// The same keys in flow read, which places this in how the name is taken
	// rather than in the duplicate check. A block key beside a flow key reads
	// too, since the two names differ by accident.
	t.Run("in flow the same two keys read", func(t *testing.T) {
		for _, tc := range []struct {
			src  string
			want map[string]any
		}{
			{"? [a]\n: 1\n? [b]\n: 2\n", map[string]any{"[a]": uint64(1), "[b]": uint64(2)}},
			{"? - a\n: 1\n? [b]\n: 2\n", map[string]any{"[a]": uint64(1), "[b]": uint64(2)}},
		} {
			var got any
			require.NoErrorf(t, codec.Unmarshal([]byte(tc.src), &got), "%q", tc.src)
			assert.Equalf(t, tc.want, got, "%q", tc.src)
		}
	})
}
