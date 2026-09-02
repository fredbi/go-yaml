// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner_test

import (
	"errors"
	"io"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	yamltestsuite "github.com/go-openapi/go-yaml/internal/yamltestsuite"
	"github.com/go-openapi/go-yaml/parser/scanner"
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

// Scan, Next, Tokens and NextToken read the same source through the same scan:
// in a batch, one pointer at a time, pushed by value and pulled by value. Up to
// a refusal -- past which only Scan reads on -- all four have to agree token for
// token.
func TestScanAndNextAgree(t *testing.T) {
	tests, err := yamltestsuite.TestSuites()
	require.NoError(t, err)
	require.NotEmpty(t, tests)

	var compared int
	for _, test := range tests {
		src := string(test.InYAML)

		var batch scanner.Scanner
		batch.Init(src)
		tks, batchErr := batch.Scan()
		if errors.Is(batchErr, io.EOF) {
			// The source held no token at all, which is not a refusal.
			batchErr = nil
		}

		var one scanner.Scanner
		one.Init(src)
		var pulled token.Tokens
		for tk := range one.All() {
			pulled = append(pulled, tk)
		}

		var byValue scanner.Scanner
		byValue.Init(src)
		var pushed []token.Token
		for tk := range byValue.Tokens() {
			pushed = append(pushed, tk)
		}

		var oneValue scanner.Scanner
		oneValue.Init(src)
		var pulledValues []token.Token
		for {
			tk, ok := oneValue.NextToken()
			if !ok {
				break
			}
			pulledValues = append(pulledValues, tk)
		}

		require.Lenf(t, pulled, len(tks), "%s: Next yielded %d tokens, Scan %d", test.Name, len(pulled), len(tks))
		require.Lenf(t, pushed, len(tks), "%s: Tokens yielded %d tokens, Scan %d", test.Name, len(pushed), len(tks))
		require.Lenf(t, pulledValues, len(tks), "%s: NextToken yielded %d tokens, Scan %d", test.Name, len(pulledValues), len(tks))
		for i := range tks {
			assert.Equalf(t, *tks[i], *pulled[i], "%s: token %d differs from Next", test.Name, i)
			assert.Equalf(t, *tks[i], pushed[i], "%s: token %d differs from Tokens", test.Name, i)
			assert.Equalf(t, *tks[i], pulledValues[i], "%s: token %d differs from NextToken", test.Name, i)
		}
		if batchErr != nil {
			require.EqualErrorf(t, one.Err(), batchErr.Error(), "%s: the refusals differ", test.Name)
		} else {
			require.NoErrorf(t, one.Err(), "%s: Next refused a source Scan accepted", test.Name)
		}
		compared += len(tks)
	}

	t.Logf("compared %d tokens over %d cases", compared, len(tests))
}

// Breaking out of Tokens leaves the scanner on the token after the one the loop
// stopped on, so reading on picks the stream up where it was left.
func TestTokensResumesAfterBreak(t *testing.T) {
	const src = "a: 1\nb: 2\nc: 3\n"

	var whole scanner.Scanner
	whole.Init(src)
	var want []token.Token
	for tk := range whole.Tokens() {
		want = append(want, tk)
	}
	require.Greater(t, len(want), 6)

	var s scanner.Scanner
	s.Init(src)

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
