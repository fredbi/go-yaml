// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner_test

import (
	"fmt"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/fuzzseeds"
	"github.com/go-openapi/go-yaml/internal/scanner"
	"github.com/go-openapi/go-yaml/internal/yamltestsuite"
	"github.com/go-openapi/go-yaml/token"
)

// unindentedFlow continues a flow mapping on a line that is not indented past the line it started on,
// which a new Scanner rejects.
const unindentedFlow = "k: {\nk\n:\nv\n}"

// TestAScannerReadsAfterAFlowCollectionItLeftOpen checks that Init clears the flow state of the previous scan:
// after a scan that stopped inside a "{" or a "[", the next source reads as it does with a new Scanner.
func TestAScannerReadsAfterAFlowCollectionItLeftOpen(t *testing.T) {
	t.Parallel()

	want, wantErr := scanEvery(new(scanner.Scanner), unindentedFlow)
	require.Error(t, wantErr)

	for _, previous := range []struct {
		src  string
		stop int // tokens read before the scan stops, 0 for all of them
	}{
		{"{a", 0},
		{"[a", 0},
		{"{a: [b", 0},
		{"{a: [b, c]}", 2},
		{"k: {a: 1}", 3},
	} {
		t.Run(fmt.Sprintf("%q after %d tokens", previous.src, previous.stop), func(t *testing.T) {
			t.Parallel()

			var s scanner.Scanner
			scanUntil(&s, previous.src, previous.stop)

			got, gotErr := scanEvery(&s, unindentedFlow)
			assert.Equal(t, want, got)
			assert.Equal(t, errText(wantErr), errText(gotErr))
		})
	}
}

// TestAReusedScannerReadsAsANewOne scans every YAML Test Suite document and fuzz seed with one Scanner,
// the previous scan stopped after a varying number of tokens, and compares each result with a new Scanner's:
// every token, and the error that stopped the scan.
func TestAReusedScannerReadsAsANewOne(t *testing.T) {
	t.Parallel()

	suites, err := yamltestsuite.TestSuites()
	require.NoError(t, err)
	seeds, err := fuzzseeds.All()
	require.NoError(t, err)

	texts := make([]string, 0, len(suites)+len(seeds))
	for _, s := range suites {
		texts = append(texts, string(s.InYAML))
	}
	texts = append(texts, seeds...)

	var reused scanner.Scanner
	for i, text := range texts {
		if i > 0 {
			scanUntil(&reused, texts[i-1], i%7)
		}

		want, wantErr := scanEvery(new(scanner.Scanner), text)
		got, gotErr := scanEvery(&reused, text)
		if !assert.Equalf(t, want, got, "source %d: tokens after a reused Init", i) ||
			!assert.Equalf(t, errText(wantErr), errText(gotErr), "source %d: error after a reused Init", i) {
			return
		}
	}
}

// TestResetLeavesANewScanner checks that a Scanner Reset part way through a source scans nothing,
// as a new Scanner does, and scans the next source Init gives it as a new Scanner does.
func TestResetLeavesANewScanner(t *testing.T) {
	t.Parallel()

	var s scanner.Scanner
	scanUntil(&s, "{a: [b, c]}", 2)
	s.Reset()

	_, more := s.NextToken()
	assert.False(t, more)
	require.NoError(t, s.Err())

	want, wantErr := scanEvery(new(scanner.Scanner), unindentedFlow)
	got, gotErr := scanEvery(&s, unindentedFlow)
	assert.Equal(t, want, got)
	assert.Equal(t, errText(wantErr), errText(gotErr))
}

// scanEvery returns every token s reads from src, and the error that stopped the scan.
func scanEvery(s *scanner.Scanner, src string) ([]token.Token, error) {
	s.Init([]byte(src))

	var out []token.Token
	for {
		tk, ok := s.NextToken()
		if !ok {
			return out, s.Err()
		}
		out = append(out, tk)
	}
}

// scanUntil reads src with s until stop tokens are read, or to the end when stop is 0, and leaves the scan there.
func scanUntil(s *scanner.Scanner, src string, stop int) {
	s.Init([]byte(src))
	for n := 0; stop == 0 || n < stop; n++ {
		if _, ok := s.NextToken(); !ok {
			return
		}
	}
}

// errText returns the message of err, and "" for nil.
func errText(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}
