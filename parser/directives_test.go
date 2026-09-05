package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser"
)

// TestParseCommentsAroundDirectives covers comment lines written between a
// directive and the '---' that opens the document.
//
// They belong to neither, and the parser used to refuse the whole document over
// them -- but only when it was reading comments, so the same source parsed or
// failed depending on the mode.
func TestParseCommentsAroundDirectives(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"comment on the directive's own line": {
			source: "%YAML 1.2 # note\n---\n\"foo\"\n",
			want:   "%YAML 1.2 # note\n---\n\"foo\"\n",
		},
		"comment on the line below": {
			source: "%YAML 1.2\n# note\n---\n\"foo\"\n",
			want:   "%YAML 1.2\n# note\n---\n\"foo\"\n",
		},
		"comment continued over lines": {
			source: "%FOO bar baz # note\n   # continued\n--- \"foo\"\n",
			want:   "%FOO bar baz # note\n# continued\n---\n\"foo\"\n",
		},
		"comment above the directive": {
			source: "# note\n%YAML 1.2\n---\n\"foo\"\n",
			want:   "# note\n%YAML 1.2\n---\n\"foo\"\n",
		},
		"tag directive with a comment": {
			source: "%TAG !e! tag:example.com,2000:app/\n# note\n---\n!e!foo bar\n",
			want:   "%TAG !e! tag:example.com,2000:app/\n# note\n---\n!e!foo bar\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(test.source), parser.WithComments())
			require.NoError(t, err)
			assert.Equal(t, test.want, file.String())

			// The same source has to parse whether or not comments are read.
			_, err = parser.ParseBytes([]byte(test.source))
			assert.NoError(t, err)
		})
	}
}

// TestParseDirectiveWithoutDocument keeps the check the comment handling had to
// step around: a directive still has to be followed by a document.
func TestParseDirectiveWithoutDocument(t *testing.T) {
	sources := map[string]string{
		"nothing after it":     "%YAML 1.2\n",
		"only a comment":       "%YAML 1.2\n# note\n",
		"content, no header":   "%YAML 1.2\nfoo: bar\n",
		"comment then content": "%YAML 1.2\n# note\nfoo: bar\n",
	}

	for name, source := range sources {
		t.Run(name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(source), parser.WithComments())
			assert.Errorf(t, err, "accepted %q", source)
		})
	}
}
