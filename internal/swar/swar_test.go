// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package swar

import (
	"context"
	"encoding/binary"
	"os/exec"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

// stopsAt reports which byte of an eight-byte word a mask flags first, or -1.
func stopsAt(word string, mask func(uint64) uint64) int {
	b := []byte(word)
	if len(b) != 8 {
		panic("stopsAt wants eight bytes")
	}
	m := mask(binary.LittleEndian.Uint64(b))
	if m == 0 {
		return -1
	}

	return FirstByte(m)
}

func TestDoubleQuoteStopMask(t *testing.T) {
	for _, test := range []struct {
		word string
		want int
	}{
		{"abcdefgh", -1},
		{`abc"efgh`, 3},
		{`ab\defgh`, 2},
		{"abc\nefgh", 3},
		{"abc\refgh", 3},
		{"abc\tefgh", 3},
		{`"bcdefgh`, 0},
		{"abcdefg\"", 7},
		{"abc\x00efgh", 3},
		// A multi-byte character is not a control character: é is 0xc3 0xa9.
		{"abédefg", -1},
		// The first stop wins, wherever the others stand.
		{"a\"c\\efgh", 1},
	} {
		assert.Equalf(t, test.want, stopsAt(test.word, DoubleQuoteStopMask), "%q", test.word)
	}
}

func TestSingleQuoteStopMask(t *testing.T) {
	for _, test := range []struct {
		word string
		want int
	}{
		{"abcdefgh", -1},
		{"abc'efgh", 3},
		// A backslash is content in a single-quoted scalar.
		{`ab\defgh`, -1},
		{"abc\nefgh", 3},
		{"abédefg", -1},
	} {
		assert.Equalf(t, test.want, stopsAt(test.word, SingleQuoteStopMask), "%q", test.word)
	}
}

func TestLanesBelow(t *testing.T) {
	w := binary.LittleEndian.Uint64([]byte{1, 2, 3, 4, 5, 6, 7, 8})
	assert.Equal(t, uint64(0), LanesBelow(w, 0))
	assert.Equal(t, uint64(1), LanesBelow(w, 1))
	assert.Equal(t, w, LanesBelow(w, 8))
}

func TestHighBitsFindsNonASCII(t *testing.T) {
	ascii := binary.LittleEndian.Uint64([]byte("abcdefgh"))
	assert.Zero(t, ascii&HighBits)

	wide := binary.LittleEndian.Uint64([]byte("abcéefg"))
	assert.NotZero(t, wide&HighBits)
}

// TestInlinable holds every function here inside the compiler's inline budget.
//
// The point of the package is that a caller's hot loop pays no call: the bit
// math is meant to land in the loop body. A function that stops inlining keeps
// working and stops being worth having, which no other test would notice.
func TestInlinable(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the package to read the compiler's inlining decisions")
	}

	out, err := exec.CommandContext(context.Background(), "go", "build", "-gcflags=-m", ".").CombinedOutput()
	require.NoErrorf(t, err, "go build -gcflags=-m: %s", out)

	text := string(out)
	for _, fn := range []string{
		"FirstByte", "LanesBelow", "DoubleQuoteStopMask", "SingleQuoteStopMask",
	} {
		assert.Containsf(t, text, "can inline "+fn,
			"%s no longer inlines, so its callers pay a call for eight bytes of work:\n%s", fn, text)
	}
}
