// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package benchmarks

import (
	"testing"

	goyaml3 "go.yaml.in/yaml/v3"

	yaml "github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
)

// BenchmarkWorkloads unmarshals each workload into any with this library and
// with go.yaml.in/yaml/v3, on the same input and into the same shape.
//
// Unmarshalling into any is the comparison both libraries can answer the same
// way: each parses the document into its own tree and walks it into Go values,
// and neither is asked for anything the other does not do.
func BenchmarkWorkloads(b *testing.B) {
	all, err := workloads.All()
	if err != nil {
		b.Fatal(err)
	}

	for _, w := range all {
		// Refuse to compare on a document one of them cannot read.
		var probe any
		if err := goyaml3.Unmarshal(w.Data, &probe); err != nil {
			b.Fatalf("%s: go.yaml.in/yaml/v3: %v", w.Name, err)
		}
		if err := yaml.Unmarshal(w.Data, &probe); err != nil {
			b.Fatalf("%s: go-openapi/go-yaml: %v", w.Name, err)
		}

		b.Run(w.Name, func(b *testing.B) {
			b.Run("goyaml3", func(b *testing.B) {
				b.SetBytes(int64(len(w.Data)))
				b.ReportAllocs()
				for b.Loop() {
					var v any
					if err := goyaml3.Unmarshal(w.Data, &v); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("goopenapi", func(b *testing.B) {
				b.SetBytes(int64(len(w.Data)))
				b.ReportAllocs()
				for b.Loop() {
					var v any
					if err := yaml.Unmarshal(w.Data, &v); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
