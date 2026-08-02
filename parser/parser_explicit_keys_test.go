package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser"
)

// TestParseExplicitKeyValues covers what may follow the ':' of an explicit key.
//
// The ':' stands alone on its line, with the key written above it after a '?'.
// The level the value is measured against is that ':' -- it had been left at
// the level of whatever the key held, so a key that was a block sequence put it
// two columns further in than the entry really sits, and a block scalar value
// was cut off at its first line.
func TestParseExplicitKeyValues(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"block scalar after a sequence key": {
			source: "? - a\n: |\n  b\n",
			want:   "? - a\n: |\n  b\n",
		},
		"folded scalar after a sequence key": {
			source: "? - a\n: >\n  b\n",
			want:   "? - a\n: >\n  b\n",
		},
		"nested under a mapping key": {
			source: "c:\n  ? - a\n  : >\n    b\n",
			want:   "c:\n  ? - a\n  : >\n    b\n",
		},
		"scalar after a sequence key": {
			source: "? - a\n: b\n",
			want:   "? - a\n: b\n",
		},
		"block scalar as the key itself": {
			source: "? >\n  a\n:\n",
			want:   "? >\n  a\n:\n",
		},
		"several lines of block scalar": {
			source: "? - a\n: |\n  b\n  c\n",
			want:   "? - a\n: |\n  b\n  c\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(test.source), parser.ParseComments)
			require.NoError(t, err)
			assert.Equal(t, test.want, file.String())

			reread, err := parser.ParseBytes([]byte(test.want), parser.ParseComments)
			require.NoErrorf(t, err, "cannot read back %q", test.want)
			assert.Equal(t, test.want, reread.String())
		})
	}
}
