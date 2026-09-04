// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"iter"
	"testing"

	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
	"github.com/go-openapi/go-yaml/internal/scanner"
	"github.com/go-openapi/go-yaml/token"
)

// BenchmarkPullOverhead prices iter.Pull against reading the same stream
// push-style.
//
// It decides how the descent may be fed. The grouping passes are iter.Seq,
// which pushes, and the parser reads forward one token at a time, which pulls.
// iter.Pull bridges the two, and the bridge is a coroutine: every token costs a
// switch out and a switch back.
//
// Measured on golang_source, 293,142 tokens: 99.5ms push against 117.8ms pull,
// +18.4%, near 62ns a token. That is one Pull over the scanner alone, with none
// of the ten grouping passes above it -- and it is the likely reason the first
// attempt at streaming the group layer cost 20-29% of the time.
//
// So a streaming parser cannot bridge with iter.Pull. The passes have to pull
// rather than push: each one a func() (*Token, bool) reading from the one below
// it, composed by ordinary calls.
func BenchmarkPullOverhead(b *testing.B) {
	w, err := workloads.ByName("golang_source")
	if err != nil {
		b.Fatal(err)
	}
	src := string(w.Data)

	b.Run("push", func(b *testing.B) {
		for b.Loop() {
			var n int
			for tk := range scannerSeq(src) {
				if tk != nil {
					n++
				}
			}
			_ = n
		}
	})

	b.Run("pull", func(b *testing.B) {
		for b.Loop() {
			next, stop := iter.Pull(scannerSeq(src))
			var n int
			for {
				tk, ok := next()
				if !ok {
					break
				}
				if tk != nil {
					n++
				}
			}
			stop()
			_ = n
		}
	})
}

// scannerSeq is the scanner's tokens as a push sequence, which is the shape the
// grouping passes read.
func scannerSeq(src string) iter.Seq[*token.Token] {
	return func(yield func(*token.Token) bool) {
		var s scanner.Scanner
		s.Init([]byte(src))

		for tk := range s.Tokens() {
			held := tk
			if !yield(&held) {
				return
			}
		}
	}
}
