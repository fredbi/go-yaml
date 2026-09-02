// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"testing"

	"github.com/go-openapi/go-yaml/internal/refparser"
	"github.com/go-openapi/go-yaml/internal/scanner"
)

// The stages the parser goes through, so the numbers subtract.
//
// BenchmarkScanTokens is the scan alone. ParserNew adds copying the tokens
// into the parser's blocks and grouping them. ParserParse adds building the
// tree.

func BenchmarkParserNew(b *testing.B) {
	forEachWorkload(b, func(b *testing.B, src []byte) {
		text := string(src)
		for b.Loop() {
			var s scanner.Scanner
			s.Init(text)
			if _, err := refparser.New(s.Tokens(), 0); err != nil {
				b.Fatal(err)
			}
			if err := s.Err(); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkParserParse(b *testing.B) {
	forEachWorkload(b, func(b *testing.B, src []byte) {
		text := string(src)
		for b.Loop() {
			var s scanner.Scanner
			s.Init(text)
			p, err := refparser.New(s.Tokens(), 0)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := p.Parse(); err != nil {
				b.Fatal(err)
			}
		}
	})
}
