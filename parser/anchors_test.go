package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser"
)

// TestParseAnchorsOnEmptyScalars covers an anchor with nothing after it.
//
// Such an anchor names the empty node -- "a: &x" is a valid document and *x
// resolves to null. Refusing it made an anchor the one thing that could not be
// attached to an absent value, which the YAML Test Suite exercises in every
// position a value may be absent in.
func TestParseAnchorsOnEmptyScalars(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"as a mapping value at the end of the document": {
			source: "b: &b\n",
			want:   "b: &b\n",
		},
		"as a mapping value with more to follow": {
			source: "a: &x\nb: 1\n",
			want:   "a: &x\nb: 1\n",
		},
		"as an explicit key": {
			source: "? &d\n",
			want:   "? &d\n:\n",
		},
		"as an explicit key's value": {
			source: "? &e\n: &a\n",
			want:   "? &e\n: &a\n",
		},
		// The space before the ':' is not decoration: ':' is a legal anchor
		// character, so "&a: a" anchors the name "a:" over the scalar a and is
		// not a mapping at all.
		"as a mapping key": {
			source: "&a : a\n",
			want:   "&a : a\n",
		},
		"as a mapping key nested under another": {
			source: "x:\n  &a : a\n  b: 1\n",
			want:   "x:\n  &a : a\n  b: 1\n",
		},
		"as a mapping key in a sequence entry": {
			source: "-\n  &a : a\n  b: 1\n",
			want:   "- &a : a\n  b: 1\n",
		},
		"as a sequence entry": {
			source: "- &a\n- a\n",
			want:   "- &a\n- a\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(test.source), parser.Comments())
			require.NoError(t, err)
			assert.Equal(t, test.want, file.String())

			// And it settles: what was written reads back to the same text.
			reread, err := parser.ParseBytes([]byte(test.want), parser.Comments())
			require.NoErrorf(t, err, "cannot read back %q", test.want)
			assert.Equal(t, test.want, reread.String())
		})
	}
}

// TestParseAnchorsOnFlowCollectionKeys covers a property written before a flow
// collection used as a mapping key.
//
// It is the key's: "&k [a, b]: v" names the sequence. The key was taken to
// begin at its '[', which left the anchor outside it and attached to the
// mapping the entry belongs to -- silently, so the document still parsed and
// resolved *k to the wrong node, and outright refused as a second anchor when
// the mapping already carried one.
func TestParseAnchorsOnFlowCollectionKeys(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"anchor on a flow sequence key": {
			source: "&key [a, b]: value\n",
			want:   "&key [a, b]: value\n",
		},
		"anchor on a flow mapping key": {
			source: "&key {a: 1}: value\n",
			want:   "&key {a: 1}: value\n",
		},
		"tag on a flow sequence key": {
			source: "!!seq [a]: value\n",
			want:   "!!seq [a]: value\n",
		},
		"beneath an anchor of the mapping's own": {
			source: "&mapping\n&key [ a ]: value\n",
			want:   "&mapping\n  &key [a]: value\n",
		},
		"inside a flow mapping": {
			source: "{ &a [a, b]: c }\n",
			want:   "{&a [a, b]: c}\n",
		},
		"with an anchored entry as well": {
			source: "&key [ &item a, b ]: value\n",
			want:   "&key [&item a, b]: value\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(test.source), parser.Comments())
			require.NoError(t, err)
			assert.Equal(t, test.want, file.String())

			reread, err := parser.ParseBytes([]byte(test.want), parser.Comments())
			require.NoErrorf(t, err, "cannot read back %q", test.want)
			assert.Equal(t, test.want, reread.String())
		})
	}
}

// TestParseAnchorsStillNeedANameAndOneValue keeps the checks that the empty
// value had to be threaded past.
func TestParseAnchorsStillNeedANameAndOneValue(t *testing.T) {
	sources := map[string]string{
		"no name":               "a: &\n",
		"two anchors in a row":  "a: &x &y value\n",
		"sequence after anchor": "a: &x - b\n",
	}

	for name, source := range sources {
		t.Run(name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(source), parser.Comments())
			assert.Errorf(t, err, "accepted %q", source)
		})
	}
}
