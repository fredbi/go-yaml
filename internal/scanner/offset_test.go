// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	yamltestsuite "github.com/go-openapi/go-yaml/internal/yamltestsuite"
	"github.com/go-openapi/go-yaml/token"
)

// sourceText is a token's text as it stands in the source: the text it was
// written as, without the whitespace and line breaks in front of it.
func sourceText(origin string) string {
	return strings.TrimLeft(origin, " \t\n\r")
}

// at returns the source from offset onwards.
//
// Token.Position.Offset() is a 0-based byte index into the source as it was
// handed in, byte order marks included: the scanner steps over a mark rather
// than deleting it. This is the one place the test encodes what an Offset
// means.
func at(src string, offset int) string {
	if offset < 0 || offset > len(src) {
		return ""
	}

	return src[offset:]
}

// offsetMissLedger records how many tokens of each type carry an Offset that
// does not address their own text, over the whole YAML Test Suite.
//
// 101 of 3,489, which is 97.1% correct. It was 1,031 until three places in the
// scanner stopped stepping over a character without counting its byte:
// scanTag over the '!', scanComment over the '#', and scanMultiLineHeaderOption
// over the '|' or '>'. Each left s.offset one byte behind ctx.idx for the rest
// of the document, so every token after the first tag, comment or block scalar
// was reported that many bytes early. Twenty-one of the twenty-five types now
// miss nothing at all.
//
// What is left is 25 of 3,489, and none of it is the counter drifting. 17 are
// Invalid, the tokens an error carries, built from the whole origin buffer
// rather than from one token's worth of it. 7 are multi-line String values --
// block scalar content -- and one is a Comment.
//
// It was 33 while the comparison ran against Token.Origin. That field held the
// scanner's buffer, which is not always the document: Context.removeRightSpaceFromBuf
// trims the spaces a line ends with from the origin as well as from the value,
// so "a: one \n  two" -- a plain scalar continued over two lines, the first
// ending in a space -- had an Origin of "a: one\n  two", which the document does
// not contain. Reading the text back from the extents compares against the
// document itself, and three of the types stopped missing anything at all.
//
// Line and Column were right throughout, which is what made the drift hard to
// see: 3,287 of 3,489 columns address their token.
//
// The ledger is a ratchet in both directions. A type that starts missing more
// fails as a regression; one that starts missing fewer fails too, and the fix
// is recorded by lowering the count.
var offsetMissLedger = map[string]int{
	"Invalid": 17,
	"String":  7,
	"Comment": 1,
}

// TestTokenOffsetsAddressTheSource measures, over the YAML Test Suite, how
// often a token's Offset addresses that token in the source.
func TestTokenOffsetsAddressTheSource(t *testing.T) {
	tests, err := yamltestsuite.TestSuites()
	require.NoError(t, err)
	require.NotEmpty(t, tests)

	missed := make(map[string]int)
	var total int

	for _, test := range tests {
		src := string(test.InYAML)
		tokens := scanAll(t, src)
		for i, tk := range tokens {
			want := sourceText(originsOf(src, tokens)[i])
			if want == "" {
				continue
			}
			total++
			if !strings.HasPrefix(at(src, int(tk.Position.Offset())), want) {
				missed[tk.Type.String()]++
			}
		}
	}

	require.NotZero(t, total)

	names := make([]string, 0, len(missed)+len(offsetMissLedger))
	for name := range missed {
		names = append(names, name)
	}
	for name := range offsetMissLedger {
		if _, seen := missed[name]; !seen {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		got, want := missed[name], offsetMissLedger[name]
		switch {
		case want == 0:
			assert.Zerof(t, got, "%s: %d tokens gained an offset that misses their text", name, got)
		case got == 0:
			assert.Failf(t, "ledger entry is stale",
				"%s: no longer misses %d offsets -- if that is a fix, delete the entry", name, want)
		default:
			assert.Equalf(t, want, got, "%s: offset misses changed", name)
		}
	}
}

// originsOf reads back the text the document wrote each token as.
//
// [token.Token] does not carry it. The tokens' extents tile the source --
// TestOriginsTileTheSource is where that is checked -- so the text of the token
// at i is the source between the end of the one before it and its own end,
// leading whitespace included.
func originsOf(src string, tokens token.Tokens) []string {
	origins := make([]string, len(tokens))
	prev := 0
	for i, tk := range tokens {
		end := int(tk.EndOffset())
		if end < prev || end > len(src) {
			origins[i] = ""
			prev = min(max(end, prev), len(src))

			continue
		}
		origins[i] = src[prev:end]
		prev = end
	}

	return origins
}
