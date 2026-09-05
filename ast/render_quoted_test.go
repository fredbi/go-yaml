package ast_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/parser"
)

// TestRenderQuotedScalarsKeepTheirValue covers a line break inside a quoted
// scalar.
//
// A single-quoted scalar has no escapes, and a break written inside one is
// folded away: one break becomes a space, and n+1 breaks become n breaks.
// Writing the value's breaks back as bare breaks therefore changed the value on
// every cycle, and where the closing quote landed in column 1 the result no
// longer parsed at all. Such a value is written double-quoted, where a break is
// an escape.
func TestRenderQuotedScalarsKeepTheirValue(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"one break folds to a space":       {"a: '\n  '\n", "a: ' '\n"},
		"a blank line encodes a break":     {"a: '\n\n  '\n", "a: \"\\n\"\n"},
		"two blank lines encode two":       {"a: '\n\n\n  '\n", "a: \"\\n\\n\"\n"},
		"double quotes already escape":     {"a: \"\n\n  \"\n", "a: \"\\n\"\n"},
		"a single-line value keeps quotes": {"a: 'x'\n", "a: 'x'\n"},
		"an embedded quote stays doubled":  {"a: 'it''s'\n", "a: 'it''s'\n"},
		"folded lines of the spec example": {
			source: "' 1st non-empty\n\n 2nd non-empty \n\t3rd non-empty '\n",
			want:   "\" 1st non-empty\\n2nd non-empty 3rd non-empty \"\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(test.source), parser.WithComments())
			require.NoError(t, err)
			assert.Equal(t, test.want, ast.NewRenderer().File(file))

			reread, err := parser.ParseBytes([]byte(test.want), parser.WithComments())
			require.NoErrorf(t, err, "cannot read back %q", test.want)
			assert.Equal(t, test.want, ast.NewRenderer().File(reread))
		})
	}
}
