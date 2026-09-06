// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

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
