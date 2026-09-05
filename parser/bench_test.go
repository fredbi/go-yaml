package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/corpus"
	"github.com/go-openapi/go-yaml/parser"
)

// The benchmarks below are the regression baseline for the parser. They measure
// this library only -- comparisons against other libraries live in the
// benchmarks and analysis modules, which take the dependencies for it.
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

// Parsing and tokenizing are measured apart in internal/analysis, over the
// workloads: BenchmarkScanTokens is the scan, BenchmarkParserNew adds the
// grouping, and BenchmarkParserParse adds the tree. parser.New reads an
// iterator rather than a slice, so there is no token slice to parse twice.

// BenchmarkRender measures turning an AST back into text, which is half of the
// round trip and is not otherwise covered.
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
