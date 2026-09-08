// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/scanner"
	"github.com/go-openapi/go-yaml/token"
)

// TestDirectiveOpensALine holds both halves of the rule c-directive states.
//
// A directive opens a line and takes no separation in front of it, so scanDirective guards the '%' with both
// s.indentNum and s.column. Neither counter catches what the other does: a space raises both, a tab raises only
// indentNum, and the '%' in "a: %foo" leaves indentNum at 0.
//
// The reference parser refuses all four indented forms below and accepts the three unindented ones, which is what
// these cases hold the scanner to.
func TestDirectiveOpensALine(t *testing.T) {
	t.Parallel()

	t.Run("a directive opening a line is read", func(t *testing.T) {
		for _, src := range []string{
			"%YAML 1.2\n---\na: 1\n",
			"%TAG ! tag:example.com,2000:\n---\na: 1\n",
			"a: 1\n...\n%YAML 1.2\n---\nb: 2\n",
		} {
			got, err := scanDirectiveTokens(t, src)
			require.NoErrorf(t, err, "%q holds a directive", src)
			assert.Containsf(t, got, token.DirectiveType, "%q opens with a directive", src)
		}
	})

	t.Run("a directive that does not open a line is refused", func(t *testing.T) {
		for _, src := range []string{
			"\t%YAML 1.2\n---\na: 1\n",  // a tab raises indentNum and leaves the column at 1
			"  %YAML 1.2\n---\na: 1\n",  // spaces raise both
			" \t%YAML 1.2\n---\na: 1\n", // and so does a space with a tab behind it
			"a: %foo\n",                 // indentNum is 0 here, and the column refuses it
		} {
			_, err := scanDirectiveTokens(t, src)
			assert.Errorf(t, err, "%q opens no directive", src)
		}
	})
}

func scanDirectiveTokens(t *testing.T, src string) ([]token.Type, error) {
	t.Helper()

	var s scanner.Scanner
	s.Init([]byte(src))

	var got []token.Type
	for tk := range s.Tokens() {
		got = append(got, tk.Type)
	}

	return got, s.Err()
}
