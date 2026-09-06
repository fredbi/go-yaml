// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"encoding/binary"
	"fmt"
	"unicode/utf8"
	"unsafe"

	"github.com/go-openapi/go-yaml/internal/scanner/swar"
	"github.com/go-openapi/go-yaml/token"
)

// byteOrderMark is YAML 1.2's c-byte-order-mark.
//
// nb-char is c-printable less b-char and this, so a byte order mark is not a character any node may hold: it marks a
// document prefix and nothing else.
// Scanner.checkByteOrderMark refuses one anywhere a node may go, and the scan steps over the rest, which is what a file
// saved by an editor that writes one needs.
//
// A mark says nothing here about the encoding.
// The spec has a stream announce UTF-16 or UTF-32 with one, and this library reads UTF-8 only -- the one place it
// departs from YAML 1.2.2 on purpose.
// A UTF-16 stream's mark is two bytes that are not a character, which validateStream refuses.
const byteOrderMark = '\ufeff'

// byteOrderMarkText is the mark's three bytes, for the prefix tests that step over a run of them.
const byteOrderMarkText = string(byteOrderMark)

// validateStream checks that the source is text a YAML stream may hold.
//
// c-printable is the set of characters a stream may contain at all, so the control characters below x20 other than tab,
// line feed and carriage return are not YAML however they are arrived at.
//
// A stream is also Unicode, and a byte that is part of no character is not one.
// Converting the source to runes turns each of them into U+FFFD, so by the time anything else looks the byte is gone
// and nothing has said so -- which is why this reads the string rather than the runes the rest of the scanner works on.
func validateStream(text string) error {
	if at := firstUnprintable(text); at >= 0 {
		return unprintableErr(text, at)
	}

	return nil
}

// firstUnprintable returns the offset of the first byte the stream may not hold, or -1 where every one of them is a
// character c-printable admits.
//
// Eight bytes at a time while they are ASCII, which is nearly all of them in nearly every document: one word says
// whether any of the eight is a control character other than tab, line feed and carriage return, or DEL.
// A word carrying a byte over 0x7f is stepped through a character at a time, since what c-printable admits up there is
// a question about the character rather than about the byte, and the word loop takes over again after it.
func firstUnprintable(text string) int {
	raw := unsafe.Slice(unsafe.StringData(text), len(text))

	i := 0
	for i+8 <= len(text) {
		w := binary.LittleEndian.Uint64(raw[i:])
		if w&swar.HighBits == 0 {
			if m := swar.ControlMask(w) &^ swar.AllowedControlMask(w); m != 0 {
				return i + swar.FirstByte(m)
			}
			i += 8

			continue
		}

		next, bad := readNonASCII(text, w, i)
		if bad >= 0 {
			return bad
		}
		i = next
	}

	return firstUnprintableTail(text, i)
}

// readNonASCII steps over the characters of the word at i that carries a byte over 0x7f, and returns where the bytes
// are ASCII again, or where the stream holds a character it may not.
//
// It takes the whole run rather than one character: a document written in a script of three bytes to the character
// loaded and tested a word for every one of them, and every test failed the same way.
// It is a call rather than the loop's own code so that the word loop above stays small -- the run is the cold path, and
// twitter_status, the most non-Latin of the workloads, reaches it for one word in five.
func readNonASCII(text string, w uint64, i int) (int, int) {
	// The ASCII bytes standing in front of the first one over 0x7f are read from the word as usual.
	k := swar.FirstByte(w & swar.HighBits)
	if m := (swar.ControlMask(w) &^ swar.AllowedControlMask(w)) & (1<<(8*k) - 1); m != 0 { //nolint:mnd // the shift is fine
		return 0, i + swar.FirstByte(m)
	}
	i += k

	for i < len(text) && text[i] >= utf8.RuneSelf {
		r, width := utf8.DecodeRuneInString(text[i:])
		if r == utf8.RuneError && width <= 1 {
			return 0, i
		}
		if !printable(r) {
			return 0, i
		}
		i += width
	}

	return i, -1
}

// firstUnprintableTail reads the last bytes of the source, fewer than a word of them, one character at a time.
func firstUnprintableTail(text string, i int) int {
	for i < len(text) {
		if c := text[i]; c < utf8.RuneSelf {
			if !printableASCII(c) {
				return i
			}
			i++

			continue
		}

		r, width := utf8.DecodeRuneInString(text[i:])
		if r == utf8.RuneError && width <= 1 {
			// Either a byte that is not text, or a U+FFFD the author wrote: only the width tells them apart.
			return i
		}
		if !printable(r) {
			return i
		}
		i += width
	}

	return -1
}

// unprintableErr names the character at offset i, which firstUnprintable found the stream may not hold.
func unprintableErr(text string, i int) error {
	r, width := utf8.DecodeRuneInString(text[i:])
	if r == utf8.RuneError && width <= 1 {
		return ErrInvalidToken("found a byte that is part of no character",
			token.Invalid(text[i:i+1], streamPosition(text, i)))
	}

	return ErrInvalidToken(
		fmt.Sprintf("found character %q that a YAML stream may not hold", r),
		token.Invalid(string(r), streamPosition(text, i)),
	)
}

// printableASCII answers [printable] for the bytes below utf8.RuneSelf, where it comes to one range and the three line
// and tab characters outside it.
func printableASCII(c byte) bool {
	return c >= 0x20 && c <= 0x7E || c == 0x09 || c == 0x0A || c == 0x0D
}

// streamPosition counts the characters before byte i, for the error that says which one a stream may not hold.
//
// Line, column and offset all count from 1, and offset counts characters rather than bytes, as validateStream reported
// them when it counted every character to report one.
func streamPosition(text string, i int) token.Position {
	line, column, offset := 1, 1, 1
	for _, r := range text[:i] {
		offset++
		if r == '\n' {
			line++
			column = 1

			continue
		}
		column++
	}

	return token.At(int32(line), int32(column), int32(offset), 0)
}

// printable is YAML 1.2's c-printable.
//
//nolint:mnd // we have a lot of runes to check and making them constants won't really improve readability.
func printable(r rune) bool {
	switch {
	case r == 0x09 || r == 0x0A || r == 0x0D:
		return true
	case r >= 0x20 && r <= 0x7E:
		return true
	case r == 0x85:
		return true
	case r >= 0xA0 && r <= 0xD7FF:
		return true
	case r >= 0xE000 && r <= 0xFFFD:
		return true
	default:
		return r >= 0x10000 && r <= 0x10FFFF
	}
}
