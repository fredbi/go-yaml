// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:build yamlprobe

package scanner_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/fuzzseeds"
	"github.com/go-openapi/go-yaml/internal/ledgers"
	"github.com/go-openapi/go-yaml/internal/probe"
	"github.com/go-openapi/go-yaml/internal/scanner"
)

// stateLedger records how often two pieces of the scanner's state that look like the same number disagree, over the
// fuzz corpus.
//
// It decides whether a field can go.
// Four went: Scanner.source, sourceSize, sourcePos and offset each agreed with the Context's src, size and idx over
// 395,323 checks without one disagreement, so they held the Context's numbers under other names.
// The three below disagree, and measuring them turns that from a guess into a count.
//
// A ratchet in both directions, as the offset ledger is.
// A pair that starts disagreeing more has lost an invariant; one that starts disagreeing less may have become
// removable, and the entry comes down to say so.
var stateLedger = map[string]int64{ //nolint:gochecknoglobals // ok to store and immutable map as a global
	// Outside a block scalar the mark is the buffer's length less the whitespace and the fold break it ends with,
	// exactly, over every read.
	// It was three until bufferedToken stopped clearing the value buffer and leaving the mark past the end of it.
	//
	// So it could be worked out at the read, a scan back over the buffer once per token, instead of a compare and a
	// store for every character written.
	// Inside a block scalar it could not: the two sites that rewrite the buffer set it outright, keeping the space that
	// folds a line and dropping the tab that ends one, and no scan of the bytes tells those apart from content.
	// 3 of 1,348 measured 2026-09-07.
	"buf.notSpaceCharPos==trimmed/plain": 0,
	"buf.notSpaceCharPos==trimmed/block": 3,

	// A mark past the end of the buffer made bufferedSrc slice a byte the last token wrote.
	// Fixed; nothing may raise this.
	"buf.notSpaceCharPos<=len(buf)": 0,

	// Both entries count a space opening a line where indentNum has stopped tracking the column. They have different
	// causes, and only the second is a surprise.
	//
	// The /tab bucket is 100% by construction, and reading it as a count of anything else is a mistake made once
	// already. updateIndent takes a tab in leading whitespace, sets indentHasTab and returns without counting it,
	// because s-indent(n) is s-space x n and a tab is separation and not indentation. The main loop advances the column
	// for it regardless, so from that tab to the end of the line indentNum lags column-1 and every following space
	// trips the probe. 12 of 12 re-baselined 2026-09-08, from 5 of 5: yamlgen began drawing tab separators, so the
	// corpus holds more spaces standing after a tab. The number counts those and nothing else, and it always reads
	// 100%. It would catch a tab starting to count as indentation, which breaks s-indent(n).
	//
	// The /spaces bucket is the one worth watching, and it holds one cause: a quoted scalar spanning a line break.
	// The quote scanners call progressLine, marking the next character as opening a line, then read the rest of the
	// scalar with progressColumn, which never reaches updateIndent. So isFirstCharAtLine is still true after the line
	// has been read into. 8 of 19,953, 0.04%, re-baselined 2026-09-08 from 7 of 11,748, 0.06%: the corpus grew faster
	// than the cause did.
	"indent.indentNum==column-1/tab":    12,
	"indent.indentNum==column-1/spaces": 8,

	// The indent level a token was given and the level the scanner stands at part company where a block opens, so the
	// two are not a redundant pair. 4,213 of 129,685 re-baselined 2026-09-08, from 2,729 of 83,807, and 2,793 of
	// 69,177 and 3,000 of 76,279 before that.
	//
	// Read the ratio, not the count. 3.26% -> 3.25% across a corpus that grew by 55%, so the scanner stands where it
	// did; the count follows the corpus. It was 4.0% while the corpus was 69,177 pos() calls, and fell as yamlgen's
	// shallow shapes arrived. A count re-baselined without the ratio beside it says nothing about whether the scanner
	// changed.
	//
	// 2 of the 4,213 are scanDirective refusing a tab-indented '%', which ends those documents earlier than they
	// ended before. The other 1,482 are corpus growth.
	"indent.lastIndentLevel==indentLevel": 4213,

	// bufferedToken assembles a token's extent from what the scanner already holds: where the origin began, how long
	// it is, and the line the text ends on. It does not read the origin back to work the extent out.
	// This compares that extent against token.MeasureOrigin's, which token.Make used.
	// Nothing may raise it: a disagreement is a token pointing at the wrong stretch of source.
	"token.extentMatchesTheOrigin": 0,
}

// TestStateLedger holds the scanner's state pairs to what they were measured at.
//
//	go test -tags yamlprobe -run TestStateLedger ./internal/scanner/
func TestStateLedger(t *testing.T) {
	seeds, err := fuzzseeds.All()
	require.NoError(t, err)
	require.NotEmpty(t, seeds)

	probe.Reset()
	for _, src := range seeds {
		var s scanner.Scanner
		s.Init([]byte(src))
		for {
			if _, ok := s.NextToken(); !ok {
				break
			}
		}
	}

	// probe.Checks holds every invariant the run touched, the ones that never disagreed included, and most of them
	// never disagree. An invariant is measured when it disagreed, so a clean one the ledger says nothing about is not
	// reported as a new defect.
	//
	// A clean invariant the ledger does name is measured too, at 0. Those entries are the ones recording that a pair
	// must never disagree, and dropping them would read as a defect that had gone away.
	checks := probe.Checks()
	measured := make(map[string]int64, len(checks))
	for name, inv := range checks {
		if _, recorded := stateLedger[name]; inv.Failed > 0 || recorded {
			measured[name] = inv.Failed
		}
	}

	// The counts move with the corpus, so a failure is only readable next to what each was measured over. Compare
	// reports the count; these lines carry the denominator and the ratio.
	for _, name := range slices.Sorted(maps.Keys(measured)) {
		inv := checks[name]
		var ratio float64
		if inv.Tested > 0 {
			ratio = 100 * float64(inv.Failed) / float64(inv.Tested)
		}
		t.Logf("%s: %d of %d (%.2f%%)", name, inv.Failed, inv.Tested, ratio)
	}

	ledgers.Compare(t, "disagreements", measured, stateLedger)

	for name := range stateLedger {
		require.Containsf(t, checks, name, "%s is in the ledger and nothing checks it", name)
	}
}
