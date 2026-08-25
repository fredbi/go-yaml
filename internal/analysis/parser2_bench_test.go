// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"testing"

	"github.com/go-openapi/go-yaml/parser2"
	"github.com/go-openapi/go-yaml/scanner"
)

// The stages parser2 goes through, so the numbers subtract.
//
// BenchmarkScanTokens is the scan alone. Parser2New adds copying the tokens
// into the parser's blocks and grouping them. Parser2Parse adds building the
// tree.

func BenchmarkParser2New(b *testing.B) {
	forEachWorkload(b, func(b *testing.B, src []byte) {
		text := string(src)
		for b.Loop() {
			var s scanner.Scanner
			s.Init(text)
			if _, err := parser2.New(s.Tokens(), 0); err != nil {
				b.Fatal(err)
			}
			if err := s.Err(); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkParser2Parse(b *testing.B) {
	forEachWorkload(b, func(b *testing.B, src []byte) {
		text := string(src)
		for b.Loop() {
			var s scanner.Scanner
			s.Init(text)
			p, err := parser2.New(s.Tokens(), 0)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := p.Parse(); err != nil {
				b.Fatal(err)
			}
		}
	})
}
