// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/scanner"
	yamltestsuite "github.com/go-openapi/go-yaml/internal/yamltestsuite"
	"github.com/go-openapi/go-yaml/token"
)

// A token's BlankLineAbove and CommentBreaksAbove are filled in as it is emitted, from the tokens emitted before it, so
// the Scanner carries the answers instead of walking back over the tokens already read.
//
// Init has to drop what the last source left there.
func TestScannerLookbackDoesNotLeakBetweenSources(t *testing.T) {
	const src = `# one
# two

a: 1

b: |
  first

  third

c: 2
`

	readAll := func(s *scanner.Scanner) []token.Token {
		t.Helper()

		var all []token.Token
		for {
			tk, ok := s.NextToken()
			if !ok {
				require.NoError(t, s.Err())

				return all
			}
			all = append(all, tk)
		}
	}

	var s scanner.Scanner

	s.Init([]byte(src))
	first := readAll(&s)
	require.NotEmpty(t, first)

	// The same Scanner, told to read the same source again.
	s.Init([]byte(src))
	second := readAll(&s)

	require.Len(t, second, len(first))
	for i := range first {
		assert.Equalf(t, first[i].BlankLineAbove(), second[i].BlankLineAbove(),
			"token %d (%s %q): BlankLineAbove differs on the second Init", i, first[i].Type, first[i].Value)
		assert.Equalf(t, first[i].CommentBreaksAbove(), second[i].CommentBreaksAbove(),
			"token %d (%s %q): CommentBreaksAbove differs on the second Init", i, first[i].Type, first[i].Value)
	}

	// And the source really does exercise both fields, or the check above compares nothing.
	var blanks, breaks int
	for _, tk := range first {
		if tk.BlankLineAbove() {
			blanks++
		}
		if tk.CommentBreaksAbove() > 0 {
			breaks++
		}
	}
	assert.NotZerof(t, blanks, "expected the source to leave a blank line above some token")
	assert.NotZerof(t, breaks, "expected the source to put a comment above some token")
}

// Scan, Next, Tokens and NextToken read the same source through the same scan: by pulling one at a time and by being
// pushed them, which are the two ways a caller drives the scanner.
//
// They have to agree token for token, and on the refusal that ends them.
func TestPushAndPullAgree(t *testing.T) {
	tests, err := yamltestsuite.TestSuites()
	require.NoError(t, err)
	require.NotEmpty(t, tests)

	var compared int
	for _, test := range tests {
		src := string(test.InYAML)

		var pushing scanner.Scanner
		pushing.Init([]byte(src))
		var pushed []token.Token
		for tk := range pushing.Tokens() {
			pushed = append(pushed, tk)
		}

		var pulling scanner.Scanner
		pulling.Init([]byte(src))
		var pulled []token.Token
		for {
			tk, ok := pulling.NextToken()
			if !ok {
				break
			}
			pulled = append(pulled, tk)
		}

		require.Lenf(t, pushed, len(pulled),
			"%s: Tokens yielded %d, NextToken %d", test.Name, len(pushed), len(pulled))
		for i := range pulled {
			assert.Equalf(t, pulled[i], pushed[i], "%s: token %d differs", test.Name, i)
		}

		switch {
		case pulling.Err() == nil:
			assert.NoErrorf(t, pushing.Err(), "%s: Tokens refused the document and NextToken did not", test.Name)
		default:
			require.EqualErrorf(t, pushing.Err(), pulling.Err().Error(), "%s: the refusals differ", test.Name)
		}

		compared++
	}

	require.NotZero(t, compared)
}

// Breaking out of Tokens leaves the scanner on the token after the one the loop stopped on, so reading on picks the
// stream up where it was left.
func TestTokensResumesAfterBreak(t *testing.T) {
	const src = "a: 1\nb: 2\nc: 3\n"

	var whole scanner.Scanner
	whole.Init([]byte(src))
	var want []token.Token
	for tk := range whole.Tokens() {
		want = append(want, tk)
	}
	require.Greater(t, len(want), 6)

	var s scanner.Scanner
	s.Init([]byte(src))

	var got []token.Token
	for tk := range s.Tokens() {
		got = append(got, tk)
		if len(got) == 3 {
			break
		}
	}
	require.Len(t, got, 3)

	// Read on, both ways, to check neither loses nor repeats a token.
	tk, ok := s.NextToken()
	require.True(t, ok)
	got = append(got, tk)

	for tk := range s.Tokens() {
		got = append(got, tk)
	}

	require.NoError(t, s.Err())
	require.Len(t, got, len(want))
	for i := range want {
		assert.Equalf(t, want[i], got[i], "token %d differs after the break", i)
	}
}
