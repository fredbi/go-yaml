// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
	"github.com/go-openapi/go-yaml/internal/lab"
	"github.com/go-openapi/go-yaml/internal/tokenarena"
)

// TestToJSONTailTrace reports what the tape would hold for a JSON conversion.
//
// A converter keeps no parent node: it announces a container, writes each child
// as it comes and closes after the last of them. So every token below the node
// just finished is finished with, and the tail follows the parse. That makes it
// the most favorable of the common uses, against the full scan -- which is the
// same parser with the pin held -- as the least.
//
// The parse still holds everything, so this records where the tail could have
// gone rather than moving it, and replays that trace through an arena of its
// own -- with the tokens arriving as the descent reads them, which the parser
// cannot do yet because New reads the scanner to its end first.
//
// So this is where the recycling shows: chunks are allocated only until the
// free list has enough to go round, and every one after that is taken back off
// it. Run with -v for the table.
func TestToJSONTailTrace(t *testing.T) {
	ordinary, err := workloads.All()
	require.NoError(t, err)

	stress, err := workloads.Stress()
	require.NoError(t, err)

	for _, group := range []struct {
		name string
		set  []workloads.Workload
	}{
		{"corpus", ordinary},
		{"stress", stress},
	} {
		t.Logf("--- %s ---", group.name)
		t.Logf("%-19s %8s %9s %10s %9s %6s %9s %10s",
			"document", "tokens", "gathered", "allocated", "recycled", "hit", "free high", "handed over")

		for _, w := range group.set {
			chunk := tokenarena.SizeFor(len(w.Data))

			trace, err := lab.TraceToJSONTail(w.Data, chunk)
			require.NoError(t, err, w.Name)

			handed := trace.Handed
			hit := 100 * float64(handed.Recycled) / float64(max(handed.Recycled+handed.Allocated, 1))

			t.Logf("%-19s %8d %8dK %10d %9d %5.0f%% %9d %9dK",
				w.Name, trace.Tokens, trace.Gathered.Bytes/1024,
				handed.Allocated, handed.Recycled, hit, handed.FreeHigh,
				handed.Bytes/1024)
		}
	}
}
