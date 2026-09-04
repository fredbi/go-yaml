// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package swar scans eight bytes of a document at a time, in one register.
//
// Each function reads one little-endian word and returns a per-lane mask: the
// high bit of a byte is set where that byte matches, and clear elsewhere. The
// caller keeps its own loop, tests the mask against zero, and calls [FirstByte]
// to find which byte stopped it.
//
// Only the per-word arithmetic lives here, never a loop. A helper that owns the
// loop is too large to inline -- go-openapi/core's JSON lexer measured cost 98
// against the budget of 80 and lost the win it was written for -- so every
// function here is small enough that the call disappears. TestInlinable is what
// holds that.
package swar

import "math/bits"

const (
	lo   = ^uint64(0) / 255 // 0x0101010101010101, the low bit of every lane
	high = lo * 0x80        // 0x8080808080808080, the high bit of every lane
)

// HighBits is the high bit of every lane.
//
// A word OR-ed into an accumulator over a run of bytes says whether the run held
// anything but ASCII: acc&HighBits == 0 means every byte stood for a character
// of its own, so a count of bytes is a count of characters and a column can
// advance by the length of the run. That is what makes a bulk skip safe.
const HighBits = high

// FirstByte returns which lane, 0 to 7, is the lowest-addressed one flagged in
// mask. mask must be non-zero and carry bits only at lane high bits, which is
// what the mask functions return.
func FirstByte(mask uint64) int { return bits.TrailingZeros64(mask) >> 3 }

// LanesBelow returns w with every lane from k upward cleared, for k in 0 to 8.
//
// It trims the word a scan stopped inside: only the bytes before the stop
// belong to the run, so only they may say the run held a byte over 0x7f. Go
// reads an over-wide shift as zero, so k of 0 gives 0 without a branch.
func LanesBelow(w uint64, k int) uint64 { return w & (^uint64(0) >> (64 - 8*k)) }

// DoubleQuoteStopMask flags the bytes that end a run inside a double-quoted
// scalar: the closing '"', a '\' opening an escape, and anything under 0x20 --
// which in a stream validateStream has passed is a tab or a line break.
//
// The needles are all under 0x80, so the cheap form holds: the &^ w of the
// first term drops any lane whose high bit is set, so a byte of a multi-byte
// character is never read as a control character, and the two equality terms
// are exact.
func DoubleQuoteStopMask(w uint64) uint64 {
	m := (w - lo*0x20) &^ w & high // under 0x20
	q := w ^ (lo * 0x22)
	m |= (q - lo) &^ q & high // '"'
	e := w ^ (lo * 0x5c)
	m |= (e - lo) &^ e & high // '\'

	return m
}

// SingleQuoteStopMask flags the bytes that end a run inside a single-quoted
// scalar: the "'" that closes it or doubles for one, and anything under 0x20.
// A single-quoted scalar has no backslash escape, so '\' is content there.
func SingleQuoteStopMask(w uint64) uint64 {
	m := (w - lo*0x20) &^ w & high // under 0x20
	q := w ^ (lo * 0x27)
	m |= (q - lo) &^ q & high // '\''

	return m
}
