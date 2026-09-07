// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"fmt"
	"runtime"
	"runtime/metrics"
	"sort"
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
	"github.com/go-openapi/go-yaml/parser"
)

// parsers are the builds a measurement is taken over.
//
// It held two while a candidate was being measured against the frozen copy it
// would replace. The candidate shipped, so there is one build left -- but the
// loops below stay a range over this slice, because a candidate's memory
// activity means little on its own and everything against the parser it would
// replace. Add the second entry and both tables report the pair again.
var parsers = []struct {
	name  string
	parse func([]byte) (*ast.File, error)
}{
	{"parser", func(src []byte) (*ast.File, error) { return parser.ParseBytes(src) }},
}

// TestGCActivity reports what a parse costs the collector.
//
// Allocated bytes say how much traffic a parse makes; they do not say what the
// collector then does about it. These are the numbers that do: how many cycles
// a parse triggers, how much CPU went to collecting rather than parsing, and
// how much of that was assist -- work the allocating goroutine was made to do
// because it was outrunning the collector.
//
// Read against TestParseChurn: churn is the traffic, this is the noise it makes.
// Run with -v for the table.
func TestGCActivity(t *testing.T) {
	all, err := workloads.All()
	require.NoError(t, err)

	t.Logf("%-19s %-7s %10s %9s %7s %9s %9s %9s",
		"workload", "built by", "allocated", "objects", "cycles", "gc cpu", "assist", "live peak")

	for _, w := range all {
		for _, build := range parsers {
			before := readGC()

			const runs = 5
			for range runs {
				file, err := build.parse(w.Data)
				require.NoError(t, err)
				runtime.KeepAlive(file)
			}

			after := readGC()
			peak := peakLiveParse(t, w.Data, build.parse)

			bytes := (after.allocBytes - before.allocBytes) / runs
			objects := (after.allocObjects - before.allocObjects) / runs

			t.Logf("%-19s %-7s %9dK %9d %7d %8.1fms %8.1fms %8dK",
				w.Name, build.name, bytes/1024, objects,
				after.cycles-before.cycles,
				1000*(after.gcCPU-before.gcCPU),
				1000*(after.assistCPU-before.assistCPU), peak/1024)
		}
	}
}

// gcSample is the collector's counters at one moment.
type gcSample struct {
	allocBytes   uint64
	allocObjects uint64
	cycles       uint64
	gcCPU        float64
	assistCPU    float64
}

func readGC() gcSample {
	names := []string{
		"/gc/heap/allocs:bytes",
		"/gc/heap/allocs:objects",
		"/gc/cycles/total:gc-cycles",
		"/cpu/classes/gc/total:cpu-seconds",
		"/cpu/classes/gc/mark/assist:cpu-seconds",
	}

	samples := make([]metrics.Sample, len(names))
	for i, name := range names {
		samples[i].Name = name
	}
	metrics.Read(samples)

	return gcSample{
		allocBytes:   samples[0].Value.Uint64(),
		allocObjects: samples[1].Value.Uint64(),
		cycles:       samples[2].Value.Uint64(),
		gcCPU:        samples[3].Value.Float64(),
		assistCPU:    samples[4].Value.Float64(),
	}
}

// peakLiveParse is the most the heap held at once while parsing data.
func peakLiveParse(t *testing.T, data []byte, parse func([]byte) (*ast.File, error)) uint64 {
	t.Helper()

	return peakLiveMax(peakLiveRuns(), func() {
		file, err := parse(data)
		require.NoError(t, err)
		runtime.KeepAlive(file)
	})
}

// TestSliceGrowth attributes the copying that append does when a slice outgrows
// its capacity.
//
// A slice that grows copies what it already held, and neither the copy nor the
// old array shows up as anything but bytes allocated: it is traffic with no
// name on it. runtime.growslice is where it happens, so the stacks that reach
// it are the sites where a slice was sized too small.
//
// Exact, and slow: MemProfileRate is 1 for the parse. Run with -v for the table.
func TestSliceGrowth(t *testing.T) {
	all, err := workloads.All()
	require.NoError(t, err)

	t.Logf("%-19s %-7s %11s %7s   %s", "workload", "built by", "grown", "sites", "the two largest")

	for _, w := range all {
		for _, build := range parsers {
			total, sites := measureGrowth(t, w.Data, build.parse)

			var top []string
			for i, site := range sites {
				if i == 2 {
					break
				}
				top = append(top, fmt.Sprintf("%s %dK", site.site, site.bytes/1024))
			}

			t.Logf("%-19s %-7s %10dK %7d   %s",
				w.Name, build.name, total/1024, len(sites), strings.Join(top, ", "))
		}
	}
}

type growthSite struct {
	site  string
	bytes uint64
}

// measureGrowth parses data and returns what runtime.growslice allocated, in
// total and by the site that grew the slice.
func measureGrowth(t *testing.T, data []byte, parse func([]byte) (*ast.File, error)) (uint64, []growthSite) {
	t.Helper()

	old := runtime.MemProfileRate
	runtime.MemProfileRate = 1
	defer func() { runtime.MemProfileRate = old }()

	runtime.GC()
	before := growthByStack()

	file, err := parse(data)
	require.NoError(t, err)
	runtime.GC()
	runtime.KeepAlive(file)
	holdAny = file

	after := growthByStack()

	// Let the tree go before returning. Held on in holdAny it stays live for
	// the rest of the package's run -- golang_source's is 46 MB -- and the
	// timing tests below measure a parse under a heap that is not theirs.
	holdAny = nil
	runtime.GC()

	var (
		total uint64
		sites []growthSite
	)
	for site, bytes := range after {
		grown := bytes - before[site]
		if bytes < before[site] || grown == 0 {
			continue
		}
		total += grown
		sites = append(sites, growthSite{site: site, bytes: grown})
	}
	sort.Slice(sites, func(i, j int) bool { return sites[i].bytes > sites[j].bytes })

	return total, sites
}

// growthByStack reads the heap profile and returns, per site, what
// runtime.growslice allocated on its behalf.
func growthByStack() map[string]uint64 {
	var records []runtime.MemProfileRecord
	for {
		n, ok := runtime.MemProfile(records, true)
		if ok {
			records = records[:n]

			break
		}
		records = make([]runtime.MemProfileRecord, n+64)
	}

	out := map[string]uint64{}
	for i := range records {
		record := &records[i]
		if site, grew := growthSiteOf(record); grew {
			out[site] += uint64(record.AllocBytes)
		}
	}

	return out
}

// growthSiteOf reports whether a record's stack passes through
// runtime.growslice, and names the first frame of ours below it.
func growthSiteOf(record *runtime.MemProfileRecord) (string, bool) {
	var (
		grew bool
		site string
	)

	frames := runtime.CallersFrames(record.Stack())
	for {
		frame, more := frames.Next()
		switch {
		case strings.HasSuffix(frame.Function, "runtime.growslice"):
			grew = true
		case grew && site == "" && strings.Contains(frame.Function, "go-openapi/go-yaml"):
			site = fmt.Sprintf("%s:%d", shortName(frame.Function), frame.Line)
		}
		if !more {
			break
		}
	}

	return site, grew && site != ""
}

// shortName drops the module path from a function name.
func shortName(fn string) string {
	if i := strings.LastIndex(fn, "go-yaml/"); i >= 0 {
		return fn[i+len("go-yaml/"):]
	}

	return fn
}
