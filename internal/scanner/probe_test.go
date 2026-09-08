// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:build yamlprobe

package scanner_test

import (
	"sort"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/fuzzseeds"
	"github.com/go-openapi/go-yaml/internal/probe"
	"github.com/go-openapi/go-yaml/internal/scanner"
	"github.com/go-openapi/go-yaml/internal/scanner/internal/testscanner"
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
	// trips the probe. 5 of 5 re-baselined 2026-09-07 evening, from 6 of 6: the corpus lost one space standing after
	// a tab. The number counts spaces after a tab in the corpus, nothing more, and it always reads 100%. It
	// would catch a tab starting to count as indentation, which breaks s-indent(n).
	//
	// The /spaces bucket is the one worth watching, and it holds one cause: a quoted scalar spanning a line break.
	// The quote scanners call progressLine, marking the next character as opening a line, then read the rest of the
	// scalar with progressColumn, which never reaches updateIndent. So isFirstCharAtLine is still true after the line
	// has been read into. 7 of 11,748 re-baselined 2026-09-07 evening, unmoved from 7 of 11,729.
	"indent.indentNum==column-1/tab":    5,
	"indent.indentNum==column-1/spaces": 7,

	// The indent level a token was given and the level the scanner stands at part company where a block opens, so the
	// two are not a redundant pair. 2,729 of 83,807 re-baselined 2026-09-07 evening, from 2,793 of 69,177 that
	// morning and 3,000 of 76,279 before it.
	//
	// ⚠️ Read the ratio, not the count: it has moved, where the earlier two entries said it held. 4.0% -> 3.3% as the
	// corpus went from 69,177 pos() calls to 83,807, which is the generated documents arriving -- yamlgen draws
	// shallow shapes, and the two levels part company where a block opens. A count re-baselined without the ratio
	// beside it says nothing about whether the scanner changed.
	"indent.lastIndentLevel==indentLevel": 2729,

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

	checks := probe.Checks()
	names := make([]string, 0, len(checks))
	for name := range checks {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		inv := checks[name]
		want, known := stateLedger[name]
		if !known {
			assert.Zerof(t, inv.Failed,
				"%s is not in the ledger and disagreed %d times of %d: %v",
				name, inv.Failed, inv.Tested, inv.Samples)

			continue
		}
		assert.Equalf(t, want, inv.Failed,
			"%s disagreed %d times of %d, and the ledger records %d: %v",
			name, inv.Failed, inv.Tested, want, inv.Samples)
	}

	for name := range stateLedger {
		assert.Containsf(t, checks, name, "%s is in the ledger and nothing checks it", name)
	}
}

// TestBufferHoldsTwoTokens holds what the scanner keeps for the caller while it reads a document through NextToken,
// which is the call the parser makes.
//
// Context.pending is a hand-over buffer, not a store.
//
// One step of scan can produce more than one token: scanMapDelim cuts the key it had been reading and then emits the
// ':', and a block scalar header emits the header and the comment on its line. The caller takes them one at a time,
// so the extras wait somewhere. rewind empties pending between steps without giving the room back, so the buffer
// settles at what one step ever produced.
//
// Two, over every document in the workloads.
// The parser's arena holds the document; this holds the overflow of one step of the scan.
//
//	go test -tags yamlprobe -run TestBufferHoldsTwoTokens ./internal/scanner/
func TestBufferHoldsTwoTokens(t *testing.T) {
	for _, doc := range testscanner.WorkloadDocs(t) {
		probe.Reset()

		var s scanner.Scanner
		s.Init(doc.Bytes())
		for {
			if _, ok := s.NextToken(); !ok {
				break
			}
		}

		counts := probe.Counts()
		assert.LessOrEqualf(t, counts["buffer.heldAtOnce"], int64(2),
			"the scanner held %d tokens at once, where one step of scan makes at most two",
			counts["buffer.heldAtOnce"])
		assert.LessOrEqualf(t, counts["buffer.roomTaken"], int64(2),
			"the buffer took room for %d tokens, where two is all one step of scan fills",
			counts["buffer.roomTaken"])
	}
}
