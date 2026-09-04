// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner_test

import (
	"strconv"
	"testing"

	"github.com/go-openapi/go-yaml/internal/scanner"
)

// BenchmarkScannerWorkloads reads the documents in the analysis workloads,
// which are what people write rather than what a generator produces.
//
// The corpus shapes are shallow: every one but deepindent opens its lines with
// three spaces or fewer, where these average 14.5 and golang_source 21.9. A
// change that pays on a long run is worth what this says it is worth, not what
// the shapes say.
func BenchmarkScannerWorkloads(b *testing.B) {
	for i, text := range workloadDocs(b) {
		src := []byte(text)
		b.Run(strconv.Itoa(i), func(b *testing.B) {
			b.SetBytes(int64(len(src)))
			b.ReportAllocs()
			for b.Loop() {
				var s scanner.Scanner
				s.Init(src)
				for {
					if _, ok := s.NextToken(); !ok {
						break
					}
				}
			}
		})
	}
}
