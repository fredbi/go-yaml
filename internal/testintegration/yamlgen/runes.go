// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"unicode"

	"pgregory.net/rapid"
)

// The characters a generated string is drawn from.
//
// rapid.String draws from unicode.Lu, unicode.Ll, unicode.Lo and sixteen more
// categories, expanded into rune slices and indexed. Those tables carry the
// Unicode version of whatever toolchain compiled them: Go 1.25 ships Unicode
// 15.0.0 and Go 1.27 ships 17.0.0, so the same seed and the same bitstream pick
// a different character. On that upgrade 4,299 of the YAML corpus's 10,214 cases
// moved, with no case added, removed or re-labelled.
//
// A corpus that claims a seed reproduces it byte for byte cannot draw from a
// table the standard library revises. So the ranges below are written here, by
// number. Unicode assigns a codepoint once and never reassigns it, which is what
// keeps a range chosen by number stable where a range chosen by category is not.
//
// They are chosen for what a YAML reader has to decide about rather than for
// breadth:
//
//   - the UTF-8 lengths, one byte through four, since the scanner counts bytes
//     and the emitter counts columns;
//   - the characters c-printable excludes, which make a document invalid and are
//     the ones a double-quoted scalar has to escape;
//   - the line breaks YAML 1.1 recognized and 1.2 does not: NEL, LS and PS;
//   - combining marks, where one character is several codepoints.

// awkwardRunes are the single characters that have caused trouble, each one a
// decision the scanner or the emitter has to make on its own.
//
// rapid.RuneFrom weights this list against all the tables together, so a draw
// lands here about half the time.
var awkwardRunes = []rune{
	' ', '\t', '\r', '\n',
	'"', '\'', '\\', '#', ':', '-', '?', ',',
	'[', ']', '{', '}', '&', '*', '!', '|', '>', '%', '@', '`',
	'\x00', '\a', '\v', '\f', '\x1b', '\x7f', // NUL, BEL, VT, FF, ESC, DEL: c-printable excludes every one
	'\u0085',           // NEL, a line break in YAML 1.1 and not in 1.2
	'\u00a0',           // no-break space, which is not s-white
	'\u2028', '\u2029', // line and paragraph separator, 1.1 line breaks likewise
	'\ufeff', // byte order mark, a character everywhere but at the start of a stream
	'\ufffd', // replacement character
	'\u202e', // right-to-left override
	'\u023a', // capital A with stroke, which lowercases from two UTF-8 bytes to three
}

// runeTables are the ranges a generated string draws from, one table per class
// so that each is drawn as often as the others rather than in proportion to the
// number of characters it holds.
var runeTables = []*unicode.RangeTable{
	// ASCII, printable.
	{R16: []unicode.Range16{{Lo: 0x0020, Hi: 0x007e, Stride: 1}}},
	// C1 controls, which c-printable excludes along with C0.
	{R16: []unicode.Range16{{Lo: 0x0080, Hi: 0x009f, Stride: 1}}},
	// Latin-1 letters and Latin Extended-A and B: two UTF-8 bytes.
	{R16: []unicode.Range16{{Lo: 0x00c0, Hi: 0x024f, Stride: 1}}},
	// Combining diacritical marks, where one character is two codepoints.
	{R16: []unicode.Range16{{Lo: 0x0300, Hi: 0x036f, Stride: 1}}},
	// Greek, Cyrillic, Hebrew and Arabic, the last two written right to left.
	{R16: []unicode.Range16{
		{Lo: 0x0370, Hi: 0x03ff, Stride: 1},
		{Lo: 0x0400, Hi: 0x04ff, Stride: 1},
		{Lo: 0x05d0, Hi: 0x05ea, Stride: 1},
		{Lo: 0x0620, Hi: 0x064a, Stride: 1},
	}},
	// Hiragana and CJK: three UTF-8 bytes.
	{R16: []unicode.Range16{
		{Lo: 0x3041, Hi: 0x3096, Stride: 1},
		{Lo: 0x4e00, Hi: 0x4eff, Stride: 1},
	}},
	// Private use, which carries no meaning and has to survive anyway.
	{R16: []unicode.Range16{{Lo: 0xe000, Hi: 0xe0ff, Stride: 1}}},
	// Past the basic plane: four UTF-8 bytes, and two UTF-16 units for anyone
	// reading the corpus from a language that counts those.
	{R32: []unicode.Range32{
		{Lo: 0x10000, Hi: 0x1000b, Stride: 1},
		{Lo: 0x1f300, Hi: 0x1f64f, Stride: 1},
	}},
}

// Runes generates one character from the set this package owns.
func Runes() *rapid.Generator[rune] {
	return rapid.RuneFrom(awkwardRunes, runeTables...)
}
