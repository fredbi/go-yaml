// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import "unicode/utf8"

// cursor is what reading one byte of the source costs.
//
// Every character the scan reads touches src, buf, idx, size, originEnd, notSpaceCharPos and originCut, through
// next, currentChar, progress, addBuf and addOriginBuf. Those seven come to 61 bytes and stand first, so a cache line
// holds all of them; the fields below the gap are read once a line or once a token and would otherwise sit among them.
//
// The order is by how often a field is read, not by width, which is what the rest of the package is ordered by.
//
// [Context] embeds it, so c.idx and c.src read as they did.
type cursor struct {
	// src is the source, held as Init was given it. size is len(src).
	src string
	// buf is where the value of the token being read is built, when the scan has to rewrite what it read.
	buf []byte
	// idx is the byte of src the scan stands on.
	idx  int32
	size int32
	// originEnd is where the current token's text ends in src, and originStart where it begins.
	//
	// See [Context.origin].
	originEnd int32
	// notSpaceCharPos is how much of buf is the value, leaving out the whitespace it ends with.
	notSpaceCharPos int32
	originStart     int32
	// originCut says originCopy is in use, a cut having taken bytes out of the middle of the text.
	originCut bool

	// Below the line the scan reads for every character.

	// raw is src's own bytes, for the word-at-a-time scans in [github.com/go-openapi/go-yaml/internal/scanner/swar].
	//
	// A string cannot be loaded eight bytes at a time without unsafe, and Init was handed the slice. indentRun reads it
	// once a line.
	raw []byte
	// originCopy holds the text once a cut has taken bytes out of the middle of it.
	originCopy []byte
}

func (c *cursor) next() bool {
	return c.idx < c.size
}

// width is how many bytes the character at the cursor takes, or 0 at the end.
func (c *cursor) width() int32 {
	if c.idx >= c.size {
		return 0
	}
	if c.src[c.idx] < utf8.RuneSelf {
		return 1
	}
	_, w := utf8.DecodeRuneInString(c.src[c.idx:])

	return int32(w)
}

// isEOS reports that no character follows the one at the cursor.
func (c *cursor) isEOS() bool {
	return c.idx+c.width() >= c.size
}

func (c *cursor) currentChar() rune {
	if c.idx < c.size {
		if b := c.src[c.idx]; b < utf8.RuneSelf {
			return rune(b)
		}

		r, _ := utf8.DecodeRuneInString(c.src[c.idx:])

		return r
	}

	return rune(0)
}

func (c *cursor) nextChar() rune {
	if w := c.width(); c.idx+w < c.size {
		r, _ := utf8.DecodeRuneInString(c.src[c.idx+w:])

		return r
	}

	return rune(0)
}

// previousChar returns the character before the cursor, stepping back over a byte order mark: the scanner steps over
// one rather than reading it, so nothing that asks what came before should see it.
func (c *cursor) previousChar() rune {
	end := c.idx
	for end > 0 {
		r, w := utf8.DecodeLastRuneInString(c.src[:end])
		if r != byteOrderMark {
			return r
		}
		end -= int32(w)
	}

	return rune(0)
}

// repeatNum counts how many times r stands at the cursor, in a row.
func (c *cursor) repeatNum(r rune) int32 {
	var cnt int32
	for i := c.idx; i < c.size; {
		cur, w := utf8.DecodeRuneInString(c.src[i:])
		if cur != r {
			break
		}
		cnt++
		i += int32(w)
	}

	return cnt
}

// progress advances the cursor by num characters and returns the bytes it crossed.
//
// Callers count columns in characters and offsets in bytes, which is why it reports both.
func (c *cursor) progress(num int32) int32 {
	start := c.idx
	for range num {
		if c.idx >= c.size {
			break
		}
		// A byte below utf8.RuneSelf stands for a character of its own, so its width is known without decoding it.
		// Decoding every character to ask how wide it is was 12% of the scanner's time.
		if c.src[c.idx] < utf8.RuneSelf {
			c.idx++

			continue
		}
		_, w := utf8.DecodeRuneInString(c.src[c.idx:])
		c.idx += int32(w)
	}

	return c.idx - start
}

// source returns the bytes between two byte offsets of c.src.
func (c *cursor) source(s, e int32) string {
	return c.src[s:e]
}

// textAt returns buf as a string and where in the source it was found, or -1 where buf is not the source's own bytes.
//
// A Go substring shares the bytes it is taken from, so a token whose text is the source's own costs nothing: it points
// into the document rather than carrying a copy of it.
// Scanning rewrites the text often enough -- escapes, folding, chomping -- that the source window is compared with buf
// rather than assumed equal to it.
//
// start is where the caller believes buf begins.
// It is a guess for a value the scanner folded: the offset it works out is the cursor less the folded length, and
// folding makes the value shorter than the source it was read from.
//
// Where the guess misses, the cursor gives the other end, and the offset that matched is the one the token should
// carry.
func (c *cursor) textAt(buf []byte, start int32) (string, int32) {
	if span, ok := c.window(buf, start); ok {
		return span, start
	}
	at := c.idx - int32(len(buf))
	if span, ok := c.window(buf, at); ok {
		return span, at
	}

	return string(buf), -1
}

// window returns the len(buf) bytes of the source at start, and reports whether they are buf's own.
func (c *cursor) window(buf []byte, start int32) (string, bool) {
	end := start + int32(len(buf))
	if start < 0 || end > int32(len(c.src)) {
		return "", false
	}

	span := c.src[start:end]

	return span, span == string(buf)
}

func (c *cursor) addBuf(r rune) {
	if len(c.buf) == 0 && (r == ' ' || r == '\t') {
		return
	}
	c.buf = utf8.AppendRune(c.buf, r)
	if r != ' ' && r != '\t' {
		c.notSpaceCharPos = int32(len(c.buf))
	}
}

func (c *cursor) addBufWithTab(r rune) {
	if len(c.buf) == 0 && r == ' ' {
		return
	}
	c.buf = utf8.AppendRune(c.buf, r)
	if r != ' ' {
		c.notSpaceCharPos = int32(len(c.buf))
	}
}

// origin is the text the current token was written as: everything read since the last token was cut, indentation and
// line breaks included.
//
// It is a window on the source.
// Nothing keeps it -- Origin left the token, and what reads it now measures it -- so the scanner records what it reads
// by moving originEnd rather than by copying the bytes into a buffer.
// Over the fuzz corpus that holds for 107,805 of 107,811 reads.
//
// The exception is a line whose trailing spaces are cut.
// Each cut takes a suffix, but the scan goes on and reads more, so what is left has a gap in the middle of it and no
// window can say so.
// The first cut copies what the window held and everything after it appends to the copy.
func (c *cursor) origin() string {
	if c.originCut {
		return string(c.originCopy)
	}

	return c.src[c.originStart:min(c.originEnd, int32(len(c.src)))]
}

// addOriginBuf records that r was read as part of the current token.
//
// One add, where appending r to a buffer was 8% of the scanner: a call that could not inline -- cost 106 against a
// budget of 80, utf8.AppendRune's body being worth 70 on its own -- around an append that copied a byte already in the
// source.
func (c *cursor) addOriginBuf(r rune) {
	if r < utf8.RuneSelf && !c.originCut {
		c.originEnd++

		return
	}

	c.addOriginWide(r)
}

// addOriginWide records a character that the window cannot count in one byte,
// or any character once a cut has put the text in a buffer.
//
// It is kept out of [cursor.addOriginBuf] so that one stays inside the
// inliner's budget: appending a rune is worth more than the whole budget on its
// own, and this is called for a byte in a thousand.
//
//go:noinline
func (c *cursor) addOriginWide(r rune) {
	if c.originCut {
		c.originCopy = utf8.AppendRune(c.originCopy, r)

		return
	}

	c.originEnd += int32(utf8.RuneLen(r))
}

// skipOrigin records that the n bytes at the cursor were read, as n calls to [cursor.addOriginBuf] would.
//
// The caller has established they are ASCII, so each is one character and one byte.
func (c *cursor) skipOrigin(n int32) {
	if c.originCut {
		c.originCopy = append(c.originCopy, c.src[c.idx:c.idx+n]...)

		return
	}

	c.originEnd += n
}

func (c *cursor) resetBuffer() {
	c.buf = c.buf[:0]
	c.notSpaceCharPos = 0
	c.originStart, c.originEnd = c.idx, c.idx
	c.originCopy = c.originCopy[:0]
	c.originCut = false
}

func (c *cursor) isMergeKey() bool {
	if c.repeatNum('<') != 2 {
		return false
	}
	src := c.src
	size := int32(len(src))
	for idx := c.idx + 2; idx < size; idx++ {
		char := src[idx]
		if char == ' ' {
			continue
		}
		if char != ':' {
			return false
		}
		if idx+1 < size {
			nc := rune(src[idx+1])
			if nc == ' ' || isNewLineChar(nc) {
				return true
			}
		}
	}

	return false
}
