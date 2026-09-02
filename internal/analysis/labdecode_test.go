// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"testing"
	"time"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
	"github.com/go-openapi/go-yaml/internal/lab"
	"github.com/go-openapi/go-yaml/internal/lab/labparser"
)

// peakLive reports the high-water mark of the live heap while work runs.
//
// HeapAlloc counts garbage the collector has not reached yet, so sampling it
// measures allocation rate rather than liveness. /gc/heap/live:bytes is the
// live heap as of the last collection, and turning the collector up to run
// almost continuously makes that a fair reading of the peak. Slow on purpose.
// peakLiveMax is the largest peak seen over several runs of the same work.
//
// The largest and not the average, because peakLive can only miss the peak, not
// invent one: /gc/heap/live:bytes moves when a cycle ends, so a run reports the
// largest live figure a collection happened to catch. One run reports where the
// collections fell. Taking the largest of several converges on where the peak
// actually was.
//
// Read one run and it lies by tens of percent in either direction. A 24% saving
// from sizing the node arena small was read off single runs and turned out,
// once it was repeated, to be nothing at all.
func peakLiveMax(runs int, work func()) uint64 {
	var most uint64
	for range runs {
		most = max(most, peakLive(work))
	}

	return most
}

// peakLiveRuns is how many runs a reported peak is taken over. Five holds the
// smaller workloads to a couple of percent between runs and keeps the tables
// that use it inside half a minute; -short takes one and reports a figure worth
// nothing, which is what -short is for.
func peakLiveRuns() int {
	if testing.Short() {
		return 1
	}

	return 5
}

func peakLive(work func()) uint64 {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	runtime.GC()

	sample := []metrics.Sample{{Name: "/gc/heap/live:bytes"}}
	read := func() uint64 {
		metrics.Read(sample)

		return sample[0].Value.Uint64()
	}

	base := read()
	var peak uint64

	done := make(chan struct{})
	go func() {
		defer close(done)
		work()
	}()

	for {
		select {
		case <-done:
			if v := read(); v > peak {
				peak = v
			}
			if peak < base {
				return 0
			}

			return peak - base
		default:
			if v := read(); v > peak {
				peak = v
			}
			time.Sleep(50 * time.Microsecond)
		}
	}
}

// TestProgressiveDecodePeak measures what the lab decoder retains against what
// Unmarshal does, on the workloads the prize was sized on.
//
// TestDecodePrize says the tree is 73% to 87% of an Unmarshal's peak. This says
// whether a decoder that forgets each node as it converts it actually gets that
// back. Run with -v.
func TestProgressiveDecodePeak(t *testing.T) {
	all, err := workloads.All()
	require.NoError(t, err)

	t.Logf("%-17s %9s %11s %11s %8s", "workload", "source", "Unmarshal", "progressive", "ratio")

	for _, w := range all {
		var want any
		require.NoError(t, yaml.Unmarshal(w.Data, &want))

		got, err := lab.DecodeProgressive(w.Data)
		require.NoError(t, err)
		require.Equal(t, want, got, "%s: the progressive decoder read a different value", w.Name)

		classic := peakLive(func() {
			var v any
			require.NoError(t, yaml.Unmarshal(w.Data, &v))
			runtime.KeepAlive(v)
		})
		progressive := peakLive(func() {
			v, err := lab.DecodeProgressive(w.Data)
			require.NoError(t, err)
			runtime.KeepAlive(v)
		})

		// The floor: what a parse alone peaks at, decoding nothing. No consumer
		// can do better than this while the parser holds what it holds.
		parseOnly := peakLive(func() {
			f, err := labparser.ParseBytes(w.Data, 0)
			require.NoError(t, err)
			runtime.KeepAlive(f)
		})

		t.Logf("%-17s %8dK %10dK %10dK %7.1fx  parse-only floor %dK",
			w.Name, len(w.Data)/1024, classic/1024, progressive/1024,
			float64(classic)/float64(progressive), parseOnly/1024)
	}
}
