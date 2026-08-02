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
			_, err := parser.ParseBytes([]byte(source), parser.ParseComments)
			assert.NoErrorf(t, err, "rejected %q", source)
		})
	}

	// Under a key there is a level, and content at column 1 is outside it.
	t.Run("not under a mapping key", func(t *testing.T) {
		_, err := parser.ParseBytes([]byte("a: |\nb\n"), parser.ParseComments)
		assert.Error(t, err)
	})
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
			file, err := parser.ParseBytes([]byte(source), parser.ParseComments)
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
			_, err := parser.ParseBytes([]byte(source), parser.ParseComments)
			assert.Errorf(t, err, "accepted %q", source)
		})
	}
}
