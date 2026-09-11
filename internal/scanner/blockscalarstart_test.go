// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner_test

import (
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/token"
)

// TestABlockOfSpacesStartsAtItsFirstContentSpace checks the offset of a block
// scalar whose content is nothing but spaces past the indentation, cut where a
// less indented line opens.
//
// The offset was counted back from the cursor, which had read that line's
// indentation by then, and the value -- a run of spaces -- matched inside it.
// "|6-" over a 13-space line over "      # c" put its content on the comment's
// line, so a rendering that removed the comment could not take the line whole
// and the next entry moved two columns.
func TestABlockOfSpacesStartsAtItsFirstContentSpace(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want int32
	}{
		// The 13th space of line 3 stands at offset 27, past the 12 columns of
		// indentation "- |6-" at column 7 asks for.
		{src: "k:\n      - |6-\n" + strings.Repeat(" ", 13) + "\n\n\n      # c\n      - x\n", want: 27},
		{src: "k:\n      - |6-\n" + strings.Repeat(" ", 13) + "\n      # c\n      - x\n", want: 27},
		{src: "k:\n  - |6-\n" + strings.Repeat(" ", 9) + "\n\n\n  # c\n  - y\n", want: 19},
		// A line that opens at column 1 never moved it.
		{src: "- |1-\n  \n\n- x\n", want: 7},
	} {
		tokens, err := scanTokens(tc.src)
		require.NoErrorf(t, err, "%q", tc.src)

		var content *token.Token
		for i := range tokens {
			if tokens[i].Type == token.StringType && tokens[i].Value == " " {
				content = &tokens[i]

				break
			}
		}
		require.NotNilf(t, content, "%q holds a block of spaces", tc.src)
		assert.Equalf(t, tc.want, content.Position.Offset(), "%q", tc.src)
		assert.Equalf(t, " ", tc.src[tc.want:tc.want+1], "the offset names a space of %q", tc.src)
	}
}
