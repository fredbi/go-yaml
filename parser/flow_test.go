package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser"
)

// TestParseFlowKeyLineBreaks covers where a flow mapping's key may end and its
// ':' may begin.
//
// A flow mapping's key is under no single-line restriction: it may span lines,
// and a line break before the ':' is ordinary separation. A pair written inside
// a flow sequence is a different thing -- an implicit key -- and is held to the
// stricter rule.
func TestParseFlowKeyLineBreaks(t *testing.T) {
	valid := map[string]string{
		"colon on the line after the key": "{foo\n: bar}\n",
		"key and colon on their own lines": "k: {\n" +
			" k\n" +
			" :\n" +
			" v\n" +
			" }\n",
		"key spanning lines":        "- { multi\n  line: value}\n",
		"key spanning lines nested": "{ matches\n% : 20 }\n",
	}

	for name, source := range valid {
		t.Run(name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(source), parser.ParseComments)
			assert.NoErrorf(t, err, "rejected %q", source)
		})
	}

	invalid := map[string]string{
		// A single-pair entry in a flow sequence is an implicit key, which has
		// to fit on one line with its ':'.
		"implicit key followed by a newline": "[ key\n  : value ]\n",
	}

	for name, source := range invalid {
		t.Run(name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(source), parser.ParseComments)
			assert.Errorf(t, err, "accepted %q", source)
		})
	}
}

// TestParseFlowComments covers comments written inside a flow collection.
//
// They used to be refused in one position, dropped in another, and in a third
// written back onto the collection's single line -- where everything after them
// is commented out, including the bracket that closes the collection.
func TestParseFlowComments(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"before the comma": {
			source: "[ word1\n# comment\n, word2]\n",
			want:   "[\n  word1,\n  # comment\n  word2\n]\n",
		},
		"after the comma on its own line": {
			source: "[ a,\n# comment\n  b ]\n",
			want:   "[\n  a,\n  # comment\n  b\n]\n",
		},
		"in a flow mapping": {
			source: "{ a: 1,\n# comment\n  b: 2 }\n",
			want:   "{\n  a: 1,\n  # comment\n  b: 2\n}\n",
		},
		"before the closing bracket": {
			source: "[ a, b\n# comment\n]\n",
			want:   "[\n  a,\n  b\n  # comment\n]\n",
		},
		// On the ',' line the comment was written about the entry the ',' comes
		// after, and stays with it.
		"on the comma's line in a sequence": {
			source: "[ a, # comment\n  b ]\n",
			want:   "[\n  a, # comment\n  b\n]\n",
		},
		"on the comma's line in a mapping": {
			source: "{ a: 1, # comment\n  b: 2 }\n",
			want:   "{\n  a: 1, # comment\n  b: 2\n}\n",
		},
		// Nothing to carry, so nothing changes: the collection stays on the one
		// line it is meant to occupy.
		"none at all": {
			source: "{a: 1, b: 2}\n",
			want:   "{a: 1, b: 2}\n",
		},
		"none at all in a sequence": {
			source: "[a, b]\n",
			want:   "[a, b]\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(test.source), parser.ParseComments)
			require.NoError(t, err)
			assert.Equal(t, test.want, file.String())

			// What is written has to read back, and to the same text again:
			// the layout it moved to is a layout the parser accepts.
			reread, err := parser.ParseBytes([]byte(test.want), parser.ParseComments)
			require.NoErrorf(t, err, "cannot read back %q", test.want)
			assert.Equal(t, test.want, reread.String())
		})
	}
}
