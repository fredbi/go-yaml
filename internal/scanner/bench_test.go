package scanner_test

import (
	"errors"
	"io"
	"testing"

	"github.com/go-openapi/go-yaml/internal/corpus"
	"github.com/go-openapi/go-yaml/internal/scanner"
	"github.com/go-openapi/go-yaml/token"
)

// The benchmarks below are the regression baseline for the scanner. They
// measure this library only -- comparisons against other libraries live in the
// benchmarks and analysis modules, which take the dependencies for it.
//
// To compare two revisions:
//
//	go test -run=XXX -bench=. -benchmem -count=10 ./... > old.txt
//	# ... make the change ...
//	go test -run=XXX -bench=. -benchmem -count=10 ./... > new.txt
//	benchstat old.txt new.txt
//
// Two numbers matter here beyond wall time. Throughput, because the scanner is
// the one stage that has to touch every byte; and allocations per operation,
// because the token stream is where most of the object graph is born and GC
// costs more than the parse does today.

func BenchmarkScan(b *testing.B) {
	corpus.ForEachDocument(b, func(b *testing.B, src []byte) {
		text := string(src)

		for b.Loop() {
			var (
				s      scanner.Scanner
				tokens token.Tokens
			)
			s.Init(text)

			for {
				subTokens, err := s.Scan()
				if errors.Is(err, io.EOF) {
					break
				}
				tokens.Add(subTokens...)
			}
		}
	})
}

// BenchmarkScanInit measures Init alone. It is the whole-document []rune
// conversion, which is 4x the source in memory and the reason the scanner
// cannot yet read from an io.Reader.
func BenchmarkScanInit(b *testing.B) {
	corpus.ForEachDocument(b, func(b *testing.B, src []byte) {
		text := string(src)

		for b.Loop() {
			var s scanner.Scanner
			s.Init(text)
		}
	})
}
