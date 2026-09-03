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
)

// sourceText is a token's text as it stands in the source: Origin without the
// whitespace and line breaks written before it.
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
// What is left splits in two, and neither part is the counter drifting:
//
//   - 53 tokens carry an offset that addresses somewhere else in the source.
//     21 are multi-line String values -- block scalar content -- and 23 are
//     Invalid, the tokens an error carries, built from the whole origin buffer.
//     It was 65 and 23 until the scanner recorded where a block scalar's
//     content begins instead of cutting the token at the end of the block and
//     asking the cursor: MultiLineState.began marks the first byte of content
//     read, and MultiLineState.from hands it back when the token is built.
//     Working it out afterwards cannot succeed, folding making the value
//     shorter than the source it came from, and ctx.originStart does not track
//     through a multi-line block.
//   - 12 tokens carry an Origin that is not a slice of the source at all, so
//     no offset can address it: trailing whitespace the origin buffer dropped,
//     and escapes a double-quoted scalar rewrote.
//
// Line and Column were right throughout, which is what made the drift hard to
// see: 3,287 of 3,489 columns address their token.
//
// The ledger is a ratchet in both directions. A type that starts missing more
// fails as a regression; one that starts missing fewer fails too, and the fix
// is recorded by lowering the count.
var offsetMissLedger = map[string]int{
	"String":      27,
	"Invalid":     23,
	"Integer":     3,
	"DoubleQuote": 1,
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
		for _, tk := range scanAll(t, src) {
			want := sourceText(tk.Origin)
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
