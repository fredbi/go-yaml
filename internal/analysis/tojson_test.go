// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"runtime"
	"testing"

	"github.com/go-openapi/testify/v2/require"

	yaml "github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
	"github.com/go-openapi/go-yaml/internal/lab"
	"github.com/go-openapi/go-yaml/internal/lab/labparser"
)

// TestToJSONPeak measures converting a document to JSON both ways.
//
// Converting is the shape most callers have: read a node, write something,
// move on, never look back. yaml.ToJSON gives such a caller the whole document
// twice over -- it unmarshals into an ordered map and marshals that, so tokens,
// tree and Go values all stand at once -- and lab.ToJSONProgressive writes each
// node as the parser finishes it and keeps none of them.
//
// Peak is the number. Both hold the output at the end, so what either retains
// afterwards says nothing; the question is how much stood at once on the way.
// Run with -v for the table.
func TestToJSONPeak(t *testing.T) {
	all, err := workloads.All()
	require.NoError(t, err)

	t.Logf("%-19s %8s %9s %10s %9s %11s %7s %9s",
		"workload", "source", "json", "parse only", "codec", "progressive", "peak", "floor%")

	for _, w := range all {
		out, err := yaml.ToJSON(w.Data)
		require.NoError(t, err)

		full := peakLiveMax(peakLiveRuns(), func() {
			b, err := yaml.ToJSON(w.Data)
			require.NoError(t, err)
			runtime.KeepAlive(b)
		})

		progressive := peakLiveMax(peakLiveRuns(), func() {
			b, err := lab.ToJSONProgressive(w.Data)
			require.NoError(t, err)
			runtime.KeepAlive(b)
		})

		// What the parse alone stands up, converting nothing. Whatever share of
		// the progressive peak this is, no consumer can get under it.
		bare := peakLiveMax(peakLiveRuns(), func() {
			file, err := labparser.ParseBytes(w.Data, 0)
			require.NoError(t, err)
			runtime.KeepAlive(file)
		})

		t.Logf("%-19s %7dK %8dK %9dK %8dK %10dK %6.2fx %8.0f%%",
			w.Name, len(w.Data)/1024, len(out)/1024,
			bare/1024, full/1024, progressive/1024,
			float64(full)/float64(progressive),
			100*float64(bare)/float64(progressive))
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

func BenchmarkToJSONProgressive(b *testing.B) {
	forEachWorkload(b, func(b *testing.B, src []byte) {
		for b.Loop() {
			if _, err := lab.ToJSONProgressive(src); err != nil {
				b.Fatal(err)
			}
		}
	})
}
