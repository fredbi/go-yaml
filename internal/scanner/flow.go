package scanner

import "github.com/go-openapi/go-yaml/token"

// scanFlowDash reports a '-' that is neither a sequence entry nor the start of
// a scalar.
//
// A plain scalar may begin with '-' only when what follows can continue it. In
// a flow collection the characters that structure the collection cannot, so
// "[-]" and "[-, -]" hold no scalar at all -- they used to be read as the
// one-character string "-".
func (s *Scanner) scanFlowDash(ctx *Context) error {
	if ctx.existsBuffer() || !s.isFlowMode() {
		return nil
	}

	switch ctx.nextChar() {
	case ',', '[', ']', '{', '}':
	default:
		return nil
	}

	ctx.addBuf('-')
	ctx.addOriginBuf('-')
	err := ErrInvalidToken(
		"'-' is not a scalar, and a flow collection has no sequence entries",
		token.Invalid(string(ctx.obuf), s.pos()),
	)
	s.progressColumn(ctx, 1)
	ctx.clear()

	return err
}

// enterFlow records what a flow collection's continuation lines must clear.
//
// Only the outermost one matters: a collection nested inside another is already
// past the indentation its parent required.
func (s *Scanner) enterFlow() {
	if s.isFlowMode() {
		return
	}
	s.flowIndent = s.contentIndent()
}

func (s *Scanner) isFlowMode() bool {
	if s.startedFlowSequenceNum > 0 {
		return true
	}
	if s.startedFlowMapNum > 0 {
		return true
	}
	return false
}

func (s *Scanner) scanFlowMapStart(ctx *Context) bool {
	if ctx.existsBuffer() && !s.isFlowMode() {
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf('{')
	ctx.addTokenValue(token.MakeMappingStart(string(ctx.obuf), s.pos()))
	s.enterFlow()
	s.startedFlowMapNum++
	s.progressColumn(ctx, 1)
	ctx.clear()
	return true
}

func (s *Scanner) scanFlowMapEnd(ctx *Context) bool {
	if s.startedFlowMapNum <= 0 {
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf('}')
	ctx.addTokenValue(token.MakeMappingEnd(string(ctx.obuf), s.pos()))
	s.startedFlowMapNum--
	s.progressColumn(ctx, 1)
	ctx.clear()
	return true
}

func (s *Scanner) scanFlowArrayStart(ctx *Context) bool {
	if ctx.existsBuffer() && !s.isFlowMode() {
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf('[')
	ctx.addTokenValue(token.MakeSequenceStart(string(ctx.obuf), s.pos()))
	s.enterFlow()
	s.startedFlowSequenceNum++
	s.progressColumn(ctx, 1)
	ctx.clear()
	return true
}

func (s *Scanner) scanFlowArrayEnd(ctx *Context) bool {
	if ctx.existsBuffer() && s.startedFlowSequenceNum <= 0 {
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf(']')
	ctx.addTokenValue(token.MakeSequenceEnd(string(ctx.obuf), s.pos()))
	s.startedFlowSequenceNum--
	s.progressColumn(ctx, 1)
	ctx.clear()
	return true
}

func (s *Scanner) scanFlowEntry(ctx *Context, c rune) bool {
	if s.startedFlowSequenceNum <= 0 && s.startedFlowMapNum <= 0 {
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf(c)
	ctx.addTokenValue(token.MakeCollectEntry(string(ctx.obuf), s.pos()))
	s.progressColumn(ctx, 1)
	ctx.clear()
	return true
}
