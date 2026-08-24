// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner_test

import (
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/lexer"
)

// TestOffsetsCountAByteOrderMark checks that a mark the scanner steps over is
// still counted in the offsets after it.
//
// Init used to delete every mark before scanning, so a document carrying one
// was tokenized against a text the caller never wrote and every offset after
// the mark was three bytes short.
func TestOffsetsCountAByteOrderMark(t *testing.T) {
	tests := map[string]struct {
		src  string
		want map[string]int // token value -> byte offset in src
	}{
		"opening the stream": {
			src:  bom + "a: 1\n",
			want: map[string]int{"a": 3, ":": 4, "1": 6},
		},
		"opening a document after a suffix": {
			src:  "a: 1\n...\n" + bom + "---\nb: 2\n",
			want: map[string]int{"...": 5, "---": 12, "b": 16, "2": 19},
		},
		"none at all": {
			src:  "a: 1\n",
			want: map[string]int{"a": 0, ":": 1, "1": 3},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			seen := make(map[string]bool)
			for _, tk := range lexer.Tokenize(test.src) {
				want, ok := test.want[tk.Value]
				if !ok {
					continue
				}
				seen[tk.Value] = true
				assert.Equalf(t, want, tk.Position.Offset, "%q", tk.Value)

				// The offset addresses the source as it was handed in.
				require.LessOrEqual(t, tk.Position.Offset, len(test.src))
				assert.Truef(t, strings.HasPrefix(test.src[tk.Position.Offset:], tk.Value),
					"offset %d does not address %q", tk.Position.Offset, tk.Value)
			}
			for value := range test.want {
				assert.Truef(t, seen[value], "no token for %q", value)
			}
		})
	}
}
