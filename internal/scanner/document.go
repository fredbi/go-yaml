// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"strings"
	"unicode/utf8"

	"github.com/go-openapi/go-yaml/token"
)

const (
	docMarker    = "---"
	altDocMarker = "..."
	docMarkerLen = int32(len(docMarker))
)

func (s *Scanner) validateDocumentSeparatorMarker(ctx *Context, src string) error {
	if foundDocumentSeparatorMarker(src) {
		return ErrInvalidToken("found unexpected document separator", token.Invalid(ctx.origin(), s.pos()))
	}

	return nil
}

// foundDocumentSeparatorMarker reports that src opens with "---" or "...", standing alone rather than beginning a
// longer scalar.
func foundDocumentSeparatorMarker(src string) bool {
	if !strings.HasPrefix(src, docMarker) && !strings.HasPrefix(src, altDocMarker) {
		return false
	}
	rest := src[docMarkerLen:]
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
	if ctx.repeatNum('-') != docMarkerLen {
		return false
	}
	if ctx.size > ctx.idx+docMarkerLen {
		c := ctx.src[ctx.idx+docMarkerLen]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			return false
		}
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addTokenValue(token.MakeDocumentHeader(ctx.origin()+docMarker, s.pos()))
	s.progressColumn(ctx, docMarkerLen)
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
	if ctx.repeatNum('.') != docMarkerLen {
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addTokenValue(token.MakeDocumentEnd(ctx.origin()+altDocMarker, s.pos()))
	s.progressColumn(ctx, 3)
	ctx.clear()
	return true
}
