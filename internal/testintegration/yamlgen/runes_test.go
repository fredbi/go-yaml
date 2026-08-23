// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/go-openapi/testify/v2/assert"

	"github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"
)

// TestTheRuneSetDoesNotMoveWithTheToolchain pins the characters a seed draws.
//
// The stored corpus is the same claim, made 10,214 cases at a time: regenerating
// it under a toolchain whose unicode tables had moved failed on 4,299 lines and
// said only that a line differed. This fails on one digest and names the cause,
// so a rune set that starts reading unicode.Lu again is caught here rather than
// four hundred kilobytes downstream.
//
// A rapid upgrade can move the digest too, since nothing promises Example draws
// the same value across versions. Either way the answer is the same: update the
// digest only alongside the deliberate change, and regenerate both corpora in
// the same commit.
func TestTheRuneSetDoesNotMoveWithTheToolchain(t *testing.T) {
	const want = "e27ac898cefa01995c66a49b3146039cb1c56118233dfb84caf84fa09b5592eb"

	runes := yamlgen.Runes()

	h := sha256.New()
	for seed := range 4096 {
		fmt.Fprintf(h, "%d:%d\n", seed, runes.Example(seed))
	}

	assert.Equal(t, want, hex.EncodeToString(h.Sum(nil)),
		"the drawn characters moved: see the digest's doc comment before updating it")
}

// TestTheRuneSetReachesEveryUTF8Length checks the classes runeTables exists to
// cover are actually drawn.
//
// A table nothing reaches costs nothing and proves nothing, and the emitter
// counts bytes in some places and columns in others, so a corpus that never got
// past two bytes per character would leave that difference untested.
func TestTheRuneSetReachesEveryUTF8Length(t *testing.T) {
	runes := yamlgen.Runes()

	widths := map[int]int{}
	for seed := range 4096 {
		widths[utf8.RuneLen(runes.Example(seed))]++
	}

	for _, n := range []int{1, 2, 3, 4} {
		assert.Positivef(t, widths[n], "no draw was %d UTF-8 bytes wide", n)
	}

	t.Logf("1 byte %d, 2 bytes %d, 3 bytes %d, 4 bytes %d",
		widths[1], widths[2], widths[3], widths[4])
}

// TestGeneratedStringsCarryTheAwkwardCharacters checks the strings a document is
// built from reach the characters YAML has to decide about, rather than only the
// letters.
func TestGeneratedStringsCarryTheAwkwardCharacters(t *testing.T) {
	strs := yamlgen.Strings()

	var drawn strings.Builder
	for seed := range 8192 {
		drawn.WriteString(strs.Example(seed))
	}

	got := drawn.String()

	for name, r := range map[string]rune{
		"NUL":                 '\x00',
		"DEL":                 '\x7f',
		"NEL":                 '\u0085',
		"line separator":      '\u2028',
		"byte order mark":     '\ufeff',
		"right-to-left":       '\u202e',
		"a combining mark":    '\u0301',
		"a private use point": '\ue000',
	} {
		assert.Containsf(t, got, string(r), "no generated string held %s (%U)", name, r)
	}
}
