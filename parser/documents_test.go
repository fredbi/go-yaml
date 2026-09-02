package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser"
)

// TestParseBlockScalarAtTheDocumentRoot covers a block scalar that is the whole
// document.
//
// Nothing encloses it, so its content has no level to be indented past and may
// start at column 1 -- which is the spec's own bare-documents example. The
// scanner held such a header to the same level as one written under a key, and
// refused its content for not being indented past a level that is not there.
func TestParseBlockScalarAtTheDocumentRoot(t *testing.T) {
	valid := map[string]string{
		"literal with content at column 1":  "|\n1\n",
		"folded with content at column 1":   ">\na\n",
		"literal indented anyway":           "|\n  a\n",
		"a stated indent counts from there": "|2-\n\n  text\n",
	}

	for name, source := range valid {
		t.Run(name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(source), parser.Comments())
			assert.NoErrorf(t, err, "rejected %q", source)
		})
	}

	// Under a key there is a level, and content at column 1 is outside it.
	t.Run("not under a mapping key", func(t *testing.T) {
		_, err := parser.ParseBytes([]byte("a: |\nb\n"), parser.Comments())
		assert.Error(t, err)
	})
}

// TestParseEmptyDocumentsKeepTheirStream covers a document with nothing in it
// between two others.
//
// It is a document like any other, and so is everything after it. One "---"
// straight after another used to end the parse: the empty document was
// returned and the rest of the stream was dropped without a word, so
// "a: 1\n---\n---\nb: 2\n" came back as two documents and the second value
// was simply gone.
func TestParseEmptyDocumentsKeepTheirStream(t *testing.T) {
	tests := map[string]struct {
		source string
		docs   int
	}{
		"an empty document in the middle":  {"a: 1\n---\n---\nb: 2\n", 3},
		"a blank line between the markers": {"a: 1\n---\n\n---\nb: 2\n", 3},
		"a comment between the markers":    {"a: 1\n---\n# c\n---\nb: 2\n", 3},
		"an empty document first":          {"---\n---\nb: 2\n", 2},
		"two in a row":                     {"---\n---\n---\nc: 3\n", 3},
		"an empty document last":           {"a: 1\n---\n", 2},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Without ParseComments the comment is not even a token, which is
			// how the drop went unnoticed: the shape it needs is two markers
			// with nothing between them.
			for _, mode := range []struct {
				name string
				opts []parser.Option
			}{{"plain", nil}, {"comments", []parser.Option{parser.Comments()}}} {
				file, err := parser.ParseBytes([]byte(test.source), mode.opts...)
				require.NoErrorf(t, err, "mode %s", mode.name)
				assert.Lenf(t, file.Docs, test.docs, "mode %s: %q", mode.name, test.source)
			}
		})
	}
}

// TestParseDocumentsAfterASuffix covers what may follow the "..." that ends a
// document.
//
// It takes the rest of its line, where only a comment may follow it. The next
// line starts a new document, and that document may be a bare one: any scalar
// written there used to be refused as content belonging to the document that
// had just ended.
func TestParseDocumentsAfterASuffix(t *testing.T) {
	valid := map[string]string{
		"a scalar on the next line":       "a\n...\nb\n",
		"a block scalar on the next line": "a\n...\n|\nb\n",
		"two suffixes in a row":           "a\n...\n# note\n...\n|\nb\n",
		"a mapping on the next line":      "a\n...\nb: 1\n",
	}

	for name, source := range valid {
		t.Run(name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(source), parser.Comments())
			require.NoErrorf(t, err, "rejected %q", source)
			assert.GreaterOrEqual(t, len(file.Docs), 2, "%q is more than one document", source)
		})
	}

	// On the "..." line itself there is no room for a second document.
	invalid := map[string]string{
		"a scalar on the same line":  "a\n... b\n",
		"a mapping on the same line": "a\n... b: 1\n",
	}

	for name, source := range invalid {
		t.Run(name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(source), parser.Comments())
			assert.Errorf(t, err, "accepted %q", source)
		})
	}
}

// TestParseExplicitKeyComments covers a comment written on the "?" line.
//
// It belongs to the key. The group that holds an explicit key ends on the key
// itself rather than on a ':', and the comment was carried over to the value as
// though it had been written after one -- so it came back on both lines, and
// the document gained a comment every time it was read and written.
func TestParseExplicitKeyComments(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"on a key with no value": {
			source: "? a # note\n",
			want:   "? a # note\n:\n",
		},
		"on a key with a value": {
			source: "? a # note\n: b\n",
			want:   "? a # note\n: b\n",
		},
		"on the value instead": {
			source: "? a\n: b # note\n",
			want:   "? a\n: b # note\n",
		},
		"on both": {
			source: "? a # key\n: b # value\n",
			want:   "? a # key\n: b # value\n",
		},
		"on a block scalar key": {
			source: "? |\n  a\n: b # note\n",
			want:   "? |\n  a\n: b # note\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(test.source), parser.Comments())
			require.NoError(t, err)
			assert.Equal(t, test.want, file.String())

			// And it settles: a second cycle adds nothing.
			reread, err := parser.ParseBytes([]byte(test.want), parser.Comments())
			require.NoErrorf(t, err, "cannot read back %q", test.want)
			assert.Equal(t, test.want, reread.String())
		})
	}
}
