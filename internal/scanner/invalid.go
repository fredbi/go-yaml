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
	if ctx.existsBuffer() && !s.isAnchor && !s.isAlias {
		return nil
	}

	ctx.addBuf(c)
	ctx.addOriginBuf(c)
	err := ErrInvalidToken(fmt.Sprintf("a plain scalar cannot begin with %q", c), token.Invalid(ctx.origin(), s.pos()))
	s.progressColumn(ctx, 1)

	return err
}

func (s *Scanner) scanReservedChar(ctx *Context, c rune) error {
	if ctx.existsBuffer() {
		return nil
	}

	ctx.addBuf(c)
	ctx.addOriginBuf(c)
	err := ErrInvalidToken(fmt.Sprintf("%q is a reserved character", c), token.Invalid(ctx.origin(), s.pos()))
	s.progressColumn(ctx, 1)
	ctx.clear()

	return err
}
