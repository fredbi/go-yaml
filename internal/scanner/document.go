package scanner

import (
	"strings"
	"unicode/utf8"

	"github.com/go-openapi/go-yaml/token"
)

func (s *Scanner) validateDocumentSeparatorMarker(ctx *Context, src string) error {
	if s.foundDocumentSeparatorMarker(src) {
		return ErrInvalidToken("found unexpected document separator", token.Invalid(string(ctx.obuf), s.pos()))
	}

	return nil
}

// foundDocumentSeparatorMarker reports that src opens with "---" or "...",
// standing alone rather than beginning a longer scalar.
func (s *Scanner) foundDocumentSeparatorMarker(src string) bool {
	if !strings.HasPrefix(src, "---") && !strings.HasPrefix(src, "...") {
		return false
	}
	rest := src[3:]
	if rest == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(rest)

	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func (s *Scanner) scanDocumentStart(ctx *Context) bool {
	if s.indentNum != 0 {
		return false
	}
	if s.column != 1 {
		return false
	}
	if ctx.repeatNum('-') != 3 {
		return false
	}
	if ctx.size > ctx.idx+3 {
		c := ctx.src[ctx.idx+3]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			return false
		}
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addTokenValue(token.MakeDocumentHeader(string(ctx.obuf)+"---", s.pos()))
	s.progressColumn(ctx, 3)
	ctx.clear()
	s.clearState()

	return true
}

func (s *Scanner) scanDocumentEnd(ctx *Context) bool {
	if s.indentNum != 0 {
		return false
	}
	if s.column != 1 {
		return false
	}
	if ctx.repeatNum('.') != 3 {
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addTokenValue(token.MakeDocumentEnd(string(ctx.obuf)+"...", s.pos()))
	s.progressColumn(ctx, 3)
	ctx.clear()
	return true
}
