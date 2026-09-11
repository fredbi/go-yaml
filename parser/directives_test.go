// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser"
)

// TestATabSeparatesADirectivesParameters checks that a directive line written with a tab reads as the same line
// written with a space.
//
// The scanner dropped the tab, so "%YAML\t1.1" named a directive "YAML1.1" and the document was read as 1.2,
// and "%YAML 1\t.1" read as the version 1.1. go.yaml.in/yaml/v3 reads the first as 1.1 and refuses the second.
func TestATabSeparatesADirectivesParameters(t *testing.T) {
	for _, tc := range []struct {
		src     string
		want    any
		refused string
	}{
		// 1.1 reads "010" as the octal 8, and 1.2 as the decimal 10.
		{src: "%YAML\t1.1\n---\nk: 010\n", want: uint64(8)},
		{src: "%YAML 1.1\t# c\n---\nk: 010\n", want: uint64(8)},
		{src: "%TAG\t!e!\ttag:yaml.org,2002:\n---\nk: !e!str x\n"},
		{src: "%YAML 1\t.1\n---\nk: 010\n", refused: "unexpected format YAML directive"},
		{src: "%TAG !e! t\tg:yaml.org,2002:\n---\nk: 1\n", refused: "unexpected format TAG directive"},
	} {
		for _, src := range []string{tc.src, strings.ReplaceAll(tc.src, "\t", " ")} {
			_, err := parser.ParseBytes([]byte(src))
			if tc.refused != "" {
				require.Errorf(t, err, "%q", src)
				assert.Containsf(t, err.Error(), tc.refused, "%q", src)

				continue
			}

			require.NoErrorf(t, err, "%q", src)
			if tc.want != nil {
				assert.Equalf(t, tc.want, firstValue(t, src), "%q", src)
			}
		}
	}
}

// TestParseCommentsAroundDirectives covers comment lines between a directive and the '---' that opens the document.
//
// Such comments belong to neither, and the document must parse whether or not comments are read.
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

// TestParseDirectiveWithoutDocument checks that a directive not followed by a "---" document is rejected,
// even with a comment after it.
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
