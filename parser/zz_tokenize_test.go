package parser_test

import (
	"testing"

	"github.com/go-openapi/go-yaml/lexer"
	"github.com/go-openapi/go-yaml/token"
)

// tokenize scans src, failing the test where the scanner refuses it.
func tokenize(t *testing.T, src string) token.Tokens {
	t.Helper()

	return mustTokens(t, src)
}

// mustTokens scans src for a test or a benchmark.
func mustTokens(tb testing.TB, src string) token.Tokens {
	tb.Helper()

	tokens, err := lexer.Tokenize(src)
	if err != nil {
		tb.Fatalf("Tokenize(%q): %v", src, err)
	}

	return tokens
}
