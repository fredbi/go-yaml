// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"unicode/utf8"

	"github.com/go-openapi/go-yaml/token"
)

// scanCommentIndicator refuses the '#' that [Scanner.scanComment] declined.
//
// Inside a plain scalar a '#' is an ordinary character, so one arriving with something already buffered is left to the
// scalar scan and this returns nil.
//
// A '#' reaching this point opens a token. It opens no comment, nothing having separated it from what came before,
// and no plain scalar, which may not begin with '#'. So it is refused.
//
// Example: "[a,#b]" is refused, a ',' not being separation.
func (s *Scanner) scanCommentIndicator(ctx *Context) error {
	if ctx.existsBuffer() || s.inAnchorName('#') {
		return nil
	}

	ctx.addBuf('#')
	ctx.addOriginBuf('#')
	err := ErrInvalidToken(
		"a comment must be preceded by a space, and a scalar cannot begin with '#'",
		token.Invalid(ctx.origin(), s.pos()),
	)
	s.progressColumn(ctx, 1)
	ctx.clear()

	return err
}

// scanComment reads a comment token, and returns false for a '#' that opens no comment.
//
// c-nb-comment-text is preceded by separation or starts a line, so this takes a '#' standing after a space, a tab or
// a line break, and one opening the stream. Every other '#' is declined and left to [Scanner.scanCommentIndicator].
//
// [cursor.previousChar] steps back over a byte order mark and returns 0 when nothing stands before the cursor, so a
// comment opening the stream behind a mark still starts a line.
//
// Example:
//
//	a: 1 # c    # Integer(1), then Comment(" c")
//	a: 1# c     # one plain scalar, String("1# c")
//	#c          # Comment("c"): a '#' opening the stream starts a line
//
// The comment runs to the end of the line, or to the end of the source when no break follows.
func (s *Scanner) scanComment(ctx *Context) bool {
	if c := ctx.previousChar(); c != rune(0) && c != ' ' && c != '\t' && !isNewLineChar(c) {
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf('#')
	// As in scanTag: the offset takes the '#', and the comment's own position is taken before the step.
	commentPos := s.pos()
	s.progress(ctx, 1) // skip '#' character

	for idx, c := range ctx.src[ctx.idx:] {
		ctx.addOriginBuf(c)
		if !isNewLineChar(c) {
			continue
		}
		if ctx.previousChar() == '\\' {
			continue
		}
		value := ctx.source(ctx.idx, ctx.idx+int32(idx))
		progress := int32(utf8.RuneCountInString(value))

		// CRLF ends one line, and progressLine steps over a single character.
		//
		// Leaving the '\n' behind would give it to the next token, whose leading whitespace then reads as a second break.
		// The next token would land two lines down, and writing the document back would open a blank line above the comment.
		crlf := c == '\r' && ctx.idx+int32(idx)+1 < int32(len(ctx.src)) && ctx.src[ctx.idx+int32(idx)+1] == '\n'
		if crlf {
			ctx.addOriginBuf('\n')
		}

		ctx.addTokenValue(token.MakeComment(value, ctx.origin(), commentPos))
		s.progressColumn(ctx, progress)
		s.progressLine(ctx)
		if crlf {
			s.progress(ctx, 1)
		}
		ctx.clear()

		return true
	}

	// document ends with comment.
	value := ctx.src[ctx.idx:]
	ctx.addTokenValue(token.MakeComment(value, ctx.origin(), commentPos))
	progress := int32(utf8.RuneCountInString(value))
	s.progressColumn(ctx, progress)
	s.progressLine(ctx)
	ctx.clear()

	return true
}
