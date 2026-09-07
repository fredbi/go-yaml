// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"math"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/codec"
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
	file, err := parser.ParseBytes([]byte("'a''b': false\n"), parser.WithComments())
	require.NoError(t, err)

	rendered := file.String()
	assert.Equal(t, "'a''b': false\n", rendered)

	reread, err := parser.ParseBytes([]byte(rendered), parser.WithComments())
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
			file, err := parser.ParseBytes([]byte(src), parser.WithComments())
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
			file, err := parser.ParseBytes([]byte(test.source), parser.WithComments())
			require.NoError(t, err)

			rendered := file.String()
			assert.Equal(t, test.want, rendered)

			reread, err := parser.ParseBytes([]byte(rendered), parser.WithComments())
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

			file, err := parser.ParseBytes([]byte(src), parser.WithComments())
			require.NoError(t, err)
			rendered := file.String()

			var after any
			require.NoError(t, yaml.Unmarshal([]byte(rendered), &after))
			assert.Equal(t, before, after, "and rendering keeps it")

			reread, err := parser.ParseBytes([]byte(rendered), parser.WithComments())
			require.NoError(t, err)
			assert.Equal(t, rendered, reread.String(), "and settles in one pass")
		})
	}
}

// TestFixedBlankLineUnderAStatedIndent: a block scalar may state its
// indentation and keep its trailing blank lines at the same time.
//
// A blank line carries no indentation of its own, and YAML allows that:
// l-empty admits s-indent(<n), so an empty line may be indented less than the
// header states. Holding the last line to the stated width refused every such
// document -- and only when both features were present, since the padded and
// mid-content spellings were always accepted.
func TestFixedBlankLineUnderAStatedIndent(t *testing.T) {
	values := map[string]any{
		"a kept blank line":          map[string]any{"k": "one\n\n"},
		"two kept blank lines":       map[string]any{"k": "one\n\n\n"},
		"the blank line padded out":  map[string]any{"k": "one\n\n"},
		"a blank line in the middle": map[string]any{"k": "one\n\ntwo\n"},
		"without the indicator":      map[string]any{"k": "one\n\n"},
	}
	sources := map[string]string{
		"a kept blank line":          "k: |2+\n  one\n\n",
		"two kept blank lines":       "k: |2+\n  one\n\n\n",
		"the blank line padded out":  "k: |2+\n  one\n  \n",
		"a blank line in the middle": "k: |2\n  one\n\n  two\n",
		"without the indicator":      "k: |+\n  one\n\n",
	}

	for name, src := range sources {
		t.Run(name, func(t *testing.T) {
			var got any
			require.NoError(t, yaml.Unmarshal([]byte(src), &got))
			assert.Equal(t, values[name], got)
		})
	}

	// Content that really is indented less than the header states is still
	// refused: it is the empty line that is exempt, not the rule.
	for name, src := range map[string]string{
		"content short of the stated width": "k: |2\n x\n",
		"content short by more":             "k: |4\n  x\n",
	} {
		t.Run(name, func(t *testing.T) {
			var v any
			assert.Error(t, yaml.Unmarshal([]byte(src), &v))
		})
	}
}

// TestFixedStatedIndentFollowsTheContent: a block scalar that states its
// indentation has the header rewritten when the content moves.
//
// The width is counted from the indentation of whatever encloses the scalar,
// and the renderer writes the content at its own width, so carrying the
// source's number over left the header describing a layout that was no longer
// there -- the value gained a column on every cycle. A document encloses
// nothing, and the spec gives its node an indentation of -1, so at the root the
// same content is stated one higher than anywhere else.
func TestFixedStatedIndentFollowsTheContent(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"at the document root":         {"|2\n a\n", "|3\n  a\n"},
		"already at the root width":    {"|3\n  a\n", "|3\n  a\n"},
		"behind an anchor at the root": {"&a |2\n x\n", "&a |3\n  x\n"},
		"under a mapping key":          {"k: |2\n   x\n", "k: |2\n   x\n"},
		"under a sequence entry":       {"- |2\n   x\n", "- |2\n   x\n"},
		"narrower than the renderer":   {"k: |1\n  x\n", "k: |2\n   x\n"},

		// A property stays on the line the header ends, so what it names is
		// still indented from the start of that line and not from the property.
		"behind an anchor in a sequence": {"- &a |\n  x\n", "- &a |\n  x\n"},
		"behind a tag in a sequence":     {"- !!str |\n  x\n", "- !!str |\n  x\n"},
		"behind an anchor under a key":   {"k: &a |\n  x\n", "k: &a |\n  x\n"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var before any
			require.NoError(t, yaml.Unmarshal([]byte(test.source), &before))

			file, err := parser.ParseBytes([]byte(test.source), parser.WithComments())
			require.NoError(t, err)
			rendered := file.String()
			assert.Equal(t, test.want, rendered)

			var after any
			require.NoError(t, yaml.Unmarshal([]byte(rendered), &after))
			assert.Equal(t, before, after, "the value survives the move")

			reread, err := parser.ParseBytes([]byte(rendered), parser.WithComments())
			require.NoError(t, err)
			assert.Equal(t, rendered, reread.String(), "and settles in one pass")
		})
	}
}

// TestFixedKeepChompingKeepsItsBlankLinesWhenFolded: `>+` is written back with
// the trailing blank lines it exists to preserve.
//
// A folded scalar's value has lost its line structure, so the renderer writes
// its content back from the source text -- and the source ends the same way
// whatever the header says, since `>`, `>-` and `>+` differ only in what they
// make of the blank lines after the content. Reading the tail back from the
// source would have kept them under all three, so it was cut under all three,
// and keep chomping lost the only thing that distinguishes it. What the value
// ends on is where that decision has already been made, and the tail is
// rebuilt from there.
func TestFixedKeepChompingKeepsItsBlankLinesWhenFolded(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"keep":                     {"k: >+\n  trail\n\n", "k: >+\n  trail\n\n"},
		"keep, several":            {"k: >+\n  trail\n\n\n", "k: >+\n  trail\n\n\n"},
		"keep, nothing but blanks": {"k: >+\n\n\n", "k: >+\n\n\n"},
		"keep, folded over a gap":  {"k: >+\n  a\n\n  b\n\n", "k: >+\n  a\n\n  b\n\n"},
		"keep, stated width":       {"k: >2+\n   trail\n\n", "k: >2+\n   trail\n\n"},
		"keep, under a sequence":   {"- >+\n  trail\n\n", "- >+\n  trail\n\n"},
		"keep, at the root":        {">+\n  trail\n\n", ">+\n  trail\n\n"},

		// The other two chomping modes drop the blank lines when reading, so
		// the tail they are written with is empty -- which is what the source
		// text alone could not tell them apart by.
		"clip":  {"k: >\n  trail\n\n", "k: >\n  trail\n"},
		"strip": {"k: >-\n  trail\n\n", "k: >-\n  trail\n"},

		// A line of spaces is blank up to the width that introduced the block
		// and content past it: a folded scalar treats a line indented further
		// than its neighbors literally, so those spaces are the value.
		"a blank line of the block's own width": {"k: >+\n  a\n  \n", "k: >+\n  a\n\n"},
		"a blank line indented further":         {"k: >+\n  a\n     \n", "k: >+\n  a\n     \n"},
		"and one under strip chomping":          {"k: >-\n  a\n     \n", "k: >-\n  a\n     \n"},

		// Trailing spaces on a line of content are content under every mode,
		// the same way they are for the literal spelling.
		"a space before the break": {"k: >+\n  trail \n\n", "k: >+\n  trail \n\n"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var before any
			require.NoError(t, yaml.Unmarshal([]byte(test.source), &before))

			file, err := parser.ParseBytes([]byte(test.source), parser.WithComments())
			require.NoError(t, err)
			rendered := file.String()
			assert.Equal(t, test.want, rendered)

			var after any
			require.NoError(t, yaml.Unmarshal([]byte(rendered), &after))
			assert.Equal(t, before, after, "the value survives being written out")

			reread, err := parser.ParseBytes([]byte(rendered), parser.WithComments())
			require.NoError(t, err)
			assert.Equal(t, rendered, reread.String(), "and settles in one pass")
		})
	}
}

// TestFixedFoldedScalarWritesItsOwnBreak: a folded block scalar is written with
// "\n", whatever line break the document it came from used.
//
// The renderer used to copy the source's break into the scalar it wrote. A CRLF
// document then rendered to a mixture of both and was a third document on the
// next render; with a lone CR the content lines drifted a column right, so a
// fold stopped folding and "x y" came back as "x\n y"; and nested, the content
// landed at the parent's own column, which the grammar refuses outright.
//
// The break was doing two jobs: reading the origin, where it has to be the
// source's, and writing the output, where it has to be "\n". Splitting them
// left one thing to get right in the order -- the content is normalized before
// dedentBy takes it apart, that function looking for "\n".
func TestFixedFoldedScalarWritesItsOwnBreak(t *testing.T) {
	for _, test := range []struct {
		name, src, want string
		value           any
	}{
		{"CRLF", ">-\r\n x\r\n", ">-\n  x\n", "x"},
		{"lone CR, two lines", ">-\r x\r y\r", ">-\n  x\n  y\n", "x y"},
		{"LF, unchanged", ">-\n x\n y\n", ">-\n  x\n  y\n", "x y"},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(test.src), parser.WithComments())
			require.NoError(t, err)

			once := file.String()
			assert.Equal(t, test.want, once, "the renderer writes its own break")

			second, err := parser.ParseBytes([]byte(once), parser.WithComments())
			require.NoError(t, err)
			assert.Equal(t, once, second.String(), "and the document settles")

			var got any
			require.NoError(t, yaml.Unmarshal([]byte(once), &got))
			assert.Equal(t, test.value, got, "the fold still folds")
		})
	}
}

// TestFixedFoldedScalarNestedRendersYAML: a folded scalar under a mapping key
// renders content indented past the key, not level with it.
func TestFixedFoldedScalarNestedRendersYAML(t *testing.T) {
	const src = "a:\r  b: >\r   x\rc: 1\r"

	file, err := parser.ParseBytes([]byte(src), parser.WithComments())
	require.NoError(t, err)

	assert.Equal(t, "a:\n  b: >\n    x\nc: 1\n", file.String())

	var got any
	require.NoError(t, yaml.Unmarshal([]byte(file.String()), &got))
	// ">" clips rather than strips, so the value keeps one trailing break.
	assert.Equal(t, map[string]any{"a": map[string]any{"b": "x\n"}, "c": uint64(1)}, got)
}

// TestFixedCRLFComentDoesNotBlankALine: a comment closing a CRLF line leaves the
// token after it on the next line, not two down.
//
// scanComment stopped at the '\r' and left the '\n' to whatever came next,
// whose leading whitespace was then read as a second break. The document gained
// a blank line next to the comment when it was written back, and lost it again
// on the render after that, so it never settled.
//
// Any comment did it, at the end of a line as much as on one of its own. The
// ledger entry that recorded it asked for a standalone comment and so missed
// `- #\r\n -\r\n`, which failed TestRenderReachesAFixedPoint at a seed the
// earlier runs had not drawn; `- 1 # c1\r\n- 2\r\n` gained its blank line with
// no open entry anywhere. Both are below, because a fix is worth no more than
// the shapes it is held to.
func TestFixedCRLFComentDoesNotBlankALine(t *testing.T) {
	for _, test := range []struct{ name, src string }{
		{"CRLF", "# c1\r\n-\r\n# c2\r\n- 1\r\n"},
		{"lone CR", "# c1\r-\r# c2\r- 1\r"},
		{"LF", "# c1\n-\n# c2\n- 1\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(test.src), parser.WithComments())
			require.NoError(t, err)

			once := file.String()
			assert.Equal(t, "# c1\n- \n# c2\n- 1\n", once,
				"every line break writes the same document")

			again, err := parser.ParseBytes([]byte(once), parser.WithComments())
			require.NoError(t, err)
			assert.Equal(t, once, again.String(), "and it settles in one pass")
		})
	}

	// A comment at the end of a line, which the entry above did not ask for.
	for _, test := range []struct{ name, src, want string }{
		{"a line comment over an open entry", "- #\r\n -\r\n", "- #\n  - \n"},
		{"a line comment and no open entry", "- 1 # c1\r\n- 2\r\n", "- 1 # c1\n- 2\n"},
		{"a line comment introducing a block", "- # c1\r\n  - x\r\n", "- # c1\n  - x\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(test.src), parser.WithComments())
			require.NoError(t, err)

			once := file.String()
			assert.Equal(t, test.want, once, "no blank line either side of the comment")

			again, err := parser.ParseBytes([]byte(once), parser.WithComments())
			require.NoError(t, err)
			assert.Equal(t, once, again.String(), "and it settles in one pass")
		})
	}
}

// TestFixedStrTagKeepsTheSpelling: `!!str` settles what a plain scalar is, so
// the spelling that went in is the one that comes back.
//
// The scalar used to be resolved first and the result turned into text, so
// `!!str null`, `!!str Null`, `!!str NULL` and `!!str ~` all read "" and
// `!!str True` and `!!str FALSE` read "true" and "false". The library
// disagreed with itself about it, which is what made it a defect rather than a
// reading of the spec: `!<tag:yaml.org,2002:str> null` is the same tag spelled
// verbatim and read "null", as did `! null`, `!foo null` and `!!str "null"`.
func TestFixedStrTagKeepsTheSpelling(t *testing.T) {
	for _, src := range []string{
		"!!str null\n", "!!str Null\n", "!!str NULL\n", "!!str ~\n",
		"!!str True\n", "!!str TRUE\n", "!!str False\n", "!!str FALSE\n",
		"!!str 5\n", "!!str on\n", "!!str true\n", "!!str x\n",
	} {
		wellFormed(t, src)

		var got any
		require.NoError(t, yaml.Unmarshal([]byte(src), &got))
		assert.Equal(t, strings.TrimSuffix(strings.TrimPrefix(src, "!!str "), "\n"), got, "%q", src)
	}

	t.Run("every spelling of the same tag gives the same text", func(t *testing.T) {
		for _, src := range []string{
			"!!str null\n", "!<tag:yaml.org,2002:str> null\n", "! null\n", "!foo null\n", "!!str \"null\"\n",
		} {
			var got any
			require.NoError(t, yaml.Unmarshal([]byte(src), &got))
			assert.Equal(t, "null", got, "%q", src)
		}
	})

	t.Run("and the tag on nothing is the empty string", func(t *testing.T) {
		var got any
		require.NoError(t, yaml.Unmarshal([]byte("a: !!str\nb: 1\n"), &got))
		assert.Equal(t, map[string]any{"a": "", "b": uint64(1)}, got)
	})
}

// TestFixedACollectionTagBeforeAnAnchorParses covers a collection tag written
// in front of an anchor.
//
// YAML 1.2 lets a node's tag and anchor stand in either order and means the
// same by both. Written second the tag held; written first it stopped the parse
// outright -- "a: !!seq &a1 [1]" was refused with "value is not allowed in this
// context" and "a: !!map &a1 {b: 1}" with "could not find map", so a document
// carrying one could not be read, rendered or reformatted at all.
//
// Fixed on 2026-09-07 by parseTagValue reading whatever node follows a
// collection tag rather than insisting on the kind: what the tag made of the
// node is ast.TagNode.Resolve's to report, and the load refuses a kind the tag
// does not name. The tag before the anchor stopped being a parse question on
// the way.
//
// The other two shapes of that defect are still open, in
// TestDefectTagBeforeAnchorIsDropped: a tag on an empty node swallows what
// follows it, and any other tag is dropped from the node the anchor names.
func TestFixedACollectionTagBeforeAnAnchorParses(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want any
	}{
		{src: "a: !!seq &a1 [1]\n", want: map[string]any{"a": []any{uint64(1)}}},
		{src: "a: !!map &a1 {b: 1}\n", want: map[string]any{"a": map[string]any{"b": uint64(1)}}},

		// The anchor written first, which always read, so the two orders agree.
		{src: "a: &a1 !!seq [1]\n", want: map[string]any{"a": []any{uint64(1)}}},
		{src: "a: &a1 !!map {b: 1}\n", want: map[string]any{"a": map[string]any{"b": uint64(1)}}},
	} {
		t.Run(tc.src, func(t *testing.T) {
			wellFormed(t, tc.src)

			var got any
			require.NoError(t, yaml.Unmarshal([]byte(tc.src), &got))
			assert.Equal(t, tc.want, got)

			// And the anchor names the tagged node, so an alias to it is the
			// same value.
			aliased := strings.TrimSuffix(tc.src, "\n") + "\nb: *a1\n"

			var both map[string]any
			require.NoError(t, yaml.Unmarshal([]byte(aliased), &both))
			assert.Equal(t, both["a"], both["b"], "%q", aliased)
		})
	}

	t.Run("and the document renders as it was written", func(t *testing.T) {
		for _, src := range []string{"a: !!seq &a1 [1]\n", "a: !!map &a1 {b: 1}\n"} {
			assert.Equal(t, src, renderOnce(t, src), "%q", src)
		}
	})
}

// TestFixedANullKeepsTheSpellingItWasWrittenWith covers a null rendering as the
// document spelled it.
//
// ast.NullNode.String wrote the four letters "null" whatever the token held, so
// "Null", "NULL" and "~" all came back as "null". Every other scalar node
// already rendered from its token -- "!!str True" keeps its capital T and
// "!!str .INF" its capitals -- and the null was the one that did not.
//
// Untagged it cost the spelling and not the value, since YAML resolves all four
// to the same node. Under "!!str" it cost the value too: the tag names the
// characters, and "Null" is not "null". Reading it was fixed first; writing it
// back is the half that was not.
func TestFixedANullKeepsTheSpellingItWasWrittenWith(t *testing.T) {
	for _, src := range []string{
		"k: !!str Null\n",
		"k: !!str NULL\n",
		"k: !!str null\n",
		"k: !!str ~\n",
		"!!str Null\n",
		"&a1 !!str Null\n",

		// Untagged, where only the spelling was at stake.
		"k: Null\n",
		"k: NULL\n",
		"k: ~\n",
		"k: null\n",
	} {
		t.Run(src, func(t *testing.T) {
			wellFormed(t, src)
			assert.Equal(t, src, renderOnce(t, src))
		})
	}

	t.Run("the value survives the write as well as the read", func(t *testing.T) {
		for _, src := range []string{"k: !!str Null\n", "k: !!str NULL\n", "k: !!str ~\n"} {
			want := strings.TrimSuffix(strings.TrimPrefix(src, "k: !!str "), "\n")

			var got any
			require.NoError(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equal(t, map[string]any{"k": want}, got, "the decode")

			var again any
			require.NoError(t, yaml.Unmarshal([]byte(renderOnce(t, src)), &again), "%q", src)
			assert.Equal(t, got, again, "the render, read back")
		}
	})

	t.Run("a null the document never wrote still has no text", func(t *testing.T) {
		// An implicit null is the absence of a value, so it renders as nothing.
		// Writing "null" for it would put a value where the document had none.
		for _, src := range []string{"k:\n", "k: !!str\n", "- \n"} {
			wellFormed(t, src)
			assert.NotContains(t, renderOnce(t, src), "null", "%q", src)
		}
	})
}

// TestFixedACommentAfterALineEndingTagSurvives covers a comment sitting after a
// tag that is the last thing on its line.
//
// The parse attached it to the tag node correctly; Renderer.tag rendered only
// n.Start.Value and the node it stands on, so the comment on the node itself
// went unread. "!!null # c1" and "- &a2 !!seq # c2" came back without theirs.
//
// An anchor never lost one, which is what said this was the tag and not
// comments on a property line in general: Renderer.anchor renders the name
// node, and the parse hangs the comment there. Nor did a tag with anything
// after it on the line -- "!!null null # c1" -- because the comment had been
// hung on the scalar instead.
func TestFixedACommentAfterALineEndingTagSurvives(t *testing.T) {
	for _, src := range []string{
		"!!null # c1\n",
		"&a1 !!null # c1\n",
		"k: !!null # c1\n",
		"- !!null # c1\n",
		"- &a2 !!seq # c2\n  - 1\n",
		"!!seq # c1\n- 1\n",
		"!!str # c1\n",

		// The shapes that always kept it, so the fix did not move them.
		"&a1 # c1\n",
		"!!null null # c1\n",
		"&a1 !!str x # c1\n",
		"k: !!str x # c1\n",
	} {
		t.Run(src, func(t *testing.T) {
			wellFormed(t, src)
			assert.Equal(t, src, renderOnce(t, src))
		})
	}

	t.Run("and it is still dropped when comments are off", func(t *testing.T) {
		// The renderer writes no comment without WithComments, tag or no tag.
		f, err := parser.ParseBytes([]byte("!!null # c1\n"))
		require.NoError(t, err)
		assert.NotContains(t, f.String(), "#")
	})
}

// TestFixedACommentAboveAPropertyLineStaysOnTheKey covers a comment on the line
// that introduces a node whose properties are written on the next one.
//
// A comment claims the rest of the line it sits on, so what follows the key
// cannot start there. Renderer.fitsOnKeyLine says yes to an anchor and a tag --
// they carry their own value and decide their own shape -- and the comment was
// then pushed past the whole value and written on the last line of it:
// "k: # c1" over "&a2" over "- 1" came back as "k: &a2" over "- 1 # c1".
//
// Harmless where the last line was a plain scalar and not harmless at all where
// it was a block scalar: the comment landed inside the content and the value
// changed with nothing reporting it.
func TestFixedACommentAboveAPropertyLineStaysOnTheKey(t *testing.T) {
	for _, src := range []string{
		"k: # c1\n  &a2\n  - 1\n",
		"k: # c1\n  !!seq\n  - 1\n",
		"k: # c1\n  &a2 |-\n    x\n",
		"k: # c1\n  &a2\n  - |-\n    trailing \n",

		// A sequence entry takes the same rule from Renderer.sequence, which
		// asked fitsOnKeyLine the same question and got the same wrong answer.
		// TestRenderPreservesValue found this one at 50,000 draws, after the
		// mapping half was fixed and the entry half was not.
		"- # c1\n  &a2\n  - |-\n    trailing \n",
		"- # c1\n  !!seq\n  - 1\n",

		// The shapes that always rendered correctly, so the fix did not move
		// them: a collection under a commented key already went below it, and a
		// block scalar keeps its header on the line because a comment after the
		// header is where YAML puts one.
		"k: # c1\n  a: 1\n",
		"k: # c1\n  [1, 2]\n",
		"k: |- # c1\n  x\n",
		"k: &a2 x # c1\n",
		"- |- # c1\n  x\n",
		"- 1 # c1\n",
	} {
		t.Run(src, func(t *testing.T) {
			wellFormed(t, src)
			assert.Equal(t, src, renderOnce(t, src))
		})
	}

	t.Run("the comment stays on its own line where the layout normalizes", func(t *testing.T) {
		// A block mapping under a property is indented from the property, with
		// or without a comment -- "!!map" over "a: 1" renders the same way. So
		// these do not come back byte for byte, and what this holds is the one
		// thing the defect moved: the comment is still on the line that
		// introduced the node, and not somewhere inside the value.
		for _, src := range []string{
			"- # c1\n  &a2\n  a: 1\n",
			"k: # c1\n  &a2\n  a: 1\n",
		} {
			wellFormed(t, src)

			got := renderOnce(t, src)
			assert.Equal(t, 1, strings.Count(got, "#"), "%q rendered %q", src, got)
			assert.Contains(t, strings.SplitN(got, "\n", 2)[0], "# c1", "%q rendered %q", src, got)
		}
	})

	t.Run("the block scalar keeps its value", func(t *testing.T) {
		const src = "k: # c1\n  &a2\n  - |-\n    trailing \n"

		var before, after any
		require.NoError(t, yaml.Unmarshal([]byte(src), &before))
		require.NoError(t, yaml.Unmarshal([]byte(renderOnce(t, src)), &after))

		assert.Equal(t, []any{"trailing "}, before.(map[string]any)["k"])
		assert.Equal(t, before, after, "the render changed the value")
	})
}

// TestFixedATagOnAnEmptyNodeKeepsWhatFollows covers a tag and an anchor written
// at the end of a line with nothing after them.
//
// The anchor went looking for a value and took the next entry of the collection
// around it. "- !!null &a1" over "- x" decoded to a one-item sequence -- the
// second entry gone, and no error at all -- and "k: !!null &a1" over "j: x"
// lost j the same way.
//
// Whatever a property at the end of a line names has to be written inside the
// entry holding it, which means further in than that entry's own column.
// parseMapValue and parseSequenceValue said exactly this for a bare anchor,
// which is why "- &a1" over "- x" never lost anything; a tag before the anchor
// sends the descent down parseTagValue, too far from either to repeat the test,
// so the parser now carries the column there.
func TestFixedATagOnAnEmptyNodeKeepsWhatFollows(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want any
	}{
		{src: "- !!null &a1\n- x\n", want: []any{nil, "x"}},
		{src: "- !!str &a1\n- x\n", want: []any{"", "x"}},
		{src: "- !!seq &a1\n- x\n", want: []any{nil, "x"}},
		{src: "k: !!null &a1\nj: x\n", want: map[string]any{"k": nil, "j": "x"}},
		{src: "k: !!seq &a1\nj: x\n", want: map[string]any{"k": nil, "j": "x"}},

		// The anchor written first, which never lost anything.
		{src: "- &a1 !!null\n- x\n", want: []any{nil, "x"}},
		{src: "- &a1\n- x\n", want: []any{nil, "x"}},
	} {
		t.Run(tc.src, func(t *testing.T) {
			wellFormed(t, tc.src)

			var got any
			require.NoError(t, yaml.Unmarshal([]byte(tc.src), &got))
			assert.Equal(t, tc.want, got)
		})
	}

	t.Run("and a property still names what is written inside its entry", func(t *testing.T) {
		// The other side of the column rule: indented past the entry, what
		// follows belongs to the property and not to the collection around it.
		for _, tc := range []struct {
			src  string
			want any
		}{
			{src: "a: &a\n  foo: 1\nb: 2\n", want: map[string]any{"a": map[string]any{"foo": uint64(1)}, "b": uint64(2)}},
			{src: "a: !!map &m\n  foo: 1\nb: 2\n", want: map[string]any{"a": map[string]any{"foo": uint64(1)}, "b": uint64(2)}},

			// And at the document's root nothing encloses the property, so it
			// names what follows however far away it is written.
			{src: "!!str\n&a2\nscalar2\n", want: "scalar2"},
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(tc.src), &got), "%q", tc.src)
			assert.Equalf(t, tc.want, got, "%q", tc.src)
		}
	})
}

// TestFixedAnAnchorAfterATagNamesTheTaggedNode covers the last shape of the
// tag-before-anchor defect.
//
// §6.9 lets a node's tag and anchor stand in either order and means the same by
// both. Written anchor first the tree is Anchor over Tag over the value, and the
// anchor names the tagged node; written tag first it is Tag over Anchor over the
// value, and the anchor named the value with the tag stripped off it. So
// "a: !!int &a1 \"5\"" read 5 at a and "5" at "b: *a1" -- one node, read as a
// number where it stands and as a string through an alias to it.
//
// The tree still keeps the order the document wrote, so it renders as it was
// written. Only what the name stands for changed.
func TestFixedAnAnchorAfterATagNamesTheTaggedNode(t *testing.T) {
	for _, tc := range []struct {
		tagFirst, anchorFirst string
		want                  any
	}{
		{
			tagFirst:    "a: !!int &a1 \"5\"\nb: *a1\n",
			anchorFirst: "a: &a1 !!int \"5\"\nb: *a1\n",
			want:        map[string]any{"a": 5, "b": 5},
		},
		{
			tagFirst:    "a: !!str &a1 5\nb: *a1\n",
			anchorFirst: "a: &a1 !!str 5\nb: *a1\n",
			want:        map[string]any{"a": "5", "b": "5"},
		},
		{
			tagFirst:    "a: !!float &a1 1\nb: *a1\n",
			anchorFirst: "a: &a1 !!float 1\nb: *a1\n",
			want:        map[string]any{"a": float64(1), "b": float64(1)},
		},
		{
			tagFirst:    "a: !!binary &a1 aGk=\nb: *a1\n",
			anchorFirst: "a: &a1 !!binary aGk=\nb: *a1\n",
			want:        map[string]any{"a": []byte("hi"), "b": []byte("hi")},
		},
	} {
		t.Run(tc.tagFirst, func(t *testing.T) {
			wellFormed(t, tc.tagFirst)
			wellFormed(t, tc.anchorFirst)

			var written, other any
			require.NoError(t, yaml.Unmarshal([]byte(tc.tagFirst), &written))
			require.NoError(t, yaml.Unmarshal([]byte(tc.anchorFirst), &other))

			assert.Equal(t, tc.want, written, "tag first")
			assert.Equal(t, tc.want, other, "anchor first")

			// And each order still renders as it was written.
			assert.Equal(t, tc.tagFirst, renderOnce(t, tc.tagFirst))
			assert.Equal(t, tc.anchorFirst, renderOnce(t, tc.anchorFirst))
		})
	}
}

// TestFixedFoldedScalarGainsNoBreakWhenRendered: a folded block scalar is
// written back with the blank lines the document gave it, and no more.
//
// token.Lookback.blankLineAbove asks linesSpannedBy how many lines a scalar
// occupies past its first, then subtracts that from the gap to the next token.
// linesSpannedBy counted the breaks in the scalar's value, which measures the
// source only for a literal block. Folding rewrites the line structure, so
// "  x\n\n  y\n" comes back as "x\ny\n", and the scalar measured one line short
// for every line its content folded away. The renderer read the leftover as a
// blank line the author had left, and wrote one.
//
// Under ">+" that blank line is content on the way back in, so the value gained
// a trailing break on every render and the document never settled. Clip and
// strip chomping discard it, which kept the same miscount out of sight.
//
// linesSpannedBy now measures the source through Token.EndLine and adds the
// blank lines chomping keeps.
func TestFixedFoldedScalarGainsNoBreakWhenRendered(t *testing.T) {
	for name, src := range map[string]string{
		// The shape the generator found, and the three the same miscount
		// reaches once the chomping indicator stops hiding it.
		"keep, folded over a gap":  "- >+\n  x\n\n  y\n- 1\n",
		"clip, folded over a gap":  "- >\n  x\n\n  y\n- 1\n",
		"strip, folded over a gap": "- >-\n  x\n\n  y\n- 1\n",
		"two lines folded to one":  "- >+\n  x\n  y\n\n- 1\n",

		// A literal block never showed it: its value keeps the source's lines.
		"literal, over a gap": "- |+\n  x\n\n  y\n- 1\n",

		// The miscount grew with the number of blank lines kept, so a scalar
		// ending on two of them gained two.
		"keep, two trailing blanks": "- >+\n  x\n\n  y\n\n\n- 1\n",
		"keep, one trailing blank":  "- >+\n  x\n\n  y\n\n- 1\n",
		"folded twice":              "- >+\n  a\n\n  b\n\n  c\n- 1\n",

		// Under a mapping key, and with a third entry after it.
		"under a mapping key": "a: >+\n  x\n\n  y\nb: 1\n",
		"two entries after":   "- >+\n  x\n\n  y\n\n- 1\n- 2\n",

		// Neither of these ever diverged: with nothing after the scalar there
		// is no gap to measure, and with no break inside it nothing folds.
		"nothing after the scalar": "- >+\n  x\n\n  y\n",
		"no break inside":          "- >+\n  x\n- 1\n",
	} {
		t.Run(name, func(t *testing.T) {
			var before any
			require.NoError(t, yaml.Unmarshal([]byte(src), &before))

			file, err := parser.ParseBytes([]byte(src), parser.WithComments())
			require.NoError(t, err)

			rendered := file.String()
			assert.Equal(t, src, rendered, "the document is written back as it was read")

			var after any
			require.NoError(t, yaml.Unmarshal([]byte(rendered), &after))
			assert.Equal(t, before, after, "the value survives being written out")

			reread, err := parser.ParseBytes([]byte(rendered), parser.WithComments())
			require.NoError(t, err)
			assert.Equal(t, rendered, reread.String(), "and settles in one pass")
		})
	}
}

// TestFixedANonStringKeyNoLongerZeroesAWholeStruct: a key no field can be named
// after is skipped, and the entries around it read.
//
// ✅ Closed 2026-09-07 in two steps. 7dc4075 inverted decodeStruct, so the
// decode walks the document's entries and looks each field up rather than
// walking the fields and reading the mapping into a map first -- the map came
// back nil at the first key that was not a string, with no error, and every
// field kept its zero. entryName then named a key by its type's own canonical
// spelling, so a key a field can be named after reaches it: "true: a" writes a
// field tagged "true".
func TestFixedANonStringKeyNoLongerZeroesAWholeStruct(t *testing.T) {
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

// TestFixedATagTypesItsScalarWhateverItsSpelling: one tag gives one tree, in
// all three spellings.
//
// "!!float 7", "!<tag:yaml.org,2002:float> 7" and "!e!float 7" under a
// "%TAG !e! tag:yaml.org,2002:" line name the same tag, and each now puts an
// *ast.IntegerNode under its *ast.TagNode. Only the shorthand did: the scanner
// forced every other spelling to a string, because it matched the text the tag
// was written with against the reserved keywords instead of the URI it expands
// to -- and the URI is what only the parser knows, since a "%TAG" line can
// repoint a handle.
//
// Three values were lost by it rather than merely retyped. A string is read
// back against its tag afterwards, so "7", "0x1f" and "true" survived the
// detour; ".inf", "-.inf" and ".nan" decoded to the float64 zero and converted
// to the JSON "0.0", each reporting nothing.
func TestFixedATagTypesItsScalarWhateverItsSpelling(t *testing.T) {
	t.Run("the tree is the same in every spelling", func(t *testing.T) {
		for _, tc := range []struct {
			src  string
			node ast.Node
		}{
			{src: "!!float 7\n", node: &ast.IntegerNode{}},
			{src: "!!float 1e3\n", node: &ast.FloatNode{}},
			{src: "!!float .inf\n", node: &ast.InfinityNode{}},
			{src: "!<tag:yaml.org,2002:float> 7\n", node: &ast.IntegerNode{}},
			{src: "!<tag:yaml.org,2002:float> 1e3\n", node: &ast.FloatNode{}},
			{src: "!<tag:yaml.org,2002:float> .inf\n", node: &ast.InfinityNode{}},
			{src: "%TAG !e! tag:yaml.org,2002:\n---\n!e!float 1e3\n", node: &ast.FloatNode{}},
		} {
			file, err := parser.ParseBytes([]byte(tc.src), parser.WithComments())
			require.NoError(t, err, "%q", tc.src)

			tag, ok := file.Docs[len(file.Docs)-1].Body.(*ast.TagNode)
			require.True(t, ok, "%q: the body is not a tag node", tc.src)
			assert.Equal(t, "tag:yaml.org,2002:float", tag.URI, "%q", tc.src)
			assert.IsType(t, tc.node, tag.Value, "%q", tc.src)
		}
	})

	t.Run("the three specials keep their value in every spelling", func(t *testing.T) {
		for text, want := range map[string]float64{
			".inf":  math.Inf(1),
			"-.inf": math.Inf(-1),
			".nan":  math.NaN(),
		} {
			for _, src := range []string{
				"!!float " + text + "\n",
				"!<tag:yaml.org,2002:float> " + text + "\n",
			} {
				var got any
				require.NoError(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
				assert.InDelta(t, want, got, 0, "%q", src)

				_, err := codec.ToJSON([]byte(src))
				require.Error(t, err, "JSON has no infinity or NaN, so refusing is right: %q", src)
			}
		}
	})

	t.Run("a tag that resolves to nothing still leaves its scalar as text", func(t *testing.T) {
		for _, src := range []string{
			"!foo 12\n",
			"! 12\n",
			"!<x:y> 12\n",
			// "%TAG !!" repoints the secondary handle, so "!!int" names
			// !local-int here and resolves to nothing. This is the spelling the
			// scanner could never have judged.
			"%TAG !! !local-\n---\n!!int 12\n",
		} {
			var got any
			require.NoError(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equal(t, "12", got, "%q", src)
		}
	})
}

// TestFixedATaggedBlockMappingResolvesItsKeys: a tag on a block mapping types
// the keys inside it, as an untagged mapping does.
//
// The tag stands on the mapping; each key is a node of its own and resolves on
// its own, so "!foo" over "False: 1" is keyed by "false" -- the canonical
// spelling of the boolean -- and not by the text "False". The scanner used to
// force the token after a tag it did not recognize to a string, and the token
// after a tag that opens a block mapping is that mapping's first key.
func TestFixedATaggedBlockMappingResolvesItsKeys(t *testing.T) {
	for _, src := range []string{
		"!foo\nFalse: 1\n",
		"!\nFalse: 1\n",
		"!<tag:yaml.org,2002:map>\nFalse: 1\n",
		"!!map\nFalse: 1\n",
		"!foo {False: 1}\n",
		"False: 1\n",
	} {
		var got any
		require.NoError(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
		assert.Equal(t, map[string]any{"false": uint64(1)}, got, "%q", src)
	}
}

// TestFixedAKeyAfterALongTagOnAnEmptyValueResolves: an entry whose value is a
// tag with nothing after it no longer stops the next key from resolving.
//
// "a: !<tag:yaml.org,2002:null>" over "False: 1" is keyed by "false", as
// "a: !!null" over the same line always was. The scanner's rule reached past
// the tag's own node to whatever token came next, and where the tag stood alone
// at the end of a line that token was the following key.
func TestFixedAKeyAfterALongTagOnAnEmptyValueResolves(t *testing.T) {
	for _, src := range []string{
		"a: !<tag:yaml.org,2002:null>\nFalse: 1\n",
		"a: !!null\nFalse: 1\n",
		"%TAG !e! tag:yaml.org,2002:\n---\na: !e!null\nFalse: 1\n",
	} {
		var got any
		require.NoError(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
		assert.Equal(t, map[string]any{"a": nil, "false": uint64(1)}, got, "%q", src)
	}
}

// TestFixedAPropertyAloneAfterAColonNamesTheEmptyNode: a tag or an anchor
// written with nothing after it stands on the empty node, and the entries below
// it stay where the document put them.
//
// 8.2.2 needs a nested block mapping indented further than the key it belongs
// to, and 8.2.1 the same for a sequence, so a token back at the entry's own
// column opens the next entry rather than continuing this one. The parser said
// that for a bare anchor after "k:" and for the tags the core schema resolves,
// and nowhere else. Two shapes went the other way and restructured the
// document without reporting anything:
//
//   - "a: !foo" over "b: 1" over "c: 2" came back as {"a": {"b": 1, "c": 2}},
//     and "- !foo" over "- 1" as [[1]]. "!!null" and "!!str" on the same empty
//     value read flat, so it was the tags naming no known type.
//   - "? a" over ": &a1" over "? b" over ": &a2" came back as
//     {"a": {"b": nil}}. The short form "a: &a1" over "b: &a2" read flat, so it
//     was the anchor and the explicit key together: an explicit key's value is
//     read through parseMapKeyValue, which recorded no entry column for the
//     property to measure itself against.
func TestFixedAPropertyAloneAfterAColonNamesTheEmptyNode(t *testing.T) {
	t.Run("the entries below stay flat", func(t *testing.T) {
		for src, want := range map[string]any{
			"a: !foo\nb: 1\nc: 2\n":    map[string]any{"a": nil, "b": uint64(1), "c": uint64(2)},
			"a: !\nb: 1\n":             map[string]any{"a": nil, "b": uint64(1)},
			"- !foo\n- b\n":            []any{nil, "b"},
			"? a\n: &a1\n? b\n: &a2\n": map[string]any{"a": nil, "b": nil},
			"? a\n: !foo\n? b\n: 1\n":  map[string]any{"a": nil, "b": uint64(1)},
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equal(t, want, got, "%q", src)
			assert.Equal(t, src, renderOnce(t, src), "%q: the nesting is written back out", src)
		}
	})

	t.Run("an alias to the anchor reads it", func(t *testing.T) {
		// It used to report `alias "a1" names an anchor that is not resolved
		// yet`, because the anchor was inside the node still being built.
		var got any
		require.NoError(t, yaml.Unmarshal([]byte("? a\n: &a1\n? b\n: *a1\n"), &got))
		assert.Equal(t, map[string]any{"a": nil, "b": nil}, got)
	})

	t.Run("and a value written further in is still the property's node", func(t *testing.T) {
		for src, want := range map[string]any{
			// Indented past the key, so it is nested and always was.
			"a: !foo\n  b: 1\n":    map[string]any{"a": map[string]any{"b": uint64(1)}},
			"? a\n: &a1\n  b: 1\n": map[string]any{"a": map[string]any{"b": uint64(1)}},
			"- !foo\n  - 1\n":      []any{[]any{uint64(1)}},
			// 8.2.1 lets a block sequence stand at its key's own column, so
			// this is the value and not the next entry.
			"a: !foo\n- 1\n": map[string]any{"a": []any{uint64(1)}},
			"k: &a\n- 1\n":   map[string]any{"k": []any{uint64(1)}},
			// Written beside the property, so it is what the property names.
			"? a\n: &a1 x\n? b\n: *a1\n": map[string]any{"a": "x", "b": "x"},
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equal(t, want, got, "%q", src)
		}
	})
}

// TestFixedAScalarUnderAnUnresolvedTagIsRead: two documents the generator found
// refused, both a scalar standing under a tag that names no type this library
// reads.
//
// Such a scalar keeps the text it was written with, and parseTagValue built
// that string without stepping past the token it had just read. The next reader
// found a token where the entry had already ended and reported "value is not
// allowed in this context". The two shapes look unrelated and are one fault: a
// tag written on its own line with a comment under it, and an unknown secondary
// tag on a root scalar under a "%YAML" directive.
func TestFixedAScalarUnderAnUnresolvedTagIsRead(t *testing.T) {
	for src, want := range map[string]any{
		"a:\n !\n # c\n 1\n":             map[string]any{"a": "1"},
		"a:\n !\n 1\n":                   map[string]any{"a": "1"},
		"%YAML 1.1\n---\n!!nulll Null\n": "Null",
		"!!nulll Null\n":                 "Null",
	} {
		var got any
		require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
		assert.Equal(t, want, got, "%q", src)
	}
}

// TestFixedAnIntTagReadsIntoAGoInteger: "!!int" on a value reads into a Go
// integer, as the same value untagged always did.
//
// [yamlgen.Tagged.Decoded] says where the two parted company: an untagged
// non-negative integer comes back as a uint64 and a negative one as an int64,
// where "!!int" hands back a plain int for a number that fits one -- which is
// what strconv.Atoi gave. Decoder.decodeValue read a uint64, an int64, a
// float64 and a string into an integer field and had no case for an int, so it
// reported `cannot unmarshal int into Go struct field box.N of type int64`.
func TestFixedAnIntTagReadsIntoAGoInteger(t *testing.T) {
	type box struct {
		N  int64   `yaml:"n"`
		I  int     `yaml:"i"`
		I8 int8    `yaml:"i8"`
		U  uint64  `yaml:"u"`
		F  float64 `yaml:"f"`
		A  any     `yaml:"a"`
	}

	t.Run("into an integer field, whatever its width", func(t *testing.T) {
		var got box
		require.NoError(t, yaml.Unmarshal([]byte("n: !!int 5\ni: !!int 6\ni8: !!int 7\nu: !!int 8\n"), &got))
		assert.Equal(t, box{N: 5, I: 6, I8: 7, U: 8}, got)

		var negative box
		require.NoError(t, yaml.Unmarshal([]byte("n: !<tag:yaml.org,2002:int> -5\n"), &negative))
		assert.Equal(t, box{N: -5}, negative)
	})

	t.Run("into a slice and into a typed map", func(t *testing.T) {
		var items []int64
		require.NoError(t, yaml.Unmarshal([]byte("- !!int 5\n"), &items))
		assert.Equal(t, []int64{5}, items)

		var byName map[string]int64
		require.NoError(t, yaml.Unmarshal([]byte("n: !!int 5\n"), &byName))
		assert.Equal(t, map[string]int64{"n": 5}, byName)
	})

	t.Run("and a number too wide for the field still overflows", func(t *testing.T) {
		// The same complaint the untagged number draws, which is the point:
		// the tag changes what the node is and not how wide the field is.
		for _, src := range []string{"i8: !!int 300\n", "i8: 300\n", "u: !!int -5\n"} {
			var got box
			err := yaml.Unmarshal([]byte(src), &got)
			require.Errorf(t, err, "%q", src)
			assert.Contains(t, err.Error(), "overflow", "%q", src)
		}
	})
}

// TestFixedAFloatTagOnAWideNumberKeepsItsWidth: "!!float" on a number no
// float64 holds reads as a big.Float, as the same number untagged always did.
//
// It failed two ways. Large, the parse stopped: ast.readsAsFloat took
// strconv.ParseFloat's ErrRange for "not a float", so "!!float 1e+310" was a
// tag naming a type its scalar is not and the document was refused. Small, it
// was worse than refused: "!!float 1e-400" came back as the float64 zero with
// nothing reported, because castToFloatValue narrowed the big.Float that the
// node held.
//
// codec.ToJSON lost the same numbers, writing "0.0" for both, and now writes
// them out in full.
func TestFixedAFloatTagOnAWideNumberKeepsItsWidth(t *testing.T) {
	t.Run("the decoder reads a big.Float, tagged or not", func(t *testing.T) {
		for _, src := range []string{
			"a: !!float 1e+310\n", "a: 1e+310\n",
			"a: !!float 1e-400\n", "a: 1e-400\n",
			"a: !!float -1e+310\n",
			"a: !<tag:yaml.org,2002:float> 1e+310\n",
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.IsType(t, new(big.Float), got.(map[string]any)["a"], "%q", src)
		}
	})

	t.Run("ToJSON writes the number, tagged or not", func(t *testing.T) {
		for _, tc := range []struct{ src, want string }{
			{src: "!!float 1e+310\n", want: "1e+310"},
			{src: "1e+310\n", want: "1e+310"},
			{src: "!!float 1e-400\n", want: "1e-400"},
			{src: "1e-400\n", want: "1e-400"},
			{src: "!!float -1e+310\n", want: "-1e+310"},
			// A float the machine word does hold still writes its fraction.
			{src: "!!float 1.5\n", want: "1.5"},
			{src: "!!float 7\n", want: "7.0"},
		} {
			out, err := codec.ToJSON([]byte(tc.src))
			require.NoErrorf(t, err, "%q", tc.src)
			assert.Equal(t, tc.want, string(out), "%q", tc.src)
		}
	})

	t.Run("and an int tag on a wide integer still reads as a big.Int", func(t *testing.T) {
		var got any
		require.NoError(t, yaml.Unmarshal([]byte("a: !!int 123456789012345678901\n"), &got))
		assert.IsType(t, new(big.Int), got.(map[string]any)["a"])
	})
}

// TestFixedAnAnchorBetweenATagAndItsScalarKeepsTheText: an anchor written
// between a tag and the scalar it types no longer changes the type.
//
// A tag that resolves to nothing leaves its scalar as the text it was written
// with, so "!foo true" is the string "true". parseTagValue applied that to the
// tag's own next token and an anchor may stand there, so "!foo &a1 true" read
// the boolean where "&a1 !foo true" -- the same two properties the other way
// round -- read the string. Only the tags naming no known type did it: "!!str"
// in the same place was unaffected.
func TestFixedAnAnchorBetweenATagAndItsScalarKeepsTheText(t *testing.T) {
	t.Run("the order of the two properties no longer decides", func(t *testing.T) {
		for _, src := range []string{
			"!foo true\n", "!foo &a1 true\n", "&a1 !foo true\n",
			"!<!foo> &a1 true\n", "! &a1 true\n", "!!str &a1 true\n",
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equal(t, "true", got, "%q", src)
		}
	})

	t.Run("in a mapping, a sequence, a flow collection and through an alias", func(t *testing.T) {
		for src, want := range map[string]any{
			"a: !foo &a1 12\n":         map[string]any{"a": "12"},
			"- !foo &a1 12\n":          []any{"12"},
			"{a: !foo &a1 12}\n":       map[string]any{"a": "12"},
			"a: !foo &a1 12\nb: *a1\n": map[string]any{"a": "12", "b": "12"},
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equal(t, want, got, "%q", src)
		}
	})

	t.Run("and a tag that does resolve still types its scalar", func(t *testing.T) {
		for src, want := range map[string]any{
			"!!int &a1 12\n":     12, // "!!int" hands back a plain int for a number that fits one.
			"!!float &a1 12\n":   float64(12),
			"!!seq &a1 [1, 2]\n": []any{uint64(1), uint64(2)},
			"!foo &a1 [1, 2]\n":  []any{uint64(1), uint64(2)},
			"!foo &a1 |\n  x\n":  "x\n",
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equal(t, want, got, "%q", src)
		}
	})
}

// TestFixedAQuotedExplicitKeyTakesABlockScalarValue: `? "a"` over `: >-` reads,
// as `? a` over the same two lines always did.
//
// The scanner measures the lines of a value against Scanner.lastDelimColumn,
// and scanMapValue picked the wrong column for it. A key already cut into
// tokens -- a quoted one, or an empty scalar carrying an anchor or a tag --
// sets the level from the key's own start, which is right while the key and its
// ":" stand on one line. Written the long way they do not: "? \"a\"" puts the
// quote in column 3, the ":" is in column 1 on the next line, and the block
// scalar's content in column 3 then read as level with its own delimiter --
// the end of the scalar rather than its first line. The header was cut short,
// an empty string went in as the value, and the content was left over for the
// document to complain about.
//
// The ":" is where the entry sits when the key was written above it, so that
// test goes first now and the key's own column is asked only for a key on the
// same line.
func TestFixedAQuotedExplicitKeyTakesABlockScalarValue(t *testing.T) {
	t.Run("the shapes that were refused", func(t *testing.T) {
		for _, src := range []string{
			"? \"a\"\n: >-\n  x\n",
			"? 'a'\n: >-\n  x\n",
			"? \"a\"\n: |\n  x\n",
			"a:\n  ? \"b\"\n  : >-\n    x\n",
			"- ? \"a\"\n  : >-\n    x\n",
			"? \"a\"\n: &an >-\n  x\n",
			"? \"a\"\n: !!str >-\n  x\n",
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
		}
	})

	t.Run("the value is the block scalar and not an empty string", func(t *testing.T) {
		for src, want := range map[string]any{
			"? \"a\"\n: >-\n  x\n":               map[string]any{"a": "x"},
			"? 'a'\n: >-\n  x\n":                 map[string]any{"a": "x"},
			"? \"a\"\n: |\n  x\n":                map[string]any{"a": "x\n"},
			"? a\n: >-\n  x\n":                   map[string]any{"a": "x"},
			"\"a\": >-\n  x\n":                   map[string]any{"a": "x"},
			"? \"a\"\n: >-\n  x\n? \"b\"\n: 1\n": map[string]any{"a": "x", "b": uint64(1)},
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equal(t, want, got, "%q", src)
		}
	})

	t.Run("a key on the ':' line still measures from the key", func(t *testing.T) {
		// The branch the fix moved: "&a :" cuts the key into tokens and the
		// value's lines are measured from the '&', not from the name after it.
		for src, want := range map[string]any{
			"&a :\n":              map[string]any{"null": nil},
			"\"a\": >-\n  x\n":    map[string]any{"a": "x"},
			"a: >-\n  x\n":        map[string]any{"a": "x"},
			"? \"a\"\n: [1]\n":    map[string]any{"a": []any{uint64(1)}},
			"? &a x\n: >-\n  y\n": map[string]any{"x": "y"},
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equal(t, want, got, "%q", src)
		}
	})
}

// TestFixedACommentOnAnExplicitKeysColonLineIsKept: a comment on the ":" line
// of the long form, with the value below it, survives.
//
// newMappingValueNode returned early for every explicit key, on the reading
// that a comment on the token it was handed was the key's own and attached
// there already. That holds where parseMapKeyValue hands the key's own last
// token over, since an explicit key written in one group ends on the key rather
// than on a ":". It does not hold where the ":" is a token of its own: the
// comment then stands on the ":" line and belongs to the value, and returning
// early dropped it with nothing reported.
//
// It goes on the value now, which is where the short form puts the same
// comment -- "a: # c3" over "  v" renders as "a: v # c3" -- so the two
// spellings normalize the same way.
func TestFixedACommentOnAnExplicitKeysColonLineIsKept(t *testing.T) {
	for src, renders := range map[string]string{
		"? a\n: # c3\n  v\n":   "? a\n: v # c3\n",
		"? a\n: # c3\n  - 1\n": "? a\n:\n# c3\n- 1\n",
		"?\n: #c1\n":           "?\n: #c1\n",
		// Every other position kept it before and still does.
		"a: # c3\n  v\n":   "a: v # c3\n",
		"a: # c3\n  - 1\n": "a: # c3\n- 1\n",
		"? a\n: v # c3\n":  "? a\n: v # c3\n",
		"? a # c3\n: v\n":  "? a # c3\n: v\n",
	} {
		once := renderOnce(t, src)
		assert.Equal(t, renders, once, "%q", src)

		assert.Equal(t, once, renderOnce(t, once), "%q: the rendering settles", src)
	}

	t.Run("the value is unchanged", func(t *testing.T) {
		for src, want := range map[string]any{
			"? a\n: # c3\n  v\n":   map[string]any{"a": "v"},
			"? a\n: # c3\n  - 1\n": map[string]any{"a": []any{uint64(1)}},
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equal(t, want, got, "%q", src)
		}
	})
}

// TestFixedALocalTagBeforeAnAnchorTypesItsScalar: a local or non-specific tag
// keeps its scalar as text whichever side of the anchor it is written on.
//
// ✅ Closed 2026-09-13 by the parser reaching the scalar an anchor group names
// and retyping the token before the node is built. It used to depend on the
// order: "!foo &a1 true" read the boolean where "!foo true" and
// "&a1 !foo true" read the string.
//
// The whole matrix is held rather than the one document that failed, because a
// fix that traded one spelling for another would otherwise look like a fix.
func TestFixedALocalTagBeforeAnAnchorTypesItsScalar(t *testing.T) {
	for _, src := range []string{
		// The four spellings, with the anchor after the tag.
		"!foo &a1 true\n",
		"! &a1 true\n",
		"!<!foo> &a1 true\n",
		// The anchor first, which always read.
		"&a1 !foo true\n",
		// No anchor at all.
		"!foo true\n",
		"! true\n",
		// A secondary tag, which was never affected.
		"!!str &a1 true\n",
	} {
		var got any
		require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
		assert.Equalf(t, "true", got, "%q", src)
	}

	t.Run("in every context", func(t *testing.T) {
		for src, want := range map[string]any{
			"a: !foo &a1 true\n":   map[string]any{"a": "true"},
			"- !foo &a1 true\n":    []any{"true"},
			"{a: !foo &a1 true}\n": map[string]any{"a": "true"},
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equalf(t, want, got, "%q", src)
		}
	})
}

// TestFixedABinaryTagReadsIntoAGoByteSlice: `!!binary` reads into the Go type
// the tag names.
//
// ✅ Closed 2026-09-13 by decodeSlice asking binaryBytes first. It used to be
// refused with "string was used where sequence is expected", while an `any`
// gave []uint8 and a string field gave the decoded bytes -- the one Go type the
// tag names was the one it could not reach.
func TestFixedABinaryTagReadsIntoAGoByteSlice(t *testing.T) {
	const src = "a: !!binary aGVsbG8=\n"

	t.Run("into a field, a typed map and a slice element", func(t *testing.T) {
		var into struct {
			A []byte `yaml:"a"`
		}
		require.NoError(t, yaml.Unmarshal([]byte(src), &into))
		assert.Equal(t, []byte("hello"), into.A)

		var byName map[string][]byte
		require.NoError(t, yaml.Unmarshal([]byte(src), &byName))
		assert.Equal(t, map[string][]byte{"a": []byte("hello")}, byName)

		var items [][]byte
		require.NoError(t, yaml.Unmarshal([]byte("- !!binary aGVsbG8=\n"), &items))
		assert.Equal(t, [][]byte{[]byte("hello")}, items)
	})

	t.Run("and the destinations that always worked still do", func(t *testing.T) {
		var loose any
		require.NoError(t, yaml.Unmarshal([]byte(src), &loose))
		assert.Equal(t, map[string]any{"a": []byte("hello")}, loose)

		var timed struct {
			T time.Time `yaml:"t"`
		}
		require.NoError(t, yaml.Unmarshal([]byte("t: !!timestamp 2001-12-14\n"), &timed))
		assert.Equal(t, time.Date(2001, time.December, 14, 0, 0, 0, 0, time.UTC), timed.T)
	})
}

// TestFixedAVersionDirectiveLeavesTheRootBlockScalarAlone: a "%YAML" line over
// a document whose body is a block scalar reads it, whatever the content
// spells.
//
// A block scalar is a string under every schema, so there was nothing there to
// resolve. Parser.retypeAhead reads the plain scalars the scan had already cut
// past the directive and types them again against the version it names, and a
// block scalar's content is cut as a plain String -- the one string a schema
// must not touch. "%YAML 1.1" over "---" over ">-" over " null" had the content
// retyped as a null, and parseLiteral then refused the document with
// "unexpected token. required string token".
//
// The tape is not grouped when retypeAhead runs, so the header and its content
// are still two tokens and the content is whatever follows the header -- which
// is how stageBlockScalars reads it too.
func TestFixedAVersionDirectiveLeavesTheRootBlockScalarAlone(t *testing.T) {
	t.Run("every content the schema would have resolved", func(t *testing.T) {
		for _, text := range []string{"null", "~", "True", "yes", "5", "1.5", "0100", "x", "x y", "null x"} {
			for _, header := range []string{">-", "|-"} {
				src := "%YAML 1.1\n---\n" + header + "\n " + text + "\n"
				wellFormed(t, src)

				var got any
				require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
				assert.Equal(t, text, got, "%q", src)
			}
		}
	})

	t.Run("under either version, and with the body nested", func(t *testing.T) {
		for src, want := range map[string]any{
			"%YAML 1.2\n---\n>-\n null\n":                   "null",
			">-\n null\n":                                   "null",
			"---\n>-\n null\n":                              "null",
			"%TAG !e! tag:yaml.org,2002:\n---\n>-\n null\n": "null",
			"%YAML 1.1\n---\nk: >-\n  null\n":               map[string]any{"k": "null"},
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equal(t, want, got, "%q", src)
		}
	})

	t.Run("and the schema still reaches every plain scalar after the directive", func(t *testing.T) {
		// The retyping is what makes a directive work at all, so the fix must
		// not stop it: 1.1 reads "0100" as 64 where 1.2 reads 100.
		for src, want := range map[string]any{
			"%YAML 1.1\n---\n0100\n":    uint64(64),
			"%YAML 1.2\n---\n0100\n":    uint64(100),
			"%YAML 1.1\n---\nyes\n":     true,
			"%YAML 1.2\n---\nyes\n":     "yes",
			"%YAML 1.1\n---\nk: 0100\n": map[string]any{"k": uint64(64)},
			// A plain scalar after a block scalar still resolves, and after two.
			"%YAML 1.1\n---\na: >-\n  null\nb: 0100\n": map[string]any{"a": "null", "b": uint64(64)},
			"%YAML 1.1\n---\na: >-\n  null\nb: |-\n  yes\nc: 0100\n": map[string]any{
				"a": "null", "b": "yes", "c": uint64(64),
			},
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equal(t, want, got, "%q", src)
		}
	})
}

// TestFixedATagOnItsOwnLineTakesTheBlockScalarUnderIt: a tag written on a line
// of its own, over a block scalar, reads.
//
// 6.9.1 and 8.1 allow both: a node's properties may stand on a line of their
// own, and the node under them may be a block scalar. "!!null" over ">" was
// refused with "value is not allowed in this context", where "!!null >" on one
// line read and so did "!foo" over ">".
//
// The tag was what decided it, because of where the grouping puts the two. A
// tag is joined only to what stands on its own line, so "!!null >" arrives at
// parseTagValue as one scalar-tag group and "!!null" over ">" as a tag and a
// folded group. The branch that reads the second returned the literal without
// stepping past it, and the document then held a token nothing had read --
// the same fault as the scalar under an unresolved tag, one branch over.
func TestFixedATagOnItsOwnLineTakesTheBlockScalarUnderIt(t *testing.T) {
	t.Run("the shapes that were refused", func(t *testing.T) {
		for src, want := range map[string]any{
			"!!null\n>\n":           nil,
			"!!str\n>-\n x\n":       "x",
			"!!int\n>-\n 5\n":       5,
			"!!bool\n>-\n true\n":   true,
			"!!null\n|\n":           nil,
			"k: !!null\n  >\n":      map[string]any{"k": nil},
			"- !!str\n  >-\n   x\n": []any{"x"},
			// The version directive reaches the scalar under the tag as it
			// reaches any other, and a block scalar is a string either way.
			"%YAML 1.1\n---\n!!str\n>-\n null\n": "null",
		} {
			wellFormed(t, src)

			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equal(t, want, got, "%q", src)
		}
	})

	t.Run("the spellings that read before still read", func(t *testing.T) {
		for src, want := range map[string]any{
			"!!null >\n":     nil,
			"!foo\n>\n":      "",
			"&a\n>\n":        "",
			"!!seq\n>\n":     "",
			"!!str >-\n x\n": "x",
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equal(t, want, got, "%q", src)
		}
	})

	t.Run("and what the tag cannot hold is still refused, at the tag", func(t *testing.T) {
		// A value the tag names a type for and cannot read, and a node of the
		// wrong kind entirely. Both parse; the refusal is resolution's.
		for src, says := range map[string]string{
			"!!null\n>-\n x\n": `cannot read "x" as !!null`,
			"!!null\n[1]\n":    "!!null names a kind this node is not",
		} {
			_, perr := parser.ParseBytes([]byte(src), parser.WithComments())
			require.NoErrorf(t, perr, "%q parses", src)

			var got any
			err := yaml.Unmarshal([]byte(src), &got)
			require.Errorf(t, err, "%q", src)
			assert.Contains(t, err.Error(), says, "%q", src)
		}
	})
}

// TestFixedAMappingKeyWrittenEmptyIsRead: an empty key reads in every position,
// and what precedes it no longer decides.
//
// Three shapes were refused with two messages, both naming the line above the
// key: "a:" over ": 2" reported `unexpected scalar value`, and "k: &a1" over
// ": 1" and "false: !!bool false" over ": &a1 !!null" both reported
// `mapping value is not allowed in this context`.
//
// keyWindow.hasNoKey took any candidate the grouping had already made
// something of as the key of the ':' that followed, whatever line it stood on.
// So the first entry's own key group, or an anchored value, became the key of
// the ':' below it. An implicit key stands on the line its ':' does -- 7.4.2 --
// so the line test applies to a grouped candidate too. An explicit key is the
// exception the rule needs: "? a" over ": 2" writes the two on separate lines
// by design, and it is told apart by opening with a "?".
func TestFixedAMappingKeyWrittenEmptyIsRead(t *testing.T) {
	t.Run("every position", func(t *testing.T) {
		for src, want := range map[string]any{
			// The three that were refused.
			"a:\n: 2\n":                           map[string]any{"a": nil, "null": uint64(2)},
			"k: &a1\n: 1\n":                       map[string]any{"k": nil, "null": uint64(1)},
			"false: !!bool false\n: &a1 !!null\n": map[string]any{"false": false, "null": nil},
			// The three that always read.
			": a\n":           map[string]any{"null": "a"},
			"a: 1\n: 2\n":     map[string]any{"a": uint64(1), "null": uint64(2)},
			"- k: 1\n  : 2\n": []any{map[string]any{"k": uint64(1), "null": uint64(2)}},
			// An anchored value with content, and nested.
			"k: &a1 x\n: 1\n":   map[string]any{"k": "x", "null": uint64(1)},
			"a:\n: 2\nb: 3\n":   map[string]any{"a": nil, "null": uint64(2), "b": uint64(3)},
			"x:\n  a:\n  : 2\n": map[string]any{"x": map[string]any{"a": nil, "null": uint64(2)}},
			"k: |\n  x\n: 1\n":  map[string]any{"k": "x\n", "null": uint64(1)},
		} {
			wellFormed(t, src)

			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equal(t, want, got, "%q", src)
		}
	})

	t.Run("an explicit key still keys the ':' below it", func(t *testing.T) {
		for src, want := range map[string]any{
			"? a\n: 2\n":           map[string]any{"a": uint64(2)},
			"? a\n: 2\n? b\n: 3\n": map[string]any{"a": uint64(2), "b": uint64(3)},
			"? [a]\n: 1\n":         map[string]any{"[a]": uint64(1)},
			// A key on the ':' line is still the key, grouped or not.
			"!!str foo: 1\n": map[string]any{"foo": uint64(1)},
			"&a1 x: 1\n":     map[string]any{"x": uint64(1)},
			// Inside a flow collection a ':' may stand on its own line.
			"{a: 1, : 2}\n": map[string]any{"a": uint64(1), "null": uint64(2)},
			"{: 1}\n":       map[string]any{"null": uint64(1)},
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equal(t, want, got, "%q", src)
		}
	})

	t.Run("and a node above a ':' is no longer taken as its key", func(t *testing.T) {
		// The laxity that went with it. grammar.NewRecognizer, the reference
		// parser, libfyaml 1.0.0b1 and go.yaml.in/yaml/v3 v3.0.5 all refuse
		// these; this library read them.
		for _, src := range []string{"a Null\n: 1\n", "!x Null\n: 1\n", "!!str Null\n: 1\n"} {
			var got any
			assert.Errorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
		}
	})
}

// TestFixedABlockScalarInASequenceKeepsAnEmptyKeyApart: the same fix, seen
// through the renderer.
//
// "a:" over " - |1-" over "   " over ":" rendered to "a:" over "- |2-    :",
// which reported `invalid header option` on the way back in -- a valid document
// rendering to one that is not YAML. The empty key was being read as part of
// the block scalar's entry, so the renderer wrote the two onto one line.
func TestFixedABlockScalarInASequenceKeepsAnEmptyKeyApart(t *testing.T) {
	for src, renders := range map[string]string{
		"a:\n - &a1 |1-\n   \n:\n":    "a:\n- &a1 |2-\n   \n:\n",
		"a:\n - |1-\n   \n:\n":        "a:\n- |2-\n   \n:\n",
		"a:\n - &a1 |1-\n   x\n:\n":   "a:\n- &a1 |2-\n   x\n:\n",
		"a:\n - &a1 |1\n   \n:\n":     "a:\n- &a1 |2\n   \n:\n",
		"a:\n - &a1 |1-\n   \nb: 1\n": "a:\n- &a1 |2-\n   \nb: 1\n",
	} {
		wellFormed(t, src)

		once := renderOnce(t, src)
		assert.Equal(t, renders, once, "%q", src)

		reread, err := parser.ParseBytes([]byte(once), parser.WithComments())
		require.NoErrorf(t, err, "%q: the rendering must parse", src)
		assert.Equal(t, once, reread.String(), "%q: and settle", src)
	}
}
