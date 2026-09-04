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
//	go test -run=XXX -bench=. -benchmem -count=10 ./internal/scanner/ > old.txt
//	# ... make the change ...
//	go test -run=XXX -bench=. -benchmem -count=10 ./internal/scanner/ > new.txt
//	benchstat old.txt new.txt
//
// Read ns/token before sec/op. A change that speeds up the scan and a change
// that emits fewer tokens both move sec/op, and only the first is what these
// are watching; ns/token separates them. B/op and allocs/op matter as much as
// either, because the token stream is where most of the object graph is born
// and GC costs more than the parse does today.

// BenchmarkScannerNextToken is the scanner's own baseline: NextToken and
// nothing kept.
//
// It is the call the parser makes -- reader.fill pulls one token at a time --
// and the one whose cost multiplies by the token count, so it is the number to
// move. Nothing is stored, so what it reports is the scan rather than the
// growth of a slice to put the result in.
func BenchmarkScannerNextToken(b *testing.B) {
	corpus.ForEachScanDocument(b, func(b *testing.B, src []byte) {
		var tokens int64
		for b.Loop() {
			var s scanner.Scanner
			s.Init(src)

			for {
				if _, ok := s.NextToken(); !ok {
					break
				}
				tokens++
			}
		}

		reportPerToken(b, tokens)
	})
}

// BenchmarkScannerScan measures Scan, which hands back a batch at a time.
//
// refparser reads a document this way and so do the tests, so it stays
// measured; the parser does not, which is why NextToken is the one above.
func BenchmarkScannerScan(b *testing.B) {
	corpus.ForEachScanDocument(b, func(b *testing.B, src []byte) {
		var tokens int64
		for b.Loop() {
			var s scanner.Scanner
			s.Init(src)

			for {
				batch, err := s.Scan()
				if errors.Is(err, io.EOF) {
					break
				}
				tokens += int64(len(batch))
			}
		}

		reportPerToken(b, tokens)
	})
}

// BenchmarkScannerCollect is Scan with the tokens kept, which is what a caller
// wanting the whole stream pays.
//
// The difference from BenchmarkScannerScan is the slice growing, not the
// scanner working. Keep the two apart: a change to the scanner should move one
// and leave the other alone.
func BenchmarkScannerCollect(b *testing.B) {
	corpus.ForEachScanDocument(b, func(b *testing.B, src []byte) {
		for b.Loop() {
			var (
				s      scanner.Scanner
				tokens token.Tokens
			)
			s.Init(src)

			for {
				batch, err := s.Scan()
				if errors.Is(err, io.EOF) {
					break
				}
				tokens.Add(batch...)
			}
		}
	})
}

// BenchmarkScanInit measures Init alone, which settles the source and resets
// the state without reading a token. It is the floor the others are measured
// against.
func BenchmarkScanInit(b *testing.B) {
	corpus.ForEachScanDocument(b, func(b *testing.B, src []byte) {
		for b.Loop() {
			var s scanner.Scanner
			s.Init(src)
		}
	})
}

// reportPerToken adds the per-token cost of a run that read tokens in total.
//
// sec/op is a document, and documents differ in how many tokens they hold, so
// it compares one shape against itself and nothing else. ns/token compares
// across shapes and says which construct is dear.
func reportPerToken(b *testing.B, tokens int64) {
	b.Helper()

	if tokens == 0 {
		return
	}

	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(tokens), "ns/token")
}
