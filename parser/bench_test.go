package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/lexer"
	"github.com/go-openapi/go-yaml/parser"
	"github.com/go-openapi/go-yaml/testdata/corpus"
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
			if _, err := parser.ParseBytes(src, 0); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkParseBytesWithComments(b *testing.B) {
	corpus.ForEachDocument(b, func(b *testing.B, src []byte) {
		for b.Loop() {
			if _, err := parser.ParseBytes(src, parser.ParseComments); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkParseTokens separates parsing from tokenizing, so that a change to
// one is not read as a change to the other.
func BenchmarkParseTokens(b *testing.B) {
	corpus.ForEachDocument(b, func(b *testing.B, src []byte) {
		tokens := lexer.Tokenize(string(src))
		b.ResetTimer()

		for b.Loop() {
			if _, err := parser.Parse(tokens, 0); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkRender measures turning an AST back into text, which is half of the
// round trip and is not otherwise covered.
func BenchmarkRender(b *testing.B) {
	corpus.ForEachDocument(b, func(b *testing.B, src []byte) {
		file, err := parser.ParseBytes(src, parser.ParseComments)
		require.NoError(b, err)
		b.ResetTimer()

		for b.Loop() {
			_ = file.String()
		}
	})
}
