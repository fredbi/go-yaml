// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"fmt"

	"github.com/go-openapi/go-yaml/token"
)

// scanPlainFirst refuses an indicator that no other scan function claimed.
//
// ns-plain-first(c) is ns-char less c-indicator, so a plain scalar cannot open on an indicator.
// Once a scalar is under way those characters are ordinary text: "a: b}c" holds a '}' and means it.
// So this refuses only an indicator that would open a token: a '}' outside a flow mapping, or a ',' outside a flow
// collection.
//
// A buffer holding an anchor or an alias name is not a scalar under way.
// Such a name ends at a flow indicator, so an indicator arriving there opens the next token and does not continue the
// name.
// Inside a flow collection another scan function claims the indicator first, which keeps "[&a, b]", an anchor on an
// empty node, apart from "&a," at the root.
func (s *Scanner) scanPlainFirst(ctx *Context, c rune) error {
	// The two guards do not fold into the "existsBuffer() || inAnchorName(c)" its neighbors use, and the difference is
	// the point: a name ended by c leaves a buffer behind, and c still opens the next token. "&a," is refused there.
	if s.inAnchorName(c) {
		return nil
	}
	if ctx.existsBuffer() && !s.isAnchor && !s.isAlias {
		return nil
	}

	return s.refuse(ctx, c, fmt.Sprintf("a plain scalar cannot begin with %q", c))
}

// refuse buffers c, steps over it, and returns the error naming it.
//
// The five scan steps that refuse a character all end this way. ctx.origin() is read before the step, so the token
// carries the text as it stands in the source.
func (s *Scanner) refuse(ctx *Context, c rune, msg string) error {
	ctx.addBuf(c)
	ctx.addOriginBuf(c)
	err := ErrInvalidToken(msg, token.Invalid(ctx.origin(), s.pos()))
	s.progressColumn(ctx, 1)

	return err
}

func (s *Scanner) scanReservedChar(ctx *Context, c rune) error {
	if ctx.existsBuffer() || s.inAnchorName(c) {
		return nil
	}

	return s.refuse(ctx, c, fmt.Sprintf("%q is a reserved character", c))
}
