// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/parser"
)

// Shapes the generator found that no longer diverge.
//
// A fixed defect leaves this much behind: the document that showed it, now
// asserting the behavior that is correct. It is worth keeping separately from
// the fix's own tests, because the generator reached these through a path
// nobody chose, and the same path will be walked again.

// TestFixedSingleQuotedKeyKeepsItsEscaping: a mapping key read from a
// single-quoted scalar is written back with its quote still doubled.
//
// The doubling used to be dropped, so a key holding one quote came back
// holding none of its escaping and the rendered document no longer parsed -- a
// valid document rendering to an invalid one. The same string in a value
// position was unaffected, which is what placed it in how keys were written
// rather than in single-quoted scalars, and the fix's own tests cover the value
// position alone.
func TestFixedSingleQuotedKeyKeepsItsEscaping(t *testing.T) {
	file, err := parser.ParseBytes([]byte("'a''b': false\n"), parser.ParseComments)
	require.NoError(t, err)

	rendered := file.String()
	assert.Equal(t, "'a''b': false\n", rendered)

	reread, err := parser.ParseBytes([]byte(rendered), parser.ParseComments)
	require.NoError(t, err, "the rendered document must still parse")
	assert.Equal(t, rendered, reread.String(), "and rendering settles")
}

// TestFixedKeyThatIsOnlyAQuote is the smallest document of the same shape,
// which is what the reducer arrived at.
func TestFixedKeyThatIsOnlyAQuote(t *testing.T) {
	require.False(t, renderChangesValue([]byte("'''':\n")))
}

// TestFixedStripChompingKeepsTrailingSpaces: `|-` removes the trailing line
// break and nothing else.
//
// Chomping is defined over line breaks: b-chomped-last and l-chomped-empty say
// what happens to the break that ends the last line and to the empty lines
// after it. A space before that break is content -- l-nb-literal-text matches
// nb-char+, and nb-char includes a space -- so it survives every chomping mode.
// It used to be trimmed along with the break, and only under `|-`, so the same
// content read two ways gave two values.
func TestFixedStripChompingKeepsTrailingSpaces(t *testing.T) {
	values := map[string]any{
		"stripped":              "trailing ",
		"clipped":               "trailing \n",
		"not on the last line":  "a \nb",
		"a whole line of space": "a\n ",
	}
	sources := map[string]string{
		"stripped":              "|-\n  trailing \n",
		"clipped":               "|\n  trailing \n",
		"not on the last line":  "|-\n  a \n  b\n",
		"a whole line of space": "|-\n  a\n   \n",
	}

	for name, src := range sources {
		t.Run(name, func(t *testing.T) {
			var got any
			require.NoError(t, yaml.Unmarshal([]byte(src), &got))
			assert.Equal(t, values[name], got)
		})
	}

	// An empty line carries no content, so stripping takes it whole.
	t.Run("an empty trailing line is still chomped", func(t *testing.T) {
		var got any
		require.NoError(t, yaml.Unmarshal([]byte("|-\n  a\n\n"), &got))
		assert.Equal(t, "a", got)
	})
}

// TestFixedBlockScalarsRenderTheirChomping: rendering writes back every line
// the chomping indicator settled on.
//
// A "|+" used to write the indicator without the blank lines it exists to
// preserve, so "a\n\n" came back as "a\n" -- the indicator was kept and its
// whole effect thrown away. Rendering took the content from the source text and
// trimmed the trailing whitespace off it, which is right for a "|" and wrong
// for the two indicators that exist to say what happens to that whitespace.
//
// The content now comes from the value, which is what chomping has already
// settled, so the indicator needs no arithmetic here at all.
func TestFixedBlockScalarsRenderTheirChomping(t *testing.T) {
	sources := map[string]string{
		"keep one blank line":    "k: |+\n  trail\n\n",
		"keep two":               "k: |+\n  trail\n\n\n",
		"keep with no content":   "k: |+\n\n\n",
		"clip":                   "k: |\n  trail\n",
		"strip":                  "k: |-\n  trail\n",
		"strip a trailing space": "k: |-\n  trail \n",
		"a stated indent":        "k: |2\n   x\n",
		"several lines":          "k: |\n  a\n  b\n",
		"a blank line inside":    "k: |\n  a\n\n  b\n",
		"in a sequence":          "- |+\n  keep\n\n- x\n",
	}

	for name, src := range sources {
		t.Run(name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(src), parser.ParseComments)
			require.NoError(t, err)
			assert.Equal(t, src, file.String(), "the document is written back as it was read")

			var before, after any
			require.NoError(t, yaml.Unmarshal([]byte(src), &before))
			require.NoError(t, yaml.Unmarshal([]byte(file.String()), &after))
			assert.Equal(t, before, after, "and means the same")
		})
	}
}

// TestFixedCommentOnASequenceEntryStaysThere: a comment written on a sequence
// entry's own line is kept on that line.
//
// It is recorded on the entry rather than on its value, because an entry whose
// value is written below it -- or is not written at all -- has nothing on that
// line to carry it. Rendering read only the values, so such a comment was
// dropped; and the one on the first entry's dash was read a second time as the
// whole sequence's comment, which is how it also reappeared at the head of the
// document. Neither survived a second pass unchanged.
func TestFixedCommentOnASequenceEntryStaysThere(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"on an entry with no value":      {"-  # c\n", "-  # c\n"},
		"under a head comment":           {"# c1\n-  # c2\n", "# c1\n-  # c2\n"},
		"beside a sibling":               {"-  # c\n- y\n", "-  # c\n- y\n"},
		"on an entry holding a sequence": {"-  # c\n  - x\n", "- # c\n  - x\n"},
		"on an entry holding a mapping":  {"-  # c\n  a: 1\n", "- # c\n  a: 1\n"},
		"written below the dash":         {"-\n# c\n - x\n", "- # c\n  - x\n"},

		// A scalar on the entry's line carries its own comment, and a block
		// scalar's header is what shares the line -- a comment after it is
		// where YAML puts one.
		"on a scalar entry":      {"- x # c\n", "- x # c\n"},
		"after a block header":   {"- | # c\n  x\n", "- | # c\n  x\n"},
		"on a mapping entry key": {"k: # c\n  j: 1\n", "k: # c\n  j: 1\n"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(test.source), parser.ParseComments)
			require.NoError(t, err)

			rendered := file.String()
			assert.Equal(t, test.want, rendered)

			reread, err := parser.ParseBytes([]byte(rendered), parser.ParseComments)
			require.NoError(t, err, "the rendered document must still parse")
			assert.Equal(t, rendered, reread.String(), "and rendering settles in one pass")
		})
	}
}

// TestFixedAnchoredEmptyEntryEndsWhereItsLineDoes: an entry whose line ends on
// an anchor that names nothing keeps the entries after it as siblings.
//
// Whatever such an anchor names has to be written inside the entry, further in
// than its '-' or its key. The parser looked only for the next entry of the
// same collection at the same column, so anything else -- a comment line, or
// the key of an enclosing mapping written level with the entry because a block
// sequence sits at its key's own column -- was taken as the anchor's value, and
// every later entry went down into it. Two siblings came back as one nested in
// the other: a document that still parses, still settles, and means something
// else.
func TestFixedAnchoredEmptyEntryEndsWhereItsLineDoes(t *testing.T) {
	values := map[string]any{
		"a comment then a sibling":            []any{nil, "x"},
		"an indented comment then a sibling":  []any{nil, "x"},
		"a mapping entry does the same":       map[string]any{"k": nil, "j": "x"},
		"an outer key level with the entry":   map[string]any{"": []any{nil}, " ": nil},
		"no comment":                          []any{nil, "x"},
		"nothing follows":                     []any{"x", nil},
		"the anchor names an indented value":  []any{[]any{"x"}},
		"the anchor names a mapping":          map[string]any{"k": map[string]any{"a": uint64(1)}, "j": "x"},
		"the anchor names an indented nested": []any{nil, "x"},
	}
	sources := map[string]string{
		"a comment then a sibling":            "- &a1\n#\n- x\n",
		"an indented comment then a sibling":  "- &a1\n  #\n- x\n",
		"a mapping entry does the same":       "k: &a1\n#\nj: x\n",
		"an outer key level with the entry":   "\"\": &a2\n - &a1\n\" \":\n",
		"no comment":                          "- &a1\n- x\n",
		"nothing follows":                     "- x\n- &a1\n#\n",
		"the anchor names an indented value":  "- &a1\n  - x\n",
		"the anchor names a mapping":          "k: &a1\n  a: 1\n#\nj: x\n",
		"the anchor names an indented nested": "- &a1\n  # c\n- x\n",
	}

	for name, src := range sources {
		t.Run(name, func(t *testing.T) {
			var before any
			require.NoError(t, yaml.Unmarshal([]byte(src), &before))
			assert.Equal(t, values[name], before, "reading is correct")

			file, err := parser.ParseBytes([]byte(src), parser.ParseComments)
			require.NoError(t, err)
			rendered := file.String()

			var after any
			require.NoError(t, yaml.Unmarshal([]byte(rendered), &after))
			assert.Equal(t, before, after, "and rendering keeps it")

			reread, err := parser.ParseBytes([]byte(rendered), parser.ParseComments)
			require.NoError(t, err)
			assert.Equal(t, rendered, reread.String(), "and settles in one pass")
		})
	}
}
