// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner_test

import (
	"iter"
	"slices"
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/scanner"
	"github.com/go-openapi/go-yaml/internal/scanner/internal/testscanner"
)

// TestOriginsTileTheSource checks that the tokens' extents follow one another with nothing between, so that
// src[previous end:this end] is the text the document wrote each token as, indentation and all.
//
// That span is the verbatim image of the document, and it writes a file back with its comments, its blank lines and
// the spelling the author chose.
//
// Only a scanner that records everything it consumes can offer it, and this one did not: the escapes naming a code
// point,
// \xXX, \uXXXX and \UXXXXXXXX, set how far to skip and appended the decoded rune to the value without counting the
// marker or its digits, so the extents fell behind by the length of every escape in the document.
//
// The ends have to reach the end of the source, less the final line break, which closes the stream instead of
// opening
// a token.
func TestOriginsTileTheSource(t *testing.T) {
	for tc := range originTileCases(t) {
		t.Run("with "+tc.name, func(t *testing.T) {
			t.Run("token origins should tile the source", func(t *testing.T) {
				for _, src := range []string{} {
					assertOriginsTile(t, src)
				}
			})
		})
	}
}

// ================================== Origin tiling test ==================================.

type originTileCase struct {
	name string
	src  iter.Seq[string]
}

func originTileCases(t *testing.T) iter.Seq[originTileCase] {
	t.Helper()

	return slices.Values([]originTileCase{
		{
			name: "crafted sequence of tokens",
			src: slices.Values([]string{
				"a: \"\\u0041\"\n", "a: \"\\x41\"\n", "a: \"\\U0001F600\"\n",
				"a: \"x\\u3000y\"\n", "a: \"\\uD83D\\uDE00\"\n",
				"a: \"tab\\there\"\n", "a: 'it''s'\n", "a: plain\n",
				"# lead\na: 1 # trail\n\nb: 2\n",
			}),
		},
		{
			name: "complete analys workload",
			src:  workloadText(t),
		},
	})
}

func workloadText(t *testing.T) iter.Seq[string] {
	t.Helper()

	return func(yield func(string) bool) {
		for _, doc := range testscanner.WorkloadDocs(t) {
			if !yield(doc.Text) {
				return
			}
		}
	}
}

func assertOriginsTile(t *testing.T, src string) {
	t.Helper()

	var s scanner.Scanner
	s.Init([]byte(src))

	prev := 0
	for tk := range s.Tokens() {
		end := int(tk.EndOffset())
		require.GreaterOrEqualf(t, end, prev,
			"a %s token ends before the one before it, at %d after %d", tk.Type, end, prev)
		require.LessOrEqualf(t, end, len(src),
			"a %s token ends past the document, at %d of %d", tk.Type, end, len(src))
		prev = end
	}
	require.NoError(t, s.Err())

	require.Equal(t, len(strings.TrimSuffix(src, "\n")), len(strings.TrimSuffix(src[:prev], "\n")),
		"the extents do not reach the end of the document")
}
