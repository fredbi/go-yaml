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

	if s.column != 1 {
		// c-directive opens a line, so a '%' standing anywhere else opens no directive.
		//
		// s.column is the test rather than s.indentNum, the two being different numbers: indentNum is 0 both for a '%'
		// opening a line and for the one in "a: %foo", and would admit the second.
		//
		// TODO: a tab does not advance s.column, so "\t%YAML 1.2" arrives here at column 1 and reads as a directive.
		// c-directive allows no s-separate in front of it, so this looks wrong. Confirm against the reference parser
		// in internal/testintegration/perlref, then refuse it here.
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
