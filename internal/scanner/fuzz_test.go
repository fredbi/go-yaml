package scanner_test

import (
	"errors"
	"io"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/fuzzseeds"
	"github.com/go-openapi/go-yaml/internal/scanner"
	"github.com/go-openapi/go-yaml/token"
)

// maxScanCalls bounds the scanning loop.
//
// Scanner.Scan only signals completion with io.EOF, so a caller that keeps
// calling it after any other error relies on the scanner always making
// progress. Nothing enforces that, so the
// bound turns a hypothetical non-progressing loop into a test failure rather
// than a fuzzing timeout.
const maxScanCalls = 1 << 16

// scanAll drives a Scanner to exhaustion and reports whether it terminated on
// its own.
func scanAll(t *testing.T, src string) token.Tokens {
	t.Helper()

	var (
		s      scanner.Scanner
		tokens token.Tokens
	)
	s.Init(src)

	for calls := 0; ; calls++ {
		require.Lessf(t, calls, maxScanCalls,
			"Scan did not terminate after %d calls on %q", maxScanCalls, src)

		subTokens, err := s.Scan()
		if errors.Is(err, io.EOF) {
			break
		}
		tokens.Add(subTokens...)
	}

	return tokens
}

// assertTokenInvariants checks the properties that hold for every token the
// scanner emits, whatever the input.
//
// Deliberately absent: Position.Column >= 1 and monotonically advancing
// positions. Both are violated today by block scalars and by tabs that look
// like indentation -- see TestPositionLedger, which pins the exact extent of
// those violations so that a fix is noticed.
func assertTokenInvariants(t *testing.T, tokens token.Tokens, src string) {
	t.Helper()

	for i, tk := range tokens {
		require.NotNilf(t, tk, "token %d is nil for %q", i, src)
		require.NotNilf(t, tk.Position, "token %d has no position for %q", i, src)

		assert.GreaterOrEqualf(t, int(tk.Position.Line), 1,
			"token %d (%v) has line %d for %q", i, tk.Type, tk.Position.Line, src)
		// Offset is a 0-based byte index into the source, so 0 is the first
		// byte and anything below it addresses nothing.
		assert.GreaterOrEqualf(t, int(tk.Position.Offset()), 0,
			"token %d (%v) has offset %d for %q", i, tk.Type, tk.Position.Offset(), src)
		assert.LessOrEqualf(t, int(tk.Position.Offset()), len(src),
			"token %d (%v) has offset %d past the end of %q", i, tk.Type, tk.Position.Offset(), src)
	}
}

func FuzzScannerScan(f *testing.F) {
	addSuiteSeeds(f)

	f.Fuzz(func(t *testing.T, src string) {
		tokens := scanAll(t, src)
		assertTokenInvariants(t, tokens, src)

		// Scanning is a pure function of the input: the same source must
		// produce the same tokens every time. This is the cheapest instrument
		// we have against the flaky-parse class of report.
		again := scanAll(t, src)
		require.Equalf(t, len(tokens), len(again), "token count is not stable for %q", src)

		for i := range tokens {
			assert.Equalf(t, tokens[i].Origin, again[i].Origin, "token %d origin is not stable for %q", i, src)
			assert.Equalf(t, tokens[i].Value, again[i].Value, "token %d value is not stable for %q", i, src)
			assert.Equalf(t, tokens[i].Type, again[i].Type, "token %d type is not stable for %q", i, src)
		}
	})
}

// addSuiteSeeds seeds a fuzz target with every document of the YAML Test Suite,
// valid and invalid alike, plus the reduced inputs from reports we track.
func addSuiteSeeds(f *testing.F) {
	f.Helper()

	seeds, err := fuzzseeds.All()
	require.NoError(f, err)

	for _, src := range seeds {
		f.Add(src)
	}
}
