package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser"
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
			_, err := parser.ParseBytes([]byte(test.invalid), parser.ParseComments)
			assert.Errorf(t, err, "accepted %q", test.invalid)

			_, err = parser.ParseBytes([]byte(test.valid), parser.ParseComments)
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
			_, err := parser.ParseBytes([]byte(source), parser.ParseComments)
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
			_, err := parser.ParseBytes([]byte(source), parser.ParseComments)
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
			_, err := parser.ParseBytes([]byte(source), parser.ParseComments)
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
	}

	for name, source := range invalid {
		t.Run(name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(source), parser.ParseComments)
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
	}

	for name, source := range valid {
		t.Run(name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(source), parser.ParseComments)
			assert.NoErrorf(t, err, "rejected %q", source)
		})
	}
}
