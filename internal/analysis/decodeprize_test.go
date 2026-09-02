// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"runtime"
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
	"github.com/go-openapi/go-yaml/parser"
)

// TestDecodePrize sizes what a decoder built on a progressively revealed AST
// would stop retaining.
//
// Unmarshal parses the whole document into an ast.File, walks it into Go
// values, and drops the tree at the end -- so the tree and the value are both
// live while the walk runs. A decoder that took nodes one at a time and forgot
// each once it had converted it would retain the value alone.
//
// The prize is therefore the tree, and this measures it against the value it is
// converted into. Run with -v.
func TestDecodePrize(t *testing.T) {
	all, err := workloads.All()
	require.NoError(t, err)

	t.Logf("%-17s %9s %9s %9s %9s %8s", "workload", "source", "tree", "value", "peak", "tree/peak")

	for _, w := range all {
		tree := retainedBytes(t, func() any {
			f, err := parser.ParseBytes(w.Data, 0)
			require.NoError(t, err)

			return f
		})

		value := retainedBytes(t, func() any {
			var v any
			require.NoError(t, yaml.Unmarshal(w.Data, &v))

			return &v
		})

		peak := tree + value
		t.Logf("%-17s %8dK %8dK %8dK %8dK %7.0f%%",
			w.Name, len(w.Data)/1024, tree/1024, value/1024, peak/1024,
			100*float64(tree)/float64(peak))
	}
}

// retained reports the live heap with what build returns still reachable.
func retainedBytes(t *testing.T, build func() any) uint64 {
	t.Helper()

	runtime.GC()
	runtime.GC()

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	v := build()

	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(v)
	holdAny = v

	if after.HeapAlloc < before.HeapAlloc {
		return 0
	}

	return after.HeapAlloc - before.HeapAlloc
}
