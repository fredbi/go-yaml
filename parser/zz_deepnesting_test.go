// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"strings"
	"testing"
	"time"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser"
)

// TestNestingCostStaysLinear checks a document of nothing but open brackets
// costs time in proportion to its length.
//
// keyWindow.release used to copy the whole window onto itself and take zero off
// every opener on every token, because nothing may be handed on while a flow
// collection is open: that collection may yet close and stand as a key. One
// token of work per token held is quadratic, and 400,000 brackets -- an 800 KB
// document -- took 67 seconds and a gigabyte.
//
// Doubling the nesting should roughly double the time. The bound here is loose
// on purpose: it is watching for the return of an exponent, not timing the
// parser.
func TestNestingCostStaysLinear(t *testing.T) {
	if testing.Short() {
		t.Skip("times a parse")
	}

	cost := func(depth int) time.Duration {
		src := []byte(strings.Repeat("[", depth) + strings.Repeat("]", depth))

		start := time.Now()
		_, err := parser.ParseBytes(src)
		require.NoError(t, err)

		return time.Since(start)
	}

	// Warm the allocator so the first measurement is not the odd one.
	cost(2000)

	small, large := cost(25_000), cost(100_000)
	t.Logf("25,000 deep: %v; 100,000 deep: %v; ratio %.1f for 4x the input",
		small, large, float64(large)/float64(small))

	// Four times the input, quadratic would be sixteen times the work. Ten
	// leaves room for a slow machine and a noisy sample.
	require.Lessf(t, large, 10*small,
		"four times the nesting took %v against %v, which is the shape of a quadratic parse", large, small)
}
