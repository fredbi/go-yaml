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
// Token.Position.Offset is a 0-based byte index. It still addresses the source
// with every byte order mark removed rather than the source as handed in, which
// is the remaining half of the defect: Init rewrites the text before scanning
// it. This is the one place the test encodes what an Offset means.
func at(src string, offset int) string {
	stripped := strings.ReplaceAll(src, bom, "")
	if offset < 0 || offset > len(stripped) {
		return ""
	}

	return stripped[offset:]
}

// offsetMissLedger records how many tokens of each type carry an Offset that
// does not address their own text, over the whole YAML Test Suite.
//
// Offset points at the start of Origin, and Origin holds the whitespace written
// before the token as well as the token, so an indented token is reported at
// the start of its indentation. A consumer drawing a caret under an error puts
// it in the wrong column. 1,031 of 3,489 tokens are affected.
//
// Offset counting bytes rather than runes was the other half of the defect and
// is fixed; this half is not. The counts barely moved when the unit changed,
// which is the point: the suite is almost entirely ASCII, so the two defects
// were always independent.
//
// The ledger is a ratchet in both directions. A type that starts missing more
// fails as a regression; one that starts missing fewer fails too, and the fix
// is recorded by lowering the count.
var offsetMissLedger = map[string]int{
	"String":         404,
	"MappingValue":   148,
	"SequenceEntry":  84,
	"Comment":        68,
	"Tag":            59,
	"Integer":        41,
	"DocumentHeader": 31,
	"Anchor":         31,
	"DoubleQuote":    25,
	"Invalid":        23,
	"Literal":        21,
	"Folded":         14,
	"DocumentEnd":    14,
	"CollectEntry":   12,
	"MappingKey":     10,
	"Float":          10,
	"Directive":      8,
	"Alias":          7,
	"MappingEnd":     6,
	"SequenceEnd":    5,
	"MappingStart":   4,
	"SequenceStart":  3,
	"Bool":           1,
	"HexInteger":     1,
	"SingleQuote":    1,
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
			require.NotNil(t, tk.Position)
			want := sourceText(tk.Origin)
			if want == "" {
				continue
			}
			total++
			if !strings.HasPrefix(at(src, tk.Position.Offset), want) {
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
