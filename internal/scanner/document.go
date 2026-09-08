// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"strings"
	"unicode/utf8"

	"github.com/go-openapi/go-yaml/token"
)

const (
	startDocMarker    = "---"
	endDocMarker      = "..."
	startDocMarkerLen = int32(len(startDocMarker))
)

func (s *Scanner) validateDocumentSeparatorMarker(ctx *Context, src string) error {
	if foundDocumentSeparatorMarker(src) {
		return ErrInvalidToken("found unexpected document separator", token.Invalid(ctx.origin(), s.pos()))
	}

	return nil
}

// foundDocumentSeparatorMarker tells when src opens with "---" or "...",
// as standing alone and not opening a longer scalar.
func foundDocumentSeparatorMarker(src string) bool {
	if !strings.HasPrefix(src, startDocMarker) && !strings.HasPrefix(src, endDocMarker) {
		return false
	}

	rest := src[startDocMarkerLen:]
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
	if ctx.repeatNum('-') != startDocMarkerLen {
		return false
	}
	if ctx.size > ctx.idx+startDocMarkerLen {
		c := ctx.src[ctx.idx+startDocMarkerLen]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			return false
		}
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addTokenValue(token.MakeDocumentHeader(ctx.origin()+startDocMarker, s.pos()))
	s.progressColumn(ctx, startDocMarkerLen)
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
	if ctx.repeatNum('.') != startDocMarkerLen {
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addTokenValue(token.MakeDocumentEnd(ctx.origin()+endDocMarker, s.pos()))
	s.progressColumn(ctx, 3)
	ctx.clear()

	return true
}
