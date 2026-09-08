// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner_test

import (
	"iter"
	"slices"
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/scanner"
	yamltestsuite "github.com/go-openapi/go-yaml/internal/yamltestsuite"
	"github.com/go-openapi/go-yaml/token"
)

// maxScanCalls bounds the scanning loop, so a scanner that stops making progress fails the test instead of running
// until the timeout.
const maxScanCalls = 1 << 16

// scanAll drives a scanner to exhaustion and returns every token it read.
//
// A refusal ends the scan, and the tokens read before it are the measurement: a ledger counts what the parser sees.
func scanAll(t *testing.T, src string) []token.Token {
	t.Helper()

	var s scanner.Scanner
	s.Init([]byte(src))

	var tokens []token.Token
	for calls := 0; ; calls++ {
		require.Lessf(t, calls, maxScanCalls,
			"NextToken did not terminate after %d calls on %q", maxScanCalls, src)

		tk, ok := s.NextToken()
		if !ok {
			break
		}
		tokens = append(tokens, tk)
	}

	return tokens
}

// yamlTests returns the YAML Test Suite cases, which both suite-wide ledgers measure over.
func yamlTests(t *testing.T) iter.Seq[*yamltestsuite.TestSuite] {
	t.Helper()

	tests, err := yamltestsuite.TestSuites()
	require.NoError(t, err)
	require.NotEmpty(t, tests)

	return slices.Values(tests)
}
