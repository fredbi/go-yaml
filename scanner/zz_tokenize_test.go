package scanner_test

import (
	"testing"

	"github.com/go-openapi/go-yaml/scanner"
	"github.com/go-openapi/go-yaml/token"
)

// scanTokens scans src and returns the tokens it holds, or the error the
// scanner stopped on.
//
// A scanner hands out one token at a time; collecting them is what a test that
// compares a whole document needs, and nothing else should.
func scanTokens(src string) (token.Tokens, error) {
	var s scanner.Scanner
	s.Init(src)

	var tokens token.Tokens
	for tk := range s.All() {
		tokens = append(tokens, tk)
	}

	return tokens, s.Err()
}

// tokenize scans src, failing the test where the scanner refuses it.
func tokenize(t *testing.T, src string) token.Tokens {
	t.Helper()

	tokens, err := scanTokens(src)
	if err != nil {
		t.Fatalf("scanning %q: %v", src, err)
	}

	return tokens
}
