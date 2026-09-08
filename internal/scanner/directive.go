// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import "github.com/go-openapi/go-yaml/token"

// scanDirective reads the '%' opening a directive, c-directive, and returns false for a '%' that opens no directive.
//
// The '%' becomes a token on its own, and what names the directive follows as the next tokens: "%YAML 1.2" reads as a
// Directive, a String and a Float.
//
// Example: "a: %foo" and "  %YAML 1.2" are both refused, a plain scalar not being allowed to begin with '%'.
func (s *Scanner) scanDirective(ctx *Context) bool {
	if ctx.existsBuffer() {
		return false
	}

	if s.indentNum != 0 || s.column != 1 {
		// c-directive opens a line and takes no separation in front of it, so a '%' standing anywhere else opens no
		// directive.
		//
		// Both counters are needed, and neither catches what the other does. indentNum is 0 for the '%' in "a: %foo",
		// which the column refuses. A tab raises indentNum without advancing the column, so "\t%YAML 1.2" passes the
		// column test and indentNum refuses it.
		//
		// scanDocumentStart and scanDocumentEnd guard "---" and "..." with the same pair.
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf('%')
	ctx.addTokenValue(token.MakeDirective(ctx.origin(), s.pos()))
	s.progressColumn(ctx, 1)
	ctx.clear()
	s.isDirective = true

	return true
}
