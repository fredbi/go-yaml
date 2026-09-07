// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package swar scans eight bytes of a document at a time, in one register.
//
// Each function reads one little-endian word and returns a per-lane mask: the high bit of a byte is set where that byte
// matches, and clear elsewhere.
// The caller keeps its own loop, tests the mask against zero, and calls [FirstByte] to find which byte stopped it.
//
// Only the per-word arithmetic lives here, never a loop.
// A helper that owns the loop is too large to inline -- go-openapi/core's JSON lexer measured cost 98 against the
// budget of 80 and lost the win it was written for -- so every function here is small enough that the call disappears.
// TestInlinable is what holds that.
//
//nolint:mnd // mnd is not really fit to analyze a low-level swag utility.
package swar

import "math/bits"

const (
	lo   = ^uint64(0) / 255 // 0x0101010101010101, the low bit of every lane
	high = lo * 0x80        // 0x8080808080808080, the high bit of every lane
)

// HighBits is the high bit of every lane.
//
// A word OR-ed into an accumulator over a run of bytes says whether the run held anything but ASCII: acc&HighBits == 0
// means every byte stood for a character of its own, so a count of bytes is a count of characters and a column can
// advance by the length of the run.
// That is what makes a bulk skip safe.
const HighBits = high

// FirstByte returns which lane, 0 to 7, is the lowest-addressed one flagged in mask. mask must be non-zero and carry
// bits only at lane high bits, which is what the mask functions return.
func FirstByte(mask uint64) int { return bits.TrailingZeros64(mask) >> 3 }

// LanesBelow returns w with every lane from k upward cleared, for k in 0 to 8.
//
// It trims the word a scan stopped inside: only the bytes before the stop belong to the run, so only they may say the
// run held a byte over 0x7f. Go reads an over-wide shift as zero, so k of 0 gives 0 without a branch.
func LanesBelow(w uint64, k int) uint64 { return w & (^uint64(0) >> (64 - 8*k)) }

// Broadcast copies b into all eight lanes.
func Broadcast(b byte) uint64 { return lo * uint64(b) }

// LanesZero flags the lanes of x that are zero, exactly.
//
// It is the one exact per-lane test the others are built from: a lane is zero iff the carry the saturating add makes
// never reaches its high bit, and the add cannot carry across a lane.
//
// The cheap "(x - lo) &^ x & high" form used by the stop masks below is smaller but can flag a lane because a lower
// lane borrowed, which is harmless where the caller takes only [FirstByte] and wrong where it tests the mask for
// emptiness or clears lanes out of another mask.
func LanesZero(x uint64) uint64 { return ^(((x &^ high) + ^high) | x) & high }

// MaskEqual flags the lanes holding b, exactly.
func MaskEqual(w uint64, b byte) uint64 { return LanesZero(w ^ Broadcast(b)) }

// ControlMask flags the lanes holding a control character: a byte under 0x20, or DEL.
//
// Every lane of w must hold a byte under 0x80, which a caller establishes with w&[HighBits] == 0.
//
// Given that precondition a byte is under 0x20 exactly where its top three bits are clear, which is one AND and one
// [LanesZero] rather than the range compare a general "less than" needs.
//
// YAML 1.2's c-printable admits three of these -- see [AllowedControlMask] -- so a caller after the bytes a stream may
// not hold writes.
//
//	ControlMask(w) &^ AllowedControlMask(w)
//
// The two are apart because together they cost 93 against the inline budget of 80, and a mask function that does not
// inline puts a call back into the loop this package exists to keep calls out of.
func ControlMask(w uint64) uint64 {
	return LanesZero(w&(lo*0xE0)) | LanesZero(w^(lo*0x7F))
}

// AllowedControlMask flags the lanes holding the three control characters YAML 1.2's c-printable admits: tab, line feed
// and carriage return.
//
// See [ControlMask].
func AllowedControlMask(w uint64) uint64 {
	m := LanesZero(w ^ (lo * 0x09))
	m |= LanesZero(w ^ (lo * 0x0A))

	return m | LanesZero(w^(lo*0x0D))
}

// DoubleQuoteStopMask flags the bytes that end a run inside a double-quoted scalar: the closing '"', a '\' opening an
// escape, and anything under 0x20 -- which in a stream validateStream has passed is a tab or a line break.
//
// The needles are all under 0x80, so the cheap form holds: the &^ w of the first term drops any lane whose high bit is
// set, so a byte of a multi-byte character is never read as a control character, and the two equality terms are exact.
func DoubleQuoteStopMask(w uint64) uint64 {
	m := (w - lo*0x20) &^ w & high // under 0x20
	q := w ^ (lo * 0x22)
	m |= (q - lo) &^ q & high // '"'
	e := w ^ (lo * 0x5c)
	m |= (e - lo) &^ e & high // '\'

	return m
}

// SingleQuoteStopMask flags the bytes that end a run inside a single-quoted scalar: the "'" that closes it or doubles
// for one, and anything under 0x20. A single-quoted scalar has no backslash escape, so '\' is content there.
func SingleQuoteStopMask(w uint64) uint64 {
	m := (w - lo*0x20) &^ w & high // under 0x20
	q := w ^ (lo * 0x27)
	m |= (q - lo) &^ q & high // '\''

	return m
}

// SpaceMask flags the bytes that are not a space, so a run of spaces ends at the first lane it sets.
//
// Indentation is spaces and nothing else -- YAML refuses a tab there -- so this is what steps over the opening of a
// line.
// Over the analysis workloads the mean run is 2.4 to 10.8 spaces, and golang_source holds 52% of its bytes in runs of
// eight or more.
func SpaceMask(w uint64) uint64 {
	// The saturating form, not the (q-lo)&^q one the stop masks use.
	// That one borrows across lanes: a lane that matches leaves 0xff behind it and the borrow walks into the lanes above.
	// A stop mask survives it because [FirstByte] reads the lowest flagged lane and a borrow can only flag one higher up.
	//
	// This mask flags what does NOT match, so the borrow lands on the lanes it is asked about: " !!null" reported its
	// first non-space at byte 4 rather than 2, and two spaces of indentation swallowed the "!!".
	//
	// Clearing the high bit of every lane before the add is what keeps the carry inside its lane: 0x7f + 0x7f is 0xfe and
	// does not reach the next.
	x := w ^ (lo * 0x20)
	y := ((x & ^high) + ^high) | x

	return y & high
}

// atLeast flags the lanes holding a byte of n or more.
//
// Every lane of w must hold a byte under 0x80, and n must be under 0x80 too, which is what makes it exact: setting
// every high bit puts each lane at 0x80+c, and 0x80+c-n cannot borrow out of its lane because it never goes below 1.
// The high bit is then left standing exactly where c >= n.
//
// The stop masks' cheaper "(q-lo)&^q" form borrows across lanes and survives it only because [FirstByte] reads the
// lowest flagged lane. This one is combined with others before anything is read off it, so it has to be exact.
func atLeast(w uint64, n byte) uint64 { return ((w | high) - Broadcast(n)) & high }

// LetterMask flags the lanes holding an ASCII letter, upper or lower case.
//
// Every lane of w must hold a byte under 0x80, which a caller establishes with w&[HighBits] == 0.
//
// Letters and digits are the continue-set a plain scalar can be stepped over with: none of the 24 characters the scan
// has a case for is one, so a run of them is a run the character loop would have appended byte by byte and nothing
// else. Over the workload corpus that covers 37% to 64% of the document in runs averaging 5.1 to 8.1 bytes -- about
// one word apiece, which is why this has to inline. A caller after the bytes that end such a run writes
//
//	^(LetterMask(w) | DigitMask(w)) & HighBits
//
// and keeps its own loop. The two are apart, and no function here spells that combination, for the reason
// [ControlMask] and [AllowedControlMask] are apart: together they come to 98 against the budget of 80.
//
// Folding with 0x20 maps A-Z onto a-z and lands nothing else in that range: only 0x41-0x5A and 0x61-0x7A give a byte
// in 0x61-0x7A, since 0x40 folds to 0x60 and 0x5B to 0x7B.
func LetterMask(w uint64) uint64 {
	folded := w | (lo * 0x20)

	return atLeast(folded, 'a') &^ atLeast(folded, 'z'+1)
}

// DigitMask flags the lanes holding an ASCII digit.
//
// Every lane of w must hold a byte under 0x80, which a caller establishes with w&[HighBits] == 0.
func DigitMask(w uint64) uint64 {
	return atLeast(w, '0') &^ atLeast(w, '9'+1)
}
