// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"runtime"
	"testing"

	"github.com/go-openapi/testify/v2/require"

	yaml "github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
	"github.com/go-openapi/go-yaml/parser"
)

// TestToJSONPeak measures converting a document to JSON against the parse it
// cannot get under.
//
// Converting is the shape most callers have: read a node, write something, move
// on, never look back. [yaml.ToJSON] is written that way -- it walks the parse
// and appends to one buffer, keeping no node and only the text of the anchors an
// alias may still name.
//
// Peak is the number. The output stands at the end either way, so what the
// conversion retains afterwards says nothing; the question is how much stood at
// once on the way. The floor is a parse that converts nothing: whatever share of
// the peak that is, no consumer can get under it while the parser holds what it
// holds. Run with -v for the table.
func TestToJSONPeak(t *testing.T) {
	all, err := workloads.All()
	require.NoError(t, err)

	t.Logf("%-19s %8s %9s %10s %9s %7s", "workload", "source", "json", "parse only", "convert", "floor%")

	for _, w := range all {
		out, err := yaml.ToJSON(w.Data)
		require.NoError(t, err)

		convert := peakLiveMax(peakLiveRuns(), func() {
			b, err := yaml.ToJSON(w.Data)
			require.NoError(t, err)
			runtime.KeepAlive(b)
		})

		bare := peakLiveMax(peakLiveRuns(), func() {
			file, err := parser.ParseBytes(w.Data)
			require.NoError(t, err)
			runtime.KeepAlive(file)
		})

		t.Logf("%-19s %7dK %8dK %9dK %8dK %6.0f%%",
			w.Name, len(w.Data)/1024, len(out)/1024,
			bare/1024, convert/1024,
			100*float64(bare)/float64(convert))
	}
}

// BenchmarkToJSON is the pair to run interleaved for allocation figures.
func BenchmarkToJSON(b *testing.B) {
	forEachWorkload(b, func(b *testing.B, src []byte) {
		for b.Loop() {
			if _, err := yaml.ToJSON(src); err != nil {
				b.Fatal(err)
			}
		}
	})
}
