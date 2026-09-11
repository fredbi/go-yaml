// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/corpus"
	"github.com/go-openapi/go-yaml/parser"
)

// The benchmarks below are the regression baseline for the parser, and measure this library only.
// Comparisons against other libraries live in the internal/benchmarks and internal/analysis modules,
// which take the dependencies for them.
//
// To compare two revisions:
//
//	go test -run=XXX -bench=. -benchmem -count=10 ./... > old.txt
//	# ... make the change ...
//	go test -run=XXX -bench=. -benchmem -count=10 ./... > new.txt
//	benchstat old.txt new.txt
//
// Every benchmark calls SetBytes, so throughput is reported in MB/s and sizes
// are comparable across shapes.

func BenchmarkParseBytes(b *testing.B) {
	corpus.ForEachDocument(b, func(b *testing.B, src []byte) {
		for b.Loop() {
			if _, err := parser.ParseBytes(src); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkParseBytesWithComments(b *testing.B) {
	corpus.ForEachDocument(b, func(b *testing.B, src []byte) {
		for b.Loop() {
			if _, err := parser.ParseBytes(src, parser.WithComments()); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkScanTokens in internal/analysis measures the scan alone, over the same workloads.

// BenchmarkRender measures turning an AST back into text, the other half of the round trip.
func BenchmarkRender(b *testing.B) {
	corpus.ForEachDocument(b, func(b *testing.B, src []byte) {
		file, err := parser.ParseBytes(src, parser.WithComments())
		require.NoError(b, err)
		b.ResetTimer()

		for b.Loop() {
			_ = file.String()
		}
	})
}
