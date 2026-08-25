package parser2_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/parser2"
)

// TestParseKeys covers the mapping keys YAML allows that are not plain
// scalars: absent keys, keys named explicitly with '?', and collections.
//
// Each case asserts the shape of the key rather than only that the document
// parses -- "accepted" is a weak claim when the alternative is accepting it
// with the value silently dropped, which is what an earlier attempt did.
func TestParseKeys(t *testing.T) {
	tests := map[string]struct {
		source  string
		keyType any
		render  string
	}{
		"absent key": {
			source:  ": a\n",
			keyType: (*ast.NullNode)(nil),
			render:  ": a\n",
		},
		"absent key in a flow mapping": {
			source:  "{key: value, : empty}\n",
			keyType: (*ast.StringNode)(nil),
			render:  "{key: value, : empty}\n",
		},
		"explicit scalar key": {
			source:  "? a\n: b\n",
			keyType: (*ast.MappingKeyNode)(nil),
			render:  "? a\n: b\n",
		},
		"explicit sequence key": {
			source:  "? - a\n: b\n",
			keyType: (*ast.MappingKeyNode)(nil),
		},
		"explicit mapping key": {
			source:  "? {a: 1}\n: b\n",
			keyType: (*ast.MappingKeyNode)(nil),
		},
		"flow sequence as key": {
			source:  "[flow]: block\n",
			keyType: (*ast.SequenceNode)(nil),
			render:  "[flow]: block\n",
		},
		"flow mapping as key": {
			source:  "{a: 1}: block\n",
			keyType: (*ast.MappingNode)(nil),
			render:  "{a: 1}: block\n",
		},
		"flow sequence as key inside a flow mapping": {
			source:  "{a: [b, c], [d, e]: f}\n",
			keyType: (*ast.StringNode)(nil),
			render:  "{a: [b, c], [d, e]: f}\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := parser2.ParseBytes([]byte(test.source), 0)
			require.NoError(t, err)
			require.Len(t, file.Docs, 1)

			mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
			require.Truef(t, ok, "expected a mapping, got %T", file.Docs[0].Body)
			require.NotEmpty(t, mapping.Values)

			assert.IsTypef(t, test.keyType, mapping.Values[0].Key,
				"key of the first entry")

			if test.render != "" {
				assert.Equal(t, test.render, file.String())
			}
		})
	}
}

// TestParseKeysRejected covers keys YAML does not allow, which a strict parser
// has to refuse rather than interpret.
func TestParseKeysRejected(t *testing.T) {
	tests := map[string]string{
		"collection key spanning lines": "[23\n]: 42\n",
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := parser2.ParseBytes([]byte(source), 0)
			assert.Error(t, err)
		})
	}
}
