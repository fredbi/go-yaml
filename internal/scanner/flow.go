// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import "github.com/go-openapi/go-yaml/token"

// scanFlowDash refuses a '-' that is neither a sequence entry nor the start of a scalar.
//
// A plain scalar may begin with '-' only when the character behind it can continue the scalar, and the characters
// structuring a flow collection cannot. A flow collection has no block sequence entries either, so such a '-' opens
// nothing at all.
//
// Example:
//
//	[-]        # refused: the ']' cannot continue a scalar
//	[-, -]     # refused, twice
//	[-1, -x]   # both read as scalars, '1' and 'x' continuing the '-'
//
// Outside a flow collection the '-' is a sequence entry, so this returns nil there and the entry scan claims it.
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
		token.Invalid(ctx.origin(), s.pos()),
	)
	s.progressColumn(ctx, 1)
	ctx.clear()

	return err
}

// enterFlow records what a flow collection's continuation lines must clear.
//
// Only the outermost one matters: a collection nested inside another is already past the indentation its parent required.
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
	ctx.addTokenValue(token.MakeMappingStart(ctx.origin(), s.pos()))
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
	ctx.addTokenValue(token.MakeMappingEnd(ctx.origin(), s.pos()))
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
	ctx.addTokenValue(token.MakeSequenceStart(ctx.origin(), s.pos()))
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
	ctx.addTokenValue(token.MakeSequenceEnd(ctx.origin(), s.pos()))
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
	ctx.addTokenValue(token.MakeCollectEntry(ctx.origin(), s.pos()))
	s.progressColumn(ctx, 1)
	ctx.clear()

	return true
}
