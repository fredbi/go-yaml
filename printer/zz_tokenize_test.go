package printer_test

import (
	"testing"

	"github.com/go-openapi/go-yaml/lexer"
	"github.com/go-openapi/go-yaml/token"
)

// tokenize scans src, failing the test where the scanner refuses it.
func tokenize(t *testing.T, src string) token.Tokens {
	t.Helper()

	tokens, err := lexer.Tokenize(src)
	if err != nil {
		t.Fatalf("Tokenize(%q): %v", src, err)
	}

	return tokens
}
