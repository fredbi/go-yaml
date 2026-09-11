// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser"
)

// TestATabIsSeparationAndNotIndentation checks that a tab is rejected in a block entry's indentation
// and admitted as separation.
//
// s-indent(n) is spaces and nothing else, so a tab among a block entry's indentation leaves the entry no indentation.
// A tab is separation, and is admitted wherever separation is:
// in front of a flow node, after a ':', and inside a flow collection.
//
// Scanner.tabStandsWhereAnEntryNeedsIndent decides it, over the two runs that can hold the tab:
// the line's own indentation, and the separation after a token already cut on this line.
func TestATabIsSeparationAndNotIndentation(t *testing.T) {
	t.Run("a block entry is refused, whatever stands in front of the tab", func(t *testing.T) {
		// Each fails with the same message, whatever the space count and whether the key is quoted.
		for _, src := range []string{
			"\ta: 1\n",
			" \ta: 1\n",
			"  \ta: 1\n",
			"   \ta: 1\n",
			"\t\"a\": 1\n",
			" \t\"a\": 1\n",
			"a:\n \tb: 1\n",
			"a:\n  \tb: 1\n",

			// Here the tab stands in the separation after the '-', the second run,
			// not in the line's indentation.
			"- \ta: 1\n",
			"-\ta: 1\n",
		} {
			_, err := parser.ParseBytes([]byte(src))
			require.Errorf(t, err, "%q", src)
			assert.Containsf(t, err.Error(),
				"tab character cannot stand for the indentation a mapping entry needs", "%q", src)
		}
	})

	t.Run("and a sequence entry the same way", func(t *testing.T) {
		for _, src := range []string{"\t- 1\n", " \t- 1\n", "  \t- 1\n", "- \t- 1\n"} {
			_, err := parser.ParseBytes([]byte(src))
			require.Errorf(t, err, "%q", src)
			assert.Containsf(t, err.Error(), "tab character cannot use as a sequence delimiter", "%q", src)
		}
	})

	t.Run("a flow collection admits it, and used to be refused", func(t *testing.T) {
		// A flow collection has no indentation, so a tab inside one is separation.
		for _, src := range []string{
			"{\ta: 1}\n",
			"{ \ta: 1}\n",
			"{a: 1,\tb: 2}\n",
			"{a:\t1}\n",
		} {
			_, err := parser.ParseBytes([]byte(src))
			assert.NoErrorf(t, err, "%q", src)
		}
	})

	t.Run("and separation elsewhere was always allowed", func(t *testing.T) {
		for _, src := range []string{"\t{}\n", "\t{a: 1}\n", "[\t1]\n", "[a,\tb]\n", "a: \tb\n", "a:\t1\n"} {
			_, err := parser.ParseBytes([]byte(src))
			assert.NoErrorf(t, err, "%q", src)
		}
	})
}
