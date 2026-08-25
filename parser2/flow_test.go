package parser2_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/parser2"
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

		// A mapping's key may still take the next line for its ':', whatever
		// the key is made of.
		"quoted key and colon on separate lines":     "{ \"key\"\n  : value }\n",
		"collection key and colon on separate lines": "{ {a: 1}\n  : value }\n",
	}

	for name, source := range valid {
		t.Run(name, func(t *testing.T) {
			_, err := parser2.ParseBytes([]byte(source), parser2.ParseComments)
			assert.NoErrorf(t, err, "rejected %q", source)
		})
	}

	invalid := map[string]string{
		// A single-pair entry in a flow sequence is an implicit key, which has
		// to fit on one line with its ':'. Quoting the key does not exempt it.
		"implicit key followed by a newline":          "[ key\n  : value ]\n",
		"quoted implicit key followed by a newline":   "[ \"key\"\n  : value ]\n",
		"quoted implicit key with an adjacent value":  "[ \"key\"\n  :value ]\n",
		"collection key followed by a newline":        "[ {a: 1}\n  : value ]\n",
		"single-quoted implicit key across two lines": "[ 'key'\n  : value ]\n",
	}

	for name, source := range invalid {
		t.Run(name, func(t *testing.T) {
			_, err := parser2.ParseBytes([]byte(source), parser2.ParseComments)
			assert.Errorf(t, err, "accepted %q", source)
		})
	}
}

// TestParseAdjacentValuesInFlow covers a ':' written with no space in front of
// its value.
//
// The spec allows it only after a JSON-like key -- a quoted scalar or a flow
// collection -- which is what makes "{a: 1}" and JSON's own "{"a":1}" both
// readable by the same parser2. After a plain scalar the ':' belongs to the
// scalar, so [ a:b ] holds one entry and not a pair.
func TestParseAdjacentValuesInFlow(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"quoted key in a flow sequence":     {"[ \"JSON like\":adjacent ]\n", "[\"JSON like\": adjacent]\n"},
		"collection key in a flow sequence": {"[ {JSON: like}:adjacent ]\n", "[{JSON: like}: adjacent]\n"},
		"quoted key in a flow mapping":      {"{\"a\":1}\n", "{\"a\": 1}\n"},
		"several in one collection":         {"[\"a\":1, [b]:2]\n", "[\"a\": 1, [b]: 2]\n"},

		// Not after a plain scalar: the ':' belongs to the scalar. In a
		// sequence that leaves one entry; in a mapping it leaves one key with
		// no value, which is written back with the ':' that says so.
		"plain key in a flow sequence": {"[ a:b ]\n", "[a:b]\n"},
		"plain key in a flow mapping":  {"{ a:b }\n", "{a:b:}\n"},

		// A ':' in front of what ends the entry closes the key wherever it is
		// written: a plain scalar cannot hold one.
		"absent value before a brace": {"{a:}\n", "{a:}\n"},
		"absent value before a comma": {"{a:, b: 1}\n", "{a:, b: 1}\n"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := parser2.ParseBytes([]byte(test.source), parser2.ParseComments)
			require.NoError(t, err)
			assert.Equal(t, test.want, file.String())

			reread, err := parser2.ParseBytes([]byte(test.want), parser2.ParseComments)
			require.NoErrorf(t, err, "cannot read back %q", test.want)
			assert.Equal(t, test.want, reread.String())
		})
	}
}

// TestParseFlowCollectionsAsKeys covers a flow collection used as a mapping
// key, nested inside another one used the same way.
//
// Once "[b]: d" is grouped as an entry, the group reports the type of the token
// it opens with -- a '['. The search for where the enclosing key begins counted
// that as one more open bracket with no ']' to match it, and gave up on the
// document; a collection used as a key was readable only at the outermost
// level.
func TestParseFlowCollectionsAsKeys(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"one level":            {"[ [b]: d ]\n", "[[b]: d]\n"},
		"two levels":           {"[ [[b]: d]: 23 ]\n", "[[[b]: d]: 23]\n"},
		"beside other entries": {"[ [a, [ [[b,c]]: d, e]]: 23 ]\n", "[[a, [[[b, c]]: d, e]]: 23]\n"},
		"in a flow mapping":    {"{ [[b]: d]: 23 }\n", "{[[b]: d]: 23}\n"},
		"spanning lines":       {"[\n  [[b]: d]: 23\n]\n", "[[[b]: d]: 23]\n"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := parser2.ParseBytes([]byte(test.source), parser2.ParseComments)
			require.NoError(t, err)
			assert.Equal(t, test.want, file.String())

			reread, err := parser2.ParseBytes([]byte(test.want), parser2.ParseComments)
			require.NoErrorf(t, err, "cannot read back %q", test.want)
			assert.Equal(t, test.want, reread.String())
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
			file, err := parser2.ParseBytes([]byte(test.source), parser2.ParseComments)
			require.NoError(t, err)
			assert.Equal(t, test.want, file.String())

			// What is written has to read back, and to the same text again:
			// the layout it moved to is a layout the parser accepts.
			reread, err := parser2.ParseBytes([]byte(test.want), parser2.ParseComments)
			require.NoErrorf(t, err, "cannot read back %q", test.want)
			assert.Equal(t, test.want, reread.String())
		})
	}
}

// TestParseEmptyNodeInAFlowCollection covers the entries a flow collection may
// hold that carry no scalar of their own.
//
// Two productions: c-ns-flow-map-empty-key-entry, an entry whose key is e-node,
// and ns-flow-pair, which a flow sequence admits as an entry and whose value may
// be e-node as well. Both were refused -- "{&a}" as "could not find flow mapping
// end token '}'" and "[:]" as "could not find '[' character corresponding to
// ']'".
//
// Each rendered form below is one the recognizer compiled from
// yaml-spec-1.2.json accepts, and each reads back to the same text.
func TestParseEmptyNodeInAFlowCollection(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
		value  any
	}{
		"an anchor alone in a flow mapping": {
			source: "{&a}\n",
			want:   "{&a :}\n",
			value:  map[string]any{"null": nil},
		},
		"an anchor alone before another entry": {
			source: "{&a, b: 1}\n",
			want:   "{&a :, b: 1}\n",
			value:  map[string]any{"null": nil, "b": uint64(1)},
		},
		"an anchor alone after another entry": {
			source: "{b: 1, &a}\n",
			want:   "{b: 1, &a :}\n",
			value:  map[string]any{"null": nil, "b": uint64(1)},
		},
		"a pair with neither side": {
			source: "[:]\n",
			want:   "[:]\n",
			value:  []any{map[string]any{"null": nil}},
		},
		"a pair with neither side, before an entry": {
			source: "[:, a]\n",
			want:   "[:, a]\n",
			value:  []any{map[string]any{"null": nil}, "a"},
		},
		"a pair with neither side, after an entry": {
			source: "[a, :]\n",
			want:   "[a, :]\n",
			value:  []any{"a", map[string]any{"null": nil}},
		},
		"a pair with no value": {
			source: "[a:]\n",
			want:   "[a:]\n",
			value:  []any{map[string]any{"a": nil}},
		},
		"an explicit key with no value": {
			source: "[? a]\n",
			want:   "[? a :]\n",
			value:  []any{map[string]any{"a": nil}},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := parser2.ParseBytes([]byte(test.source), parser2.ParseComments)
			require.NoError(t, err)
			assert.Equal(t, test.want, file.String())

			reread, err := parser2.ParseBytes([]byte(test.want), parser2.ParseComments)
			require.NoErrorf(t, err, "cannot read back %q", test.want)
			assert.Equal(t, test.want, reread.String())

			var got any
			require.NoError(t, yaml.Unmarshal([]byte(test.source), &got))
			assert.Equal(t, test.value, got)
		})
	}
}
