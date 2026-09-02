package refparser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/refparser"
)

// TestRejectsMalformedDocuments covers the constructs YAML forbids that the
// parser used to take.
//
// Accepting too much is the worse direction to be wrong in: a document another
// implementation refuses passes through here unremarked, and is handed on as if
// it were sound. Each case below is paired with the nearest well-formed
// document, so that a fix which over-corrects fails too.
func TestRejectsMalformedDocuments(t *testing.T) {
	tests := map[string]struct {
		invalid string
		valid   string
	}{
		// A comment starts a line or follows a space. Pressed up against what
		// came before, there is nothing to separate it from.
		"comment after a quoted scalar": {
			invalid: "key: \"value\"# comment\n",
			valid:   "key: \"value\" # comment\n",
		},
		"comment after a comma": {
			invalid: "[a, b,#comment\n]\n",
			valid:   "[a, b, #comment\n]\n",
		},
		"comment after a closing bracket": {
			invalid: "[a, b]#comment\n",
			valid:   "[a, b] #comment\n",
		},
		// '#' cannot begin a scalar, though it is ordinary inside one.
		"scalar beginning with a hash": {
			invalid: "key: [#a]\n",
			valid:   "key: [a#b]\n",
		},

		// A flow collection has no sequence entries, and '-' alone is not a
		// scalar there.
		"dash as a flow sequence entry": {
			invalid: "[-]\n",
			valid:   "[-a]\n",
		},
		"dashes as flow sequence entries": {
			invalid: "- [-, -]\n",
			valid:   "- [-a, -b]\n",
		},

		// A tag shorthand cannot hold a ',': it has to be percent-encoded. A
		// verbatim tag holds a URI and takes it as written.
		"comma in a tag shorthand": {
			invalid: "- !!str, xxx\n",
			valid:   "- !<tag:yaml.org,2002:str> xxx\n",
		},

		// A TAG directive defines a handle for the one document that follows.
		"tag handle used past its document": {
			invalid: "%TAG !p! tag:example.com,2011:\n--- !p!A\na: b\n--- !p!B\nc: d\n",
			valid:   "%TAG !p! tag:example.com,2011:\n--- !p!A\na: b\n",
		},

		// A construct carrying on to the next line is marked as one value by
		// being indented under what introduced it.
		"flow collection not indented past its key": {
			invalid: "flow: [a,\nb]\n",
			valid:   "flow: [a,\n b]\n",
		},
		"flow collection continued after a tab": {
			invalid: "- [\n\tfoo,\n foo\n ]\n",
			valid:   "- [\n foo,\n foo\n ]\n",
		},
		"quoted scalar not indented past its key": {
			invalid: "quoted: \"a\nb\nc\"\n",
			valid:   "quoted: \"a\n b\n c\"\n",
		},
		// The same rule where the key was never written. The ':' is where the
		// entry begins, so a token level with it opens the next entry, and "1"
		// opens nothing. This was the one shape of it that got through: the
		// key written out, "k:\n1\n", and the empty key carrying a property,
		// ": &a\n1\n", were both refused already.
		"value level with an empty key": {
			invalid: ":\n1\n",
			valid:   ":\n  1\n",
		},

		// ns-anchor-name is one character or more, and it ends at a flow
		// indicator. A collection opening straight onto it is a node with no
		// separation in front of it.
		"anchor with no name": {
			invalid: "& e\n",
			valid:   "&a e\n",
		},
		"flow sequence touching the anchor in front of it": {
			invalid: "&a[]\n",
			valid:   "&a []\n",
		},
		"flow mapping touching the anchor in front of it": {
			invalid: "[&a{1: 2}]\n",
			valid:   "[&a {1: 2}]\n",
		},

		// A directive opens a line and nothing else does, so a '%' anywhere
		// else is not one -- and it cannot open a plain scalar either.
		"percent sign as a value": {
			invalid: " k: %\n",
			valid:   " k: a%b\n",
		},
		"percent sign as a sequence entry": {
			invalid: " - %\n",
			valid:   " - 100%\n",
		},
		"percent sign opening a block value": {
			invalid: "k:\n  %\n",
			valid:   "%YAML 1.2\n---\nk: 1\n",
		},

		// A plain scalar cannot open on an indicator. Inside one they are
		// ordinary characters, and inside a flow collection they are claimed
		// before a scalar could begin -- so an anchor on an empty node keeps
		// working there.
		"closing brace as a whole document": {
			invalid: "}\n",
			valid:   "{}\n",
		},
		"comma as a whole document": {
			invalid: ",\n",
			valid:   "a,b\n",
		},
		"comma after an anchor outside a flow collection": {
			invalid: "&a,\n",
			valid:   "[&a, b]\n",
		},
		"closing brace after an anchor outside a flow mapping": {
			invalid: "&a}\n",
			valid:   "{&a: b}\n",
		},
		"closing bracket after an anchor outside a flow sequence": {
			invalid: "&a]\n",
			valid:   "[&a]\n",
		},

		// A block scalar header takes one indentation indicator and one
		// chomping indicator, in either order, and either may be left out. Two
		// of either is not a header.
		"two chomping indicators": {
			invalid: "|--\n",
			valid:   "|-\n",
		},
		"two chomping indicators, folded": {
			invalid: ">++\n",
			valid:   ">+\n",
		},
		"two indentation indicators": {
			invalid: "|12\n  a\n",
			valid:   "|1\n  a\n",
		},
		// The comment after a header is separated from it, like every other
		// comment. Pressed up against the indicators it starts nothing.
		"comment touching a block scalar header": {
			invalid: "|-#\n",
			valid:   "|- #\n",
		},

		// A numeric escape takes hexadecimal digits, and how many is fixed by
		// which escape it is. Only the count used to be checked, so an escape
		// with the wrong characters in it decoded to some other character
		// rather than being refused -- the one kind of laxity nothing
		// downstream is in a position to notice.
		"escaped 8-bit character with no hex digits": {
			invalid: `"\xZZ"` + "\n",
			valid:   `"\x41"` + "\n",
		},
		"escaped UTF-16 character shifted by a letter": {
			invalid: `"\uu0BA"` + "\n",
			valid:   `"º"` + "\n",
		},
		"escaped UTF-32 character with no hex digits": {
			invalid: `"\U0000004G"` + "\n",
			valid:   `"\U00000041"` + "\n",
		},
		"low surrogate with no hex digits": {
			invalid: `"\uD83D\uDEZZ"` + "\n",
			valid:   `"😀"` + "\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := refparser.ParseBytes([]byte(test.invalid), refparser.ParseComments)
			assert.Errorf(t, err, "accepted %q", test.invalid)

			_, err = refparser.ParseBytes([]byte(test.valid), refparser.ParseComments)
			require.NoErrorf(t, err, "rejected %q", test.valid)
		})
	}
}

// TestAcceptsDocumentsAtTheRoot covers the exemption the indentation rules
// need: a construct that is the document itself has nothing around it, so its
// further lines have nothing to be indented past.
func TestAcceptsDocumentsAtTheRoot(t *testing.T) {
	sources := map[string]string{
		"flow mapping at the root":   "{\n? explicit: entry,\nimplicit: entry,\n}\n",
		"flow sequence at the root":  "[\na,\nb,\n]\n",
		"nested flow at the root":    "{ key: [[[\n  value\n ]]]\n}\n",
		"quoted scalar at the root":  "\"a\nb\nc\"\n",
		"blank line inside a scalar": "key: \"a\n\n b\"\n",
	}

	for name, source := range sources {
		t.Run(name, func(t *testing.T) {
			_, err := refparser.ParseBytes([]byte(source), refparser.ParseComments)
			assert.NoErrorf(t, err, "rejected %q", source)
		})
	}
}

// TestParseRefusesWhatIsNotAStream covers the source itself rather than what it
// says.
//
// c-printable is the set of characters a YAML stream may hold, so the control
// characters below x20 other than tab, line feed and carriage return are not
// YAML however they arrive. A stream is Unicode too, and a byte belonging to no
// character is not one: it used to be turned into U+FFFD on the way in, so the
// byte was gone and nothing had said so.
//
// Escapes are unaffected. They are the mechanism the spec provides for writing
// these characters down, and what they produce is a value rather than source.
func TestParseRefusesWhatIsNotAStream(t *testing.T) {
	invalid := map[string]string{
		"a NUL as the whole document":  "\x00\n",
		"a NUL in a value":             "a: \x00\n",
		"a control character in a key": "a\x01b: 1\n",
		"a byte that is not text":      "\xbf\n",
		"a truncated character":        "a: \xe2\x82\n",
	}

	for name, source := range invalid {
		t.Run(name, func(t *testing.T) {
			_, err := refparser.ParseBytes([]byte(source), refparser.ParseComments)
			assert.Errorf(t, err, "accepted %q", source)
		})
	}

	valid := map[string]string{
		"a NUL written as an escape":  "k: \"\\x00\"\n",
		"text outside ASCII":          "k: héllo\n",
		"a character outside the BMP": "k: 😀\n",
		"a replacement character":     "k: \uFFFD\n",
		"a next-line character":       "k: \u0085\n",
		"a non-breaking space":        "k: \u00a0\n",
	}

	for name, source := range valid {
		t.Run(name, func(t *testing.T) {
			_, err := refparser.ParseBytes([]byte(source), refparser.ParseComments)
			assert.NoErrorf(t, err, "rejected %q", source)
		})
	}
}

// TestParseDropsAByteOrderMarkOpeningTheStream covers a mark written by an
// editor at the head of a file.
//
// nb-char is c-printable less b-char and c-byte-order-mark, so a mark is not a
// character any node may hold: it marks an l-document-prefix and nothing else.
// Read as an ordinary character it became part of whatever came next -- the
// key of the first entry, or the '%' of a directive, which then stopped being
// one. The document it opens is refused for the same reason.
func TestParseDropsAByteOrderMarkOpeningTheStream(t *testing.T) {
	const mark = "\ufeff"

	tests := map[string]struct {
		source string
		want   string
	}{
		"before a mapping key":     {mark + "a: 1\n", "a: 1\n"},
		"before a document marker": {mark + "---\na: 1\n", "---\na: 1\n"},
		"before a comment":         {mark + "# c\na: 1\n", "# c\na: 1\n"},
		"before a directive":       {mark + "%YAML 1.2\n---\na: 1\n", "%YAML 1.2\n---\na: 1\n"},

		// l-document-prefix is a run, so more than one is a run of them.
		"twice over": {mark + mark + "a: 1\n", "a: 1\n"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := refparser.ParseBytes([]byte(test.source), refparser.ParseComments)
			require.NoErrorf(t, err, "rejected %q", test.source)
			assert.Equal(t, test.want, file.String())
		})
	}
}

// TestParseTabWhereIndentationBelongs covers a tab opening a line at the root.
//
// s-indent(n) is s-space x n, so block structure is introduced by spaces and
// nothing else and a tab cannot stand in for them. A tab is separation rather
// than indentation, though, and a flow node or a scalar at the root is reached
// through s-separate -- so the same tab that makes "\tfoo: 1" invalid leaves
// "\t{}" a perfectly good document.
//
// The distinction was lost at the root, where the entry has no enclosing level
// to be measured against: a tab there was read as the indentation it may not
// be. It survived for a quoted key longest, since the check it slipped past
// read the origin buffer and a quoted scalar resets it.
func TestParseTabWhereIndentationBelongs(t *testing.T) {
	invalid := map[string]string{
		"a tab before a quoted key at the root": "\t\"\": a\n",
		"a tab before a single-quoted key":      "\t'k': v\n",
		"a tab before a plain key at the root":  "\tfoo: a\n",
		"a tab before a sequence at the root":   "\t- a\n",
		"a tab before a key, further entries":   "\ta: 1\nb: 2\n",
	}

	for name, source := range invalid {
		t.Run(name, func(t *testing.T) {
			_, err := refparser.ParseBytes([]byte(source), refparser.ParseComments)
			assert.Errorf(t, err, "accepted %q", source)
		})
	}

	// The same tab in front of something reached through s-separate.
	valid := map[string]string{
		"a tab before a flow mapping":     "\t{}\n",
		"a tab before a flow sequence":    "\t[\n\t]\n",
		"a tab before a flow pair":        "\t{a: 1}\n",
		"a tab before a plain scalar":     "\tfoo\n",
		"a tab before a quoted scalar":    "\t\"foo\"\n",
		"a tab before a comment":          "\t#c\n",
		"a tab after a space of indent":   "foo:\n \tbar\n",
		"a tab inside a folded plain one": "x:\n - x\n  \tx\n",
	}

	for name, source := range valid {
		t.Run(name, func(t *testing.T) {
			_, err := refparser.ParseBytes([]byte(source), refparser.ParseComments)
			assert.NoErrorf(t, err, "rejected %q", source)
		})
	}
}

// TestParseWhitespaceOnlyLines covers a line that holds nothing but whitespace.
//
// It is a blank line however it is spelled. A tab cannot be indentation and is
// refused where one is expected, which is right -- but on a line of its own
// there is no indentation to refuse, only a gap between the entries around it.
func TestParseWhitespaceOnlyLines(t *testing.T) {
	valid := map[string]string{
		"a line holding one tab":            "foo: 1\n\t\nbar: 2\n",
		"a line mixing tabs and spaces":     "foo: 1\n\t \t\nbar: 2\n",
		"a tab line inside a nested map":    "foo:\n  a: 1\n\t\n  b: 2\n",
		"a tab line between sequence items": "- a\n\t\n- b\n",
	}

	for name, source := range valid {
		t.Run(name, func(t *testing.T) {
			_, err := refparser.ParseBytes([]byte(source), refparser.ParseComments)
			assert.NoErrorf(t, err, "rejected %q", source)
		})
	}

	// And a tab that does stand in for indentation is still refused.
	invalid := map[string]string{
		"a tab before a mapping entry":  "foo: 1\n\tbar: 2\n",
		"a tab before a sequence entry": "foo:\n\t- a\n",
	}

	for name, source := range invalid {
		t.Run(name, func(t *testing.T) {
			_, err := refparser.ParseBytes([]byte(source), refparser.ParseComments)
			assert.Errorf(t, err, "accepted %q", source)
		})
	}
}

// TestParseValueMustBeIndentedPastItsKey covers a value written level with the
// key it belongs to.
//
// An entry's value goes further in than its key. Level with the key, a token
// can only open the next entry: another key, or the '-' of a block sequence,
// which by convention sits at its own key's column. Anything else has nowhere
// to belong, and reading it as the value made "a:\nb" the mapping {a: b} where
// every other implementation refuses the document.
func TestParseValueMustBeIndentedPastItsKey(t *testing.T) {
	invalid := map[string]string{
		"a plain scalar level with the key": "a:\nb\n",
		"a quoted key, same":                "\"a\":\nb\n",
		"a flow collection level with it":   "a:\n[1, 2]\n",
		"an alias level with it":            "k: &x 1\na:\n*x\n",
		"nested one level in":               "top:\n  a:\n  b\n",

		// A property standing on the key's line names a block node, which is
		// indented past the key like any other value. The property used to
		// swallow whatever came next whatever column it sat at, so "k: &a\n1"
		// read as {k: 1}.
		"an anchor on the key's line":    "k: &a\n1\n",
		"a tag on the key's line":        "k: !!str\n1\n",
		"an anchor under an empty key":   ": &a\n1\n",
		"an anchor, nested one level in": "top:\n  k: &a\n  1\n",
	}

	for name, source := range invalid {
		t.Run(name, func(t *testing.T) {
			_, err := refparser.ParseBytes([]byte(source), refparser.ParseComments)
			assert.Errorf(t, err, "accepted %q", source)
		})
	}

	valid := map[string]string{
		"the value indented past the key":      "a:\n  b\n",
		"a block sequence at the key's column": "a:\n- x\n",
		"the next entry":                       "a:\nb: 1\n",
		"an empty value at the end":            "a:\n",
		"a comment between them":               "a:\n# c\nb: 1\n",
		"a new document":                       "a:\n---\nb: 1\n",

		// The same property with its node where a node belongs.
		"an anchor naming an indented value":     "k: &a\n  1\n",
		"an anchor naming a sequence":            "k: &a\n- 1\n",
		"an anchor under an empty key, indented": ": &a\n  1\n",
		"an anchor naming nothing at all":        ": &a\n",
		"an anchor before the next entry":        "k: &a\nnext: 1\n",
		"an anchor before a new document":        "k: &a\n---\nb: 1\n",
	}

	for name, source := range valid {
		t.Run(name, func(t *testing.T) {
			_, err := refparser.ParseBytes([]byte(source), refparser.ParseComments)
			assert.NoErrorf(t, err, "rejected %q", source)
		})
	}
}
