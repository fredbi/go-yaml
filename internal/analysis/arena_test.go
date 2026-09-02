// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
	"github.com/go-openapi/go-yaml/internal/refparser"
	"github.com/go-openapi/go-yaml/internal/scanner"
)

// TestArenaStats reports what the tree of each workload cost, exactly.
//
// Read off counters the arena keeps rather than off a heap profile. The profile
// gave the same figures to within a few percent and could not name them: the
// nodes of one type come from a generic block, so a stack calls
// MappingValueNode "block[struct { BaseNode; Start *token.Token; ... }].next".
//
// Unused is the column a profile cannot give at all -- the tail of each type's
// last block, allocated and never handed out. Run with -v for the table.
func TestArenaStats(t *testing.T) {
	all, err := workloads.All()
	require.NoError(t, err)

	for _, w := range all {
		var s scanner.Scanner
		s.Init(string(w.Data))

		p, err := refparser.New(s.Tokens(), 0)
		require.NoError(t, err)

		_, err = p.Parse()
		require.NoError(t, err)

		stats := p.ArenaStats()
		require.NotZero(t, stats.Total.Nodes)

		t.Logf("%s -- %dK of source, blocks of %d nodes", w.Name, len(w.Data)/1024, stats.BlockSize)
		t.Logf("    %-18s %9s %8s %10s %9s %7s", "type", "nodes", "blocks", "bytes", "unused", "B/node")

		for _, row := range append(stats.ByType, stats.Total) {
			t.Logf("    %-18s %9d %8d %9dK %8dK %6.0f",
				row.Type, row.Nodes, row.Blocks, row.Bytes/1024, row.Unused/1024,
				float64(row.Bytes)/float64(max(row.Nodes, 1)))
		}
	}
}

// TestArenaStatsAccountForEveryNode ties the counters to the tree they describe.
//
// Every node the arena hands out is in the tree and every node in the tree bar
// the documents came from the arena, so the two counts differ by exactly what
// ast.Walk cannot reach. Today that is the sequence entries: Walk descends a
// SequenceNode's Values and never its Entries, so a walk misses one node per
// sequence entry in the document.
//
// The equation is asserted rather than the inequality, so that fixing Walk to
// descend Entries -- or adding a node type without a row in ast.Arena.Stats --
// fails here instead of quietly skewing every table built on these figures.
func TestArenaStatsAccountForEveryNode(t *testing.T) {
	w, err := workloads.ByName("azure_swagger")
	require.NoError(t, err)

	var s scanner.Scanner
	s.Init(string(w.Data))

	p, err := refparser.New(s.Tokens(), 0)
	require.NoError(t, err)

	file, err := p.Parse()
	require.NoError(t, err)

	stats := p.ArenaStats()

	var fromArena, entries int
	for _, row := range stats.ByType {
		switch row.Type {
		case "mapping runs": // lists, not nodes
		case "SequenceEntryNode":
			entries = row.Nodes
			fromArena += row.Nodes
		default:
			fromArena += row.Nodes
		}
	}

	walked, _, _ := countTreeShape(file)

	require.Positive(t, fromArena)
	require.Positive(t, entries)
	require.Equal(t, walked, fromArena-entries+len(file.Docs),
		"arena handed out %d nodes (%d of them sequence entries), a walk reaches %d over %d documents",
		fromArena, entries, walked, len(file.Docs))
}
