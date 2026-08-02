package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser"
)

// TestParseTagsOnEmptyScalars covers a scalar tag with nothing after it.
//
// Such a tag marks the empty node, exactly as an anchor with nothing after it
// names it. What follows the tag is the next entry of the collection around it,
// not the tag's value, and reading it as one is what used to fail.
func TestParseTagsOnEmptyScalars(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"as a mapping value at the end of the document": {
			source: "a: !!str\n",
			want:   "a: !!str\n",
		},
		"as a mapping value with a sibling below": {
			source: "b: !!str\nc: 1\n",
			want:   "b: !!str\nc: 1\n",
		},
		"as a sequence entry at the end of the document": {
			source: "- !!str\n",
			want:   "- !!str\n",
		},
		"as a sequence entry with a sibling below": {
			source: "- !!str\n- a\n",
			want:   "- !!str\n- a\n",
		},
		"as an explicit key": {
			source: "? !!str\n",
			want:   "? !!str\n:\n",
		},
		"as a value in a flow mapping": {
			source: "{a: !!str, b: 1}\n",
			want:   "{a: !!str, b: 1}\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(test.source), parser.ParseComments)
			require.NoError(t, err)
			assert.Equal(t, test.want, file.String())

			// And it settles: what was written reads back to the same text.
			reread, err := parser.ParseBytes([]byte(test.want), parser.ParseComments)
			require.NoErrorf(t, err, "cannot read back %q", test.want)
			assert.Equal(t, test.want, reread.String())
		})
	}
}

// TestParseEmptyKeysWithAnchorsAndTags pins the one position an anchored or
// tagged empty scalar is still refused in, so that fixing it is noticed.
//
// The key is accepted at the top level of a document but not inside a nested
// block mapping with a sibling entry below it. Both anchors and tags fail the
// same way and at the same place, on a check that compares the key's line and
// column with the token after it -- the positions of the null node standing in
// for the empty scalar are what mislead it, so this is about that node rather
// than about anchors or tags.
func TestParseEmptyKeysWithAnchorsAndTags(t *testing.T) {
	accepted := map[string]string{
		"anchored key at the top level": "&a : a\n",
		"tagged key at the top level":   "!!null : a\n",
		"tagged key and tagged value":   "!!str : !!null\n",
	}

	for name, source := range accepted {
		t.Run(name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(source), parser.ParseComments)
			assert.NoErrorf(t, err, "rejected %q", source)
		})
	}

	refused := map[string]string{
		"anchored key nested under a key":  "x:\n  &a : a\n  b: 1\n",
		"tagged key nested under a key":    "x:\n  !!null : a\n  b: 1\n",
		"anchored key in a sequence entry": "-\n  &a : a\n  b: 1\n",
		"tagged key in a sequence entry":   "-\n  !!null : a\n  b: 1\n",
	}

	for name, source := range refused {
		t.Run(name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(source), parser.ParseComments)
			assert.Errorf(t, err,
				"%q is accepted now -- move it to the accepted set above", source)
		})
	}
}
