// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"testing"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
	"github.com/go-openapi/go-yaml/parser"
	"github.com/go-openapi/go-yaml/parser2"
)

// The three stages a document goes through, benchmarked over the same
// workloads so the numbers subtract.
//
// Tokenize is the scanner and the lexer. Parse is Tokenize plus the token
// grouping plus the tree build, because parser.ParseBytes does all three and
// that is the call a consumer makes. Decode is Parse plus construction into
// Go values.
//
// b.SetBytes reports MB/s, which is the number to compare across workloads:
// ns/op says nothing when golang_source is seven times the size of
// canada_geometry.

func BenchmarkWorkloadTokenize(b *testing.B) {
	forEachWorkload(b, func(b *testing.B, src []byte) {
		text := string(src)
		for b.Loop() {
			_ = tokenize(b, text)
		}
	})
}

func BenchmarkWorkloadParse(b *testing.B) {
	forEachWorkload(b, func(b *testing.B, src []byte) {
		for b.Loop() {
			if _, err := parser.ParseBytes(src, 0); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkWorkloadParse2(b *testing.B) {
	forEachWorkload(b, func(b *testing.B, src []byte) {
		for b.Loop() {
			if _, err := parser2.ParseBytes(src, 0); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkWorkloadParseWithComments(b *testing.B) {
	forEachWorkload(b, func(b *testing.B, src []byte) {
		for b.Loop() {
			if _, err := parser.ParseBytes(src, parser.ParseComments); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkWorkloadDecode(b *testing.B) {
	forEachWorkload(b, func(b *testing.B, src []byte) {
		for b.Loop() {
			var v any
			if err := yaml.Unmarshal(src, &v); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// forEachWorkload runs one measurement over every workload, reporting bytes so
// the result comes out as MB/s.
func forEachWorkload(b *testing.B, measure func(*testing.B, []byte)) {
	b.Helper()

	all, err := workloads.All()
	if err != nil {
		b.Fatal(err)
	}

	for _, workload := range all {
		b.Run(workload.Name, func(b *testing.B) {
			b.SetBytes(int64(len(workload.Data)))
			b.ReportAllocs()
			b.ResetTimer()
			measure(b, workload.Data)
		})
	}
}
