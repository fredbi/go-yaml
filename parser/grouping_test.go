// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/tokenarena"
)

// TestGroupingHolds logs how many tokens the grouping holds ahead of the descent, for each corpus document.
//
// A grouping pass runs ahead of the parse and holds what it cannot settle yet,
// so the tape keeps at least that much however far the tail has moved.
// groupMapKeysByValue holds the most: keyWindow.keepFrom reaches back to the start of any open flow collection,
// because that collection may still close and become a key.
//
// It asserts only that each document parses. Run with -v for the table.
func TestGroupingHolds(t *testing.T) {
	ordinary := readCorpus(t, corpusDir())
	stress := readCorpus(t, stressDir())

	t.Logf("%-19s %8s %7s %10s %12s", "document", "tokens", "chunk", "held", "chunks held")

	for _, set := range [][]corpusDoc{ordinary, stress} {
		for _, w := range set {
			chunk := tokenarena.SizeFor(len(w.Data))

			p := New(WithChunkSize(chunk))
			_, err := p.Parse(w.Data)
			require.NoError(t, err, w.Name)

			held := p.groupingHeld()
			t.Logf("%-19s %8d %7d %10d %12d",
				w.Name, p.tapeStats().Tokens, chunk, held, held/chunk+1)
		}
	}
}

// TestGroupingHoldsLittleInBlockStyle checks that the grouping holds at most 16 tokens
// on every corpus document written in block style, however wide.
//
// The pass settles each key as its ':' arrives and hands the rest on, so the width of a mapping costs nothing.
func TestGroupingHoldsLittleInBlockStyle(t *testing.T) {
	all := readCorpus(t, corpusDir())
	wide := readCorpus(t, stressDir())

	for _, set := range [][]corpusDoc{all, wide} {
		for _, w := range set {
			if w.Name == "flow_wide" || w.Name == "flow_long_scalars" || w.Name == "flow_nested" {
				continue
			}

			p := New()
			_, err := p.Parse(w.Data)
			require.NoError(t, err, w.Name)

			require.LessOrEqual(t, p.groupingHeld(), 16,
				"%s: the grouping held %d tokens, so a flow collection is open somewhere it was not before",
				w.Name, p.groupingHeld())
		}
	}
}

// TestAFlowCollectionHoldsToItsClose checks that the grouping holds more than 50,000 tokens of flow_wide at once.
//
// A flow collection may close and become a mapping key, so nothing inside one is settled until its ']' arrives,
// and the grouping holds the whole of flow_wide's single sequence.
// The sequence is written as a value ("wide: [...]"), so it cannot be a key,
// but the key window does not use that to release its tokens.
func TestAFlowCollectionHoldsToItsClose(t *testing.T) {
	for _, w := range readCorpus(t, stressDir()) {
		if w.Name != "flow_wide" {
			continue
		}

		p := New()
		_, err := p.Parse(w.Data)
		require.NoError(t, err)

		require.Greater(t, p.groupingHeld(), 50_000,
			"the window released inside a flow collection, which would be new")
		t.Logf("flow_wide: %d tokens, the grouping holds %d of them at once",
			p.tapeStats().Tokens, p.groupingHeld())
	}
}
