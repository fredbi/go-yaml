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

func TestSpaceMask(t *testing.T) {
	for _, test := range []struct {
		word string
		want int
	}{
		{"        ", -1},
		{"       x", 7},
		{"x       ", 0},
		{"  a     ", 2},
		{"  \t     ", 2},
		{"  \n     ", 2},
		// The lane after a run of matches is where a borrow would land.
		{"  !!null", 2},
		{"       !", 7},
		{"  \x00     ", 2},
		{"  é    ", 2},
	} {
		assert.Equalf(t, test.want, stopsAt(test.word, SpaceMask), "%q", test.word)
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
// The point of the package is that a caller's hot loop pays no call: the bit math is meant to land in the loop body.
// A function that stops inlining keeps working and stops being worth having, which no other test would notice.
func TestInlinable(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the package to read the compiler's inlining decisions")
	}

	out, err := exec.CommandContext(context.Background(), "go", "build", "-gcflags=-m", ".").CombinedOutput()
	require.NoErrorf(t, err, "go build -gcflags=-m: %s", out)

	text := string(out)
	for _, fn := range []string{
		"FirstByte", "LanesBelow", "DoubleQuoteStopMask", "SingleQuoteStopMask", "SpaceMask",
		"Broadcast", "LanesZero", "MaskEqual", "ControlMask", "AllowedControlMask",
	} {
		assert.Containsf(t, text, "can inline "+fn,
			"%s no longer inlines, so its callers pay a call for eight bytes of work:\n%s", fn, text)
	}
}

// TestControlMasksMatchTheByteRule holds the two control masks to the rule they stand for, over every word one lane of
// which is any byte under 0x80 and the other seven are drawn from the awkward neighborhood: the boundaries of the
// control range, the three characters c-printable admits, and DEL.
//
// The masks are exact, which the cheap forms in this file are not: a lane may not be flagged because a lower lane
// matched.
// TestControlMaskHasNoBorrow holds that separately.
func TestControlMasksMatchTheByteRule(t *testing.T) {
	// unprintable is c-printable's verdict on a byte under 0x80.
	unprintable := func(c byte) bool {
		if c == 0x09 || c == 0x0A || c == 0x0D {
			return false
		}

		return c < 0x20 || c == 0x7F
	}

	neighbors := []byte{0x00, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x1F, 0x20, 0x21, 0x7E, 0x7F}

	for _, filler := range neighbors {
		for c := range 0x80 {
			for lane := range 8 {
				var b [8]byte
				for i := range b {
					b[i] = filler
				}
				b[lane] = byte(c)
				w := binary.LittleEndian.Uint64(b[:])

				got := ControlMask(w) &^ AllowedControlMask(w)

				var want uint64
				for i, v := range b {
					if unprintable(v) {
						want |= 0x80 << (8 * i)
					}
				}
				require.Equalf(t, want, got, "% x", b)
			}
		}
	}
}

// TestControlMaskHasNoBorrow holds the mask to reporting nothing for a word of bytes a stream may all hold.
//
// This is the failure the cheap forms have: "\n " subtracts and the space's lane borrows, so a word that is entirely
// valid comes back flagged.
// A mask read for emptiness has to be right in every lane, not only in the lowest.
func TestControlMaskHasNoBorrow(t *testing.T) {
	for _, text := range []string{
		"\n       ", " \n      ", "\n \n \n \n ", "\ta\rb\nc d",
		"        ", "\t\t\t\t\t\t\t\t", "\r\n\r\n\r\n\r\n", "~~~~~~~~",
	} {
		require.Lenf(t, text, 8, "%q is not a word", text)
		w := binary.LittleEndian.Uint64([]byte(text))
		assert.Zerof(t, ControlMask(w)&^AllowedControlMask(w),
			"%q holds nothing a stream may not, and the mask flagged a lane", text)
	}
}
