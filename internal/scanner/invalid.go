package scanner

import (
	"fmt"

	"github.com/go-openapi/go-yaml/token"
)

// scanPlainFirst reports an indicator that no scan function claimed.
//
// ns-plain-first(c) is ns-char less c-indicator, so a plain scalar cannot open
// on one of them. Inside a scalar they are ordinary characters -- "a: b}c"
// holds a '}' and means it -- so this refuses only one that would start a
// token, which is what a '}' outside a flow mapping or a ',' outside a flow
// collection does.
//
// TODO: this comment is not understandable.
//
// A buffer holding an anchor or alias name is not a scalar in progress: the
// name ends at a flow indicator, so one arriving there starts the next token
// rather than continuing this one. Inside a flow collection the indicator is
// claimed before it reaches here, which is what keeps "[&a, b]" -- an anchor on
// an empty node -- apart from "&a," at the root.
func (s *Scanner) scanPlainFirst(ctx *Context, c rune) error {
	if ctx.existsBuffer() && !s.isAnchor && !s.isAlias {
		return nil
	}

	ctx.addBuf(c)
	ctx.addOriginBuf(c)
	err := ErrInvalidToken(fmt.Sprintf("a plain scalar cannot begin with %q", c), token.Invalid(string(ctx.obuf), s.pos()))
	s.progressColumn(ctx, 1)

	return err
}

func (s *Scanner) scanReservedChar(ctx *Context, c rune) error {
	if ctx.existsBuffer() {
		return nil
	}

	ctx.addBuf(c)
	ctx.addOriginBuf(c)
	err := ErrInvalidToken(fmt.Sprintf("%q is a reserved character", c), token.Invalid(string(ctx.obuf), s.pos()))
	s.progressColumn(ctx, 1)
	ctx.clear()

	return err
}
