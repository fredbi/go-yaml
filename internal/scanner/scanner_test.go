// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner_test

import (
	"testing"

	"github.com/go-openapi/go-yaml/internal/scanner"
	"github.com/go-openapi/go-yaml/token"
	"github.com/go-openapi/testify/v2/require"
)

// maxScanCalls bounds the scanning loop.
//
// Scanner.Scan only signals completion with io.EOF.
//
// A caller that keeps calling it after any other error relies on the scanner always making progress or on its internal
// bounds.
//
// Currently, nothing enforces that, so the bound turns a hypothetical non-progressing loop into a test failure rather
// than a fuzzing timeout.
//
// TODO(fred): enforce a scanner bound??
const maxScanCalls = 1 << 16

// Doc is a convenience type to share utilities that accept either a string or []byte.
type Doc interface {
	string | []byte
}

// scanAll drives a Scanner to exhaustion and reports whether it terminated on its own.
//
// It materialize all tokens so a test can reason about the entire collection.
func scanAll[V Doc](t *testing.T, src V) []token.Token {
	t.Helper()

	var s scanner.Scanner
	s.Init([]byte(src))
	tokens := make([]token.Token, 0, estimateTokens(src))

	for calls := 0; ; calls++ {
		require.Lessf(t, calls, maxScanCalls,
			"NextToken did not terminate after %d calls on %q",
			maxScanCalls, src,
		)

		tk, ok := s.NextToken()
		if !ok {
			break
		}
		held := tk
		tokens = append(tokens, held)
	}

	return tokens
}

// scanTokens scans src and returns the tokens it holds, or the error the scanner stopped on.
//
// A scanner hands out one token at a time; collecting them is what a test that compares a whole document needs, and
// nothing else should.
func scanTokens[V Doc](src V) ([]token.Token, error) {
	var s scanner.Scanner
	s.Init([]byte(src))

	tokens := make([]token.Token, 0, estimateTokens(src))
	for tk := range s.Tokens() { // NOTE(fred): at this moment, the Tokens() iterator is not being used - candidate for removal
		held := tk
		tokens = append(tokens, held)
	}

	return tokens, s.Err()
}

// tokenize scans src, failing the test where the scanner refuses it.
func tokenize(t *testing.T, src string) []token.Token {
	t.Helper()

	const maxPrint = 512 // limit the output when errors are reported
	tokens, err := scanTokens(src)
	require.NoErrorf(t, err, "scanning %q: %v", src[:min(len(src), maxPrint)], err)

	return tokens
}

func estimateTokens[V Doc](src V) int {
	return len(src) / 4 // TODO(fred): find a better heuristic
}
