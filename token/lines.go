// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package token

import "strings"

// linesSpannedBy returns how many lines past its first the scalar tk occupies.
//
// A block scalar is measured by its value, because that is what chomping has
// already settled: the blank lines a "|+" keeps are content and are lines the
// scalar is written on, while the ones a "|" clips away are not the scalar's at
// all. Every other scalar is measured by its source text, whose trailing blank
// lines run on to whatever comes next rather than belonging to it.
func linesSpannedBy(tk *Token, before Tokens, lbc string) int {
	if isBlockScalarContent(before) {
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

// isBlockScalarContent reports whether tk holds the content of a literal or
// folded block scalar, which is to say that its header is what precedes it.
func isBlockScalarContent(before Tokens) bool {
	for i := len(before) - 1; i >= 0; i-- {
		switch before[i].Type {
		case CommentType:
			// A header may carry a comment, which stands between it and the
			// content without separating them.
			continue
		case LiteralType, FoldedType:
			return true
		default:
			return false
		}
	}

	return false
}

// blankLineAbove reports whether the author left an empty line above t, where
// before holds the tokens already read.
func blankLineAbove(t *Token, before Tokens) bool {
	if len(before) > 0 {
		lbc := "\n"
		prevIdx := len(before) - 1
		prev := before[prevIdx]
		var adjustment int
		// A sequence entry's '-' says nothing about a gap: the gap the author
		// left is above the '-', so the comparison steps back past it. The
		// lines between the '-' and t are then the entry's own layout --
		// -
		//   b: c
		// -- and not part of that gap.
		if prev.Type == SequenceEntryType {
			adjustment = t.Position.Line - prev.Position.Line
			if prevIdx > 0 {
				prevIdx--
				prev = before[prevIdx]
			}
		}
		lineDiff := t.Position.Line - prev.Position.Line - 1
		if lineDiff > 0 {
			switch prev.Type {
			case StringType, SingleQuoteType, DoubleQuoteType:
				// A scalar may span lines: a quoted one written across two, a
				// block one whose content is several, and the blank lines a
				// "|+" keeps. Those lines are the scalar's own, and none of
				// them is a gap the author left above t.
				adjustment += linesSpannedBy(prev, before[:prevIdx], lbc)
			}
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
			if prev.Type == NullType || prev.Type == ImplicitNullType {
				return strings.Count(prev.Origin, lbc) > 0
			}
			if lineDiff-adjustment > 0 {
				return true
			}
		}
	}
	return false
}

// commentBreaksAbove counts the line breaks the comments written immediately
// above tk take up, where before holds the tokens already read.
//
// A comment token carries its own line break in its origin. Where the comments
// are dropped -- a node rendered without them -- those breaks have to be
// written back, or the line after a comment runs into the line before it.
func commentBreaksAbove(tk *Token, before Tokens) int32 {
	if tk.Type == CommentType {
		return 0
	}

	var breaks int32
	for i := len(before) - 1; i >= 0; i-- {
		if before[i].Type != CommentType {
			break
		}
		breaks += int32(strings.Count(normalizeNewLineChars(before[i].Origin), "\n"))
	}

	return breaks
}

// normalizeNewLineChars reads CR LF and CR as one line break each.
func normalizeNewLineChars(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}
