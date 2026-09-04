package scanner

import (
	"unicode/utf8"

	"github.com/go-openapi/go-yaml/token"
)

// scanCommentIndicator reports the '#' that scanComment declined.
//
// Inside a plain scalar a '#' is an ordinary character, so one that follows
// something already buffered is left alone. Starting a token it is neither a
// comment -- nothing separates it from what came before -- nor the first
// character of a plain scalar, which YAML does not allow it to be.
func (s *Scanner) scanCommentIndicator(ctx *Context) error {
	if ctx.existsBuffer() {
		return nil
	}

	ctx.addBuf('#')
	ctx.addOriginBuf('#')
	err := ErrInvalidToken(
		"a comment must be preceded by a space, and a scalar cannot begin with '#'",
		token.Invalid(string(ctx.obuf), s.pos()),
	)
	s.progressColumn(ctx, 1)
	ctx.clear()

	return err
}

func (s *Scanner) scanComment(ctx *Context) bool {
	// A comment starts a line or follows a space. The check used to run only
	// while a plain scalar was being buffered, so a '#' pressed up against
	// anything that had already been emitted -- a closing quote, a comma, a
	// bracket -- started a comment where YAML has none.
	// previousChar steps back over a byte order mark and returns 0 where
	// nothing stands before the cursor, so a comment opening the stream after
	// one is a comment that starts a line.
	if c := ctx.previousChar(); c != rune(0) && c != ' ' && c != '\t' && !isNewLineChar(c) {
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf('#')
	// As in scanTag: the offset takes the '#', and the comment's own position
	// is taken before the step.
	commentPos := s.pos()
	s.offset += s.progress(ctx, 1) // skip '#' character

	for idx, c := range ctx.src[ctx.idx:] {
		ctx.addOriginBuf(c)
		if !isNewLineChar(c) {
			continue
		}
		if ctx.previousChar() == '\\' {
			continue
		}
		value := ctx.source(ctx.idx, ctx.idx+idx)
		progress := utf8.RuneCountInString(value)

		// CRLF ends one line. progressLine steps over a single character, so
		// leaving the '\n' behind gives it to the token that follows, whose
		// leading whitespace is then read as another break: a comment closing
		// a CRLF line put the next token two lines down instead of one, and a
		// blank line appeared above the comment when the document was written
		// back.
		crlf := c == '\r' && ctx.idx+idx+1 < len(ctx.src) && ctx.src[ctx.idx+idx+1] == '\n'
		if crlf {
			ctx.addOriginBuf('\n')
		}

		ctx.addTokenValue(token.MakeComment(value, ctx.obuf, commentPos))
		s.progressColumn(ctx, progress)
		s.progressLine(ctx)
		if crlf {
			s.offset += s.progress(ctx, 1)
		}
		ctx.clear()
		return true
	}
	// document ends with comment.
	value := ctx.src[ctx.idx:]
	ctx.addTokenValue(token.MakeComment(value, ctx.obuf, commentPos))
	progress := utf8.RuneCountInString(value)
	s.progressColumn(ctx, progress)
	s.progressLine(ctx)
	ctx.clear()
	return true
}
