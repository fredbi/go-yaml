// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package token

import "strings"

// Lookback carries what the tokens already read say about the next one: whether
// the author left a blank line above it, and how many line breaks the comments
// above it take up.
//
// Both answers depend only on the last two tokens, on whether the scalar before
// those is the content of a block header, and on a running count of the breaks
// in the comments just read. Lookback keeps those four things and updates them
// as each token goes past, so a stream can be read once and forward -- nothing
// has to walk back over the tokens already emitted, and nothing has to hold
// them.
type Lookback struct {
	prev  *Token
	prev2 *Token

	// blockHeader answers, for the prefix ending one and two tokens back,
	// whether it ends on a literal or folded header once the comments between
	// are skipped. Index 0 is the prefix through prev, 1 the prefix before
	// prev, 2 the prefix before prev2.
	blockHeader [3]bool

	// commentBreaks counts the line breaks taken by the run of comments read
	// since the last token that was not a comment.
	commentBreaks int32
}

// Derive sets tk.BlankLineAbove and tk.CommentBreaksAbove from the tokens l has
// read, then reads tk itself.
//
// The first token of a stream keeps the zero values: nothing stands above it.
func (l *Lookback) Derive(tk *Token) {
	if tk == nil {
		return
	}
	if l.prev != nil {
		tk.BlankLineAbove = l.blankLineAbove(tk)
		tk.CommentBreaksAbove = l.commentBreaksAbove(tk)
	}
	l.read(tk)
}

// Reset drops what l has read, so it starts again on a fresh stream.
func (l *Lookback) Reset() {
	*l = Lookback{}
}

// read moves tk into l's state, shifting out the token that falls off the end.
func (l *Lookback) read(tk *Token) {
	l.blockHeader[2] = l.blockHeader[1]
	l.blockHeader[1] = l.blockHeader[0]
	if tk.Type == CommentType {
		// A comment stands between a header and its content without separating
		// them, so it neither makes nor breaks a block header, and its breaks
		// add to the run above whatever comes next.
		l.commentBreaks += int32(strings.Count(normalizeNewLineChars(tk.Origin), "\n"))
	} else {
		l.blockHeader[0] = tk.Type == LiteralType || tk.Type == FoldedType
		l.commentBreaks = 0
	}
	l.prev2 = l.prev
	l.prev = tk
}

// blankLineAbove reports whether the author left an empty line above t.
func (l *Lookback) blankLineAbove(t *Token) bool {
	const lbc = "\n"

	prev := l.prev
	// blockHeader for the prefix before prev, which is what says whether prev
	// is block scalar content.
	header := l.blockHeader[1]

	var adjustment int32
	// A sequence entry's '-' says nothing about a gap: the gap the author left
	// is above the '-', so the comparison steps back past it. The lines between
	// the '-' and t are then the entry's own layout --
	// -
	//   b: c
	// -- and not part of that gap.
	if prev.Type == SequenceEntryType {
		adjustment = t.Position.Line - prev.Position.Line
		if l.prev2 != nil {
			prev = l.prev2
			header = l.blockHeader[2]
		}
	}

	lineDiff := t.Position.Line - prev.Position.Line - 1
	if lineDiff <= 0 {
		return false
	}

	switch prev.Type {
	case StringType, SingleQuoteType, DoubleQuoteType:
		// A scalar may span lines: a quoted one written across two, a block one
		// whose content is several, and the blank lines a "|+" keeps. Those
		// lines are the scalar's own, and none of them is a gap the author left
		// above t.
		adjustment += int32(linesSpannedBy(prev, header, lbc))
	case NullType, ImplicitNullType:
		// Due to the way that comment parsing works its assumed that when a null value does not have new line in origin
		// it was squashed therefore difference is ignored.
		// foo:
		//  bar:
		//  # comment
		//  baz: 1
		// becomes
		// foo:
		//  bar: null # comment
		//
		//  baz: 1
		return strings.Count(prev.Origin, lbc) > 0
	}

	return lineDiff-adjustment > 0
}

// commentBreaksAbove counts the line breaks the comments written immediately
// above tk take up.
//
// A comment token carries its own line break in its origin. Where the comments
// are dropped -- a node rendered without them -- those breaks have to be
// written back, or the line after a comment runs into the line before it.
func (l *Lookback) commentBreaksAbove(tk *Token) int32 {
	if tk.Type == CommentType {
		return 0
	}

	return l.commentBreaks
}

// linesSpannedBy returns how many lines past its first the scalar tk occupies.
// isContent says whether tk holds the content of a literal or folded block.
//
// A block scalar is measured by its value, because that is what chomping has
// already settled: the blank lines a "|+" keeps are content and are lines the
// scalar is written on, while the ones a "|" clips away are not the scalar's at
// all. Every other scalar is measured by its source text, whose trailing blank
// lines run on to whatever comes next rather than belonging to it.
func linesSpannedBy(tk *Token, isContent bool, lbc string) int {
	if isContent {
		lines := strings.Count(tk.Value, lbc)
		if !strings.HasSuffix(tk.Value, lbc) {
			lines++
		}
		if lines < 1 {
			return 0
		}

		return lines - 1
	}

	body := strings.TrimSpace(tk.Origin)

	return strings.Count(strings.TrimRight(body, lbc), lbc)
}

// normalizeNewLineChars reads CR LF and CR as one line break each.
func normalizeNewLineChars(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}
