// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner_test

import (
	"errors"
	"io"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/scanner"
	"github.com/go-openapi/go-yaml/token"
)

// A token's BlankLineAbove and CommentBreaksAbove are filled in as it is
// emitted, from the tokens emitted before it, so the answers are carried on the
// Scanner rather than found by walking back over the tokens already read. Init
// has to drop what the last source left there.
func TestScannerLookbackDoesNotLeakBetweenSources(t *testing.T) {
	const src = `# one
# two

a: 1

b: |
  first

  third

c: 2
`

	scanAll := func(s *scanner.Scanner) token.Tokens {
		t.Helper()

		var all token.Tokens
		for {
			tks, err := s.Scan()
			all = append(all, tks...)
			if errors.Is(err, io.EOF) {
				return all
			}
			require.NoError(t, err)
		}
	}

	var s scanner.Scanner

	s.Init(src)
	first := scanAll(&s)
	require.NotEmpty(t, first)

	// The same Scanner, told to read the same source again.
	s.Init(src)
	second := scanAll(&s)

	require.Len(t, second, len(first))
	for i := range first {
		assert.Equalf(t, first[i].BlankLineAbove, second[i].BlankLineAbove,
			"token %d (%s %q): BlankLineAbove differs on the second Init", i, first[i].Type, first[i].Value)
		assert.Equalf(t, first[i].CommentBreaksAbove, second[i].CommentBreaksAbove,
			"token %d (%s %q): CommentBreaksAbove differs on the second Init", i, first[i].Type, first[i].Value)
	}

	// And the source really does exercise both fields, or the check above
	// compares nothing.
	var blanks, breaks int
	for _, tk := range first {
		if tk.BlankLineAbove {
			blanks++
		}
		if tk.CommentBreaksAbove > 0 {
			breaks++
		}
	}
	assert.NotZerof(t, blanks, "expected the source to leave a blank line above some token")
	assert.NotZerof(t, breaks, "expected the source to put a comment above some token")
}
