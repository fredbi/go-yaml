// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"testing"

	"github.com/go-openapi/go-yaml/parser"
)

// The lab parser measured against the production one, on the same workloads.
//
// Run the pair in one window and let benchstat subtract them:
//
//	go test -run XXX -bench 'WorkloadParse$|LabWorkloadParse$' -count 10 -benchmem ./... > ab.txt
//	benchstat -filter '.name:/^(Lab)?WorkloadParse$/' ab.txt
//
// One window, interleaved, never against a stored baseline: a single-shot
// comparison of this suite once drifted 13% on a benchmark the change under
// test could not touch.
//
// A number here means nothing until TestLabParserMatchesProduction passes. A
// candidate that parses differently is a different parser, not a faster one --
// see the package comment of internal/lab.

func BenchmarkLabWorkloadParse(b *testing.B) {
	forEachWorkload(b, func(b *testing.B, src []byte) {
		for b.Loop() {
			if _, err := parser.ParseBytes(src); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkLabWorkloadParseWithComments(b *testing.B) {
	forEachWorkload(b, func(b *testing.B, src []byte) {
		for b.Loop() {
			if _, err := parser.ParseBytes(src, parser.Comments()); err != nil {
				b.Fatal(err)
			}
		}
	})
}
