package scanner

import "github.com/go-openapi/go-yaml/token"

func (s *Scanner) scanDirective(ctx *Context) bool {
	if ctx.existsBuffer() {
		return false
	}
	if s.column != 1 {
		// c-directive opens a line and nothing else does, so a '%' anywhere
		// else is not one. Measured by the indentation count before, which is
		// zero for a '%' that opens a line and also for one written after a
		// key on a line that carries no indentation of its own.
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf('%')
	ctx.addTokenValue(token.MakeDirective(ctx.obuf, s.pos()))
	s.progressColumn(ctx, 1)
	ctx.clear()
	s.isDirective = true
	return true
}
