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
