// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"testing"

	"github.com/go-openapi/go-yaml/internal/scanner"
	"github.com/go-openapi/go-yaml/token"
)

// The four ways to read a source through the scanner, over the same workloads.
// Nothing here holds the tokens: what is measured is the cost of producing
// them, so the numbers say what the scanner itself spends.
//
// Scan and Next hand over *token.Token, so every token has to be kept in a
// block for its address to hold. Tokens and NextToken hand over values, so the
// scanner reuses one block whatever the document's length.

func BenchmarkScanNextToken(b *testing.B) {
	forEachWorkload(b, func(b *testing.B, src []byte) {
		text := string(src)
		for b.Loop() {
			var s scanner.Scanner
			s.Init([]byte(text))
			for {
				if _, ok := s.NextToken(); !ok {
					break
				}
			}
		}
	})
}

func BenchmarkScanTokens(b *testing.B) {
	forEachWorkload(b, func(b *testing.B, src []byte) {
		text := string(src)
		for b.Loop() {
			var s scanner.Scanner
			s.Init([]byte(text))
			var last token.Type
			for tk := range s.Tokens() {
				last = tk.Type
			}
			_ = last
		}
	})
}
