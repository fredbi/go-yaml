// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
	"github.com/go-openapi/go-yaml/internal/lab/labparser"
	"github.com/go-openapi/go-yaml/internal/lab/tokenarena"
)

// anchored names the documents holding an anchor, whose chunks a walk saves.
var anchored = map[string]bool{
	"anchors_far": true, "anchors_many": true, "anchors_nested": true,
}

// counting is a visitor that keeps nothing, which is the point -- except for
// the first anchor it meets, which it keeps on purpose so that what Save
// promises can be checked after the walk.
type counting struct {
	nodes      int
	anchor     ast.Node
	anchorName string
}

func (v *counting) Enter(node ast.Node, _ labparser.Step) bool {
	v.nodes++
	if anchor, ok := node.(*ast.AnchorNode); ok && v.anchor == nil {
		v.anchor, v.anchorName = anchor, anchor.GetToken().Value
	}

	return true
}

func (v *counting) Leave(ast.Node, labparser.Step) {}

// TestWalkLetsTheTapeGo checks a walk hands the tape back as it reads.
//
// The walk keeps nothing it is handed, so the tail follows the descent and
// every chunk behind it reaches the free list. Two chunks stand at the end,
// whatever the document: the one being read and the one before it.
//
// ⚠️ It frees; it does not recycle, and the table is written so that cannot be
// misread. A chunk counts as recycled when a later Add takes it back off the
// free list, and every Add happens in New, under the pin, before a walk starts.
// So the free list only grows: the hit ratio is 0% and the free-list high-water
// mark climbs to nearly every chunk the document needed.
//
// Read "free high" as idle rather than as a cost, and read it against "saved
// high", which it complements: anchors_far frees 249 and saves 1, anchors_many
// frees 0 and saves 218. Neither says anything about recycling, and both add up
// to the chunks the document needed. What an anchor costs is the saved column
// alone.
//
// Recycling that worked would read the other way round -- a free list that
// stays short and a hit ratio near 100%, with chunks allocated only where a
// node spans more of them than the tape holds.
// TestTheTailReleasesWhatIsBehindIt shows that shape on the arena alone: 800
// tokens through a lag of two chunks takes 4 chunks, recycles 96, and never has
// more than one chunk waiting.
//
// The parser cannot show it until New reads as the parse does. working set is
// what the walk needed at once, and is what the tape would hold then.
func TestWalkLetsTheTapeGo(t *testing.T) {
	ordinary, err := workloads.All()
	require.NoError(t, err)

	stress, err := workloads.Stress()
	require.NoError(t, err)

	t.Logf("%-19s %8s %10s %9s %6s %9s %8s %7s %10s",
		"document", "tokens", "allocated", "recycled", "hit", "free high", "live", "saved high", "working set")

	for _, set := range [][]workloads.Workload{ordinary, stress} {
		for _, w := range set {
			p := labparser.New(labparser.ChunkSize(tokenarena.SizeFor(len(w.Data))))

			keep := &counting{}
			_, err := p.Walk(w.Data, keep)
			require.NoError(t, err, w.Name)

			stats := p.TokenStats()

			if anchored[w.Name] {
				// Saved is zero by now: the document ended and what its anchors
				// saved went back with it. SavedHigh is what they held while it
				// ran.
				require.Positive(t, stats.SavedHigh,
					"%s holds anchors and the walk saved no chunk for them", w.Name)
				require.Zero(t, stats.Saved,
					"%s: the document ended and its saves did not go back", w.Name)

				// What Save promises. It holds trivially while nothing refills
				// the free list, and becomes a real check the moment tokens
				// arrive during a walk.
				require.NotNil(t, keep.anchor)
				require.Equal(t, keep.anchorName, keep.anchor.GetToken().Value,
					"%s: the anchor's token was filled again under it", w.Name)
			}

			require.False(t, stats.Frozen, "the walk kept its pin")
			require.Equal(t, stats.Allocated, stats.Live+stats.Free+stats.Saved,
				"%s: chunks went missing", w.Name)

			require.LessOrEqual(t, stats.Live, 3,
				"%s: a walk left %d chunks live, so something is holding what it was handed",
				w.Name, stats.Live)

			// held is the memory the arena has taken and not given back --
			// every chunk it allocated, free list included. working set is
			// what the walk actually needed at once, which is what the same
			// walk would hold if the tokens arrived as it read rather than
			// all before it.
			hit := 100 * float64(stats.Recycled) / float64(max(stats.Recycled+stats.Allocated, 1))

			t.Logf("%-19s %8d %10d %9d %5.0f%% %9d %8d %7d %9dK",
				w.Name, stats.Tokens, stats.Allocated, stats.Recycled, hit,
				stats.FreeHigh, stats.Live, stats.SavedHigh,
				(stats.Live+stats.SavedHigh)*stats.ChunkSize*56/1024)
		}
	}
}
