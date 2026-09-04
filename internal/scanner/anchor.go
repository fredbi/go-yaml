package scanner

import "github.com/go-openapi/go-yaml/token"

func (s *Scanner) scanAnchor(ctx *Context) (bool, error) {
	if ctx.existsBuffer() {
		return false, nil
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf('&')
	if err := s.validateAnchorName(ctx, "an anchor"); err != nil {
		s.progressColumn(ctx, 1)

		return false, err
	}
	ctx.addTokenValue(token.MakeAnchor(ctx.obuf, s.pos()))
	s.progressColumn(ctx, 1)
	s.isAnchor = true
	ctx.clear()
	return true, nil
}

func (s *Scanner) scanAlias(ctx *Context) (bool, error) {
	if ctx.existsBuffer() {
		return false, nil
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf('*')
	if err := s.validateAnchorName(ctx, "an alias"); err != nil {
		s.progressColumn(ctx, 1)

		return false, err
	}
	ctx.addTokenValue(token.MakeAlias(ctx.obuf, s.pos()))
	s.progressColumn(ctx, 1)
	s.isAlias = true
	ctx.clear()
	return true, nil
}

// validateAnchorName checks the name an '&' or '*' introduces.
//
// ns-anchor-name is one ns-anchor-char or more, so an indicator with nothing
// after it names nothing -- "& e" used to read as the empty document rather
// than being refused.
//
// A flow collection opening straight onto the name is the other half: what
// follows a property has to be separated from it by s-separate, and an alias is
// a whole node with no room for another behind it. "&a []" is the empty
// sequence with an anchor on it, where "&a[]" used to read as nothing at all --
// losing a value rather than merely admitting a document.
func (s *Scanner) validateAnchorName(ctx *Context, what string) error {
	start := ctx.idx + 1
	end := anchorNameEnd(ctx.src, start)

	switch {
	case end == start:
		return ErrInvalidToken(what+" must be followed by a name", token.Invalid(string(ctx.obuf), s.pos()))
	case end < len(ctx.src) && (ctx.src[end] == '[' || ctx.src[end] == '{'):
		return ErrInvalidToken(what+" must be separated from the node that follows it", token.Invalid(string(ctx.obuf), s.pos()))
	default:
		return nil
	}
}

// anchorNameEnd returns where the name starting at start ends.
//
// ns-anchor-char is ns-char less the flow indicators, so a name runs up to
// whitespace, a line break, the end of the input, or one of ',', '[', ']', '{'
// or '}'. A ':' is none of those and belongs to the name, which is why
// "{&a: b}" anchors a node named "a:".
func anchorNameEnd(src string, start int) int {
	// Every character that ends a name is ASCII, so this can walk bytes: no
	// byte of a multi-byte character is one of them.
	end := start
	for end < len(src) && !endsAnchorName(rune(src[end])) {
		end++
	}

	return end
}
