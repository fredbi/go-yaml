// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"strings"
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
