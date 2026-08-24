package analysis

import (
	"testing"

	"github.com/go-openapi/go-yaml/lexer"
	"github.com/go-openapi/go-yaml/token"
)

// tokenize scans src, failing the caller where the scanner refuses it.
func tokenize(tb testing.TB, src string) token.Tokens {
	tb.Helper()

	tokens, err := lexer.Tokenize(src)
	if err != nil {
		tb.Fatalf("Tokenize: %v", err)
	}

	return tokens
}
