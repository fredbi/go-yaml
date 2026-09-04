package scanner

import (
	"fmt"
	"unicode/utf8"

	"github.com/go-openapi/go-yaml/token"
)

func (s *Scanner) scanTag(ctx *Context) (bool, error) {
	if ctx.existsBuffer() || s.isDirective {
		return false, nil
	}

	ctx.addOriginBuf('!')
	// The offset counts the bytes the cursor has crossed, so it takes the '!'
	// too. Left out, it stayed one byte behind for the rest of the document and
	// every token after this one was reported a byte early. tagPos is taken
	// before the step, where the tag's own text begins.
	tagPos := s.pos()
	s.progress(ctx, 1) // skip '!' character

	// A verbatim tag, "!<...>", holds a URI and takes it as written: the
	// characters a shorthand may not contain are ordinary inside the brackets.
	verbatim := ctx.currentChar() == '<'

	// idx counts bytes into the source; progress counts the characters the
	// column has to advance by, which is not the same thing.
	var progress int
	for idx, c := range ctx.src[ctx.idx:] {
		progress++
		if verbatim {
			ctx.addOriginBuf(c)
			if c == '>' {
				verbatim = false
			}

			continue
		}
		switch c {
		case ' ':
			ctx.addOriginBuf(c)
			value := ctx.source(ctx.idx-1, ctx.idx+idx)
			ctx.addTokenValue(token.MakeTag(value, ctx.origin(), tagPos))
			s.progressColumn(ctx, utf8.RuneCountInString(value))
			ctx.clear()
			return true, nil
		case ',':
			if s.startedFlowSequenceNum > 0 || s.startedFlowMapNum > 0 {
				value := ctx.source(ctx.idx-1, ctx.idx+idx)
				ctx.addTokenValue(token.MakeTag(value, ctx.origin(), tagPos))
				s.progressColumn(ctx, utf8.RuneCountInString(value)-1) // progress column before collect-entry for scanning it at scanFlowEntry function.
				ctx.clear()
				return true, nil
			}
			// Outside a flow collection nothing ends the tag here, and a ',' is
			// not a character a tag may contain: it has to be percent-encoded.
			ctx.addOriginBuf(c)
			s.progressColumn(ctx, progress)

			return false, ErrInvalidToken(fmt.Sprintf("found invalid tag character %q", c), token.Invalid(string(ctx.origin()), s.pos()))
		case '\n', '\r':
			ctx.addOriginBuf(c)
			value := ctx.source(ctx.idx-1, ctx.idx+idx)
			ctx.addTokenValue(token.MakeTag(value, ctx.origin(), tagPos))
			s.progressColumn(ctx, utf8.RuneCountInString(value)-1) // progress column before new-line-char for scanning new-line-char at scanNewLine function.
			ctx.clear()
			return true, nil
		case '}', ']':
			if s.startedFlowSequenceNum > 0 || s.startedFlowMapNum > 0 {
				// The closer ends the collection the tag stands in, so it ends
				// the tag: "[!]" is the non-specific tag on the empty node and
				// not a tag whose name is "]".
				value := ctx.source(ctx.idx-1, ctx.idx+idx)
				ctx.addTokenValue(token.MakeTag(value, ctx.origin(), tagPos))
				s.progressColumn(ctx, utf8.RuneCountInString(value)-1) // progress column before the closer so it is scanned on its own

				ctx.clear()

				return true, nil
			}

			ctx.addOriginBuf(c)
			s.progressColumn(ctx, progress)
			invalidMsg := fmt.Sprintf("found invalid tag character %q", c)
			invalidTk := token.Invalid(string(ctx.origin()), s.pos())

			return false, ErrInvalidToken(invalidMsg, invalidTk)
		case '{':
			ctx.addOriginBuf(c)
			s.progressColumn(ctx, progress)
			invalidMsg := fmt.Sprintf("found invalid tag character %q", c)
			invalidTk := token.Invalid(string(ctx.origin()), s.pos())

			return false, ErrInvalidToken(invalidMsg, invalidTk)
		default:
			ctx.addOriginBuf(c)
		}
	}
	s.progressColumn(ctx, progress)
	ctx.clear()
	return true, nil
}
