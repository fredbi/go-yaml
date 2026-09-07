// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

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
	ctx.addTokenValue(token.MakeAnchor(ctx.origin(), s.pos()))
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
	ctx.addTokenValue(token.MakeAlias(ctx.origin(), s.pos()))
	s.progressColumn(ctx, 1)
	s.isAlias = true
	ctx.clear()
	return true, nil
}

// validateAnchorName checks the name an '&' or '*' introduces.
//
// ns-anchor-name is one ns-anchor-char or more, so an indicator with nothing after it names nothing.
// "& e" used to read as the empty document, and is refused now.
//
// A flow collection opening straight onto the name is the other half.
// s-separate must separate a property from whatever follows it, and an alias is a whole node with no room for another
// behind it.
// "&a []" is the empty sequence carrying an anchor. "&a[]" used to read as nothing at all, losing a value instead of
// merely admitting a document.
func (s *Scanner) validateAnchorName(ctx *Context, what string) error {
	start := ctx.idx + 1
	end := anchorNameEnd(ctx.src, start)

	switch {
	case end == start:
		return ErrInvalidToken(what+" must be followed by a name", token.Invalid(ctx.origin(), s.pos()))
	case end < int32(len(ctx.src)) && (ctx.src[end] == '[' || ctx.src[end] == '{'):
		return ErrInvalidToken(what+" must be separated from the node that follows it", token.Invalid(ctx.origin(), s.pos()))
	default:
		return nil
	}
}

// anchorNameEnd returns where the name starting at start ends.
//
// ns-anchor-char is ns-char less the flow indicators, so a name runs up to whitespace, a line break, the end of the
// input, or one of ',', '[', ']', '{' or '}'.
// A ':' is none of those and belongs to the name, so "{&a: b}" anchors a node named "a:".
func anchorNameEnd(src string, start int32) int32 {
	// Every character that ends a name is ASCII, so this can walk bytes: no byte of a multi-byte character is one of them.
	end := start
	for end < int32(len(src)) && !endsAnchorName(rune(src[end])) {
		end++
	}

	return end
}

// inAnchorName reports whether c belongs to the anchor or alias name now being
// read.
//
// ns-anchor-char is ns-char less the flow indicators, so every character that
// does not end a name is one of its characters -- "@", "`", "#", a quote and a
// "%" among them. Each of those opens a token of its own elsewhere, and the
// scan steps that claim them ask this first: "&@" is an anchor named "@" and
// not a reserved character, which is what libfyaml 1.0.0b1 and the reference
// parser read it as.
func (s *Scanner) inAnchorName(c rune) bool {
	return (s.isAnchor || s.isAlias) && !endsAnchorName(c)
}

func endsAnchorName(c rune) bool {
	switch c {
	case ' ', '\t', '\r', '\n', ',', '[', ']', '{', '}':
		return true
	default:
		return false
	}
}
