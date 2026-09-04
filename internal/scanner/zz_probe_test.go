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
)

// stateLedger records how often two pieces of the scanner's state that look
// like the same number disagree, over the fuzz corpus.
//
// It is what says whether a field can go. Four did: Scanner.source, sourceSize,
// sourcePos and offset each agreed with the Context's src, size and idx over
// 395,323 checks without one disagreement, so they were the Context's by
// another name and were removed. The three below are not, and this is where
// that stops being a guess.
//
// A ratchet in both directions, as the offset ledger is. A pair that starts
// disagreeing more has lost an invariant; one that starts disagreeing less may
// have become removable, and the entry comes down to say so.
var stateLedger = map[string]int64{
	// Outside a block scalar the mark is the buffer's length less the
	// whitespace and the fold break it ends with, exactly: 135,723 reads and no
	// disagreement. It was three until bufferedToken stopped clearing the value
	// buffer and leaving the mark past the end of it.
	//
	// So it could be worked out at the read -- a scan back over the buffer,
	// once per token -- instead of a compare and a store for every character
	// written. Inside a block scalar it could not: the two sites that rewrite
	// the buffer set it outright, keeping the space that folds a line and
	// dropping the tab that ends one, and no scan of the bytes tells those
	// apart from content.
	"buf.notSpaceCharPos==trimmed/plain": 0,
	"buf.notSpaceCharPos==trimmed/block": 4,

	// A mark past the end of the buffer made bufferedSrc slice a byte the last
	// token wrote. Fixed; nothing may raise this.
	"buf.notSpaceCharPos<=len(buf)": 0,

	// One cause, in two shapes: isFirstCharAtLine is still true after characters
	// have been read on the line by a path that does not reach updateIndent's
	// space branch, so the column has moved and the indentation has not.
	//
	// The three with indentHasTab are a tab inside a block scalar's content,
	// past the indentation the header set -- which is content and not
	// indentation, YAML having none of the latter but spaces (s-indent(n) is
	// s-space x n, and a tab there is refused). updateIndent runs for every
	// character the main loop reads, block scalar content included, so it sets
	// indentHasTab for a tab that indents nothing. Harmless: progressLine
	// clears it, and nothing between reads it inside a block.
	//
	// The nine without are a quoted scalar spanning a line break. The quote
	// scanners call progressLine, which says the next character opens a line,
	// then read the rest of the scalar with progressColumn, which never reaches
	// updateIndent.
	"indent.indentNum==column-1/tab":    3,
	"indent.indentNum==column-1/spaces": 9,

	// The indent level a token was given and the level the scanner stands at
	// part company where a block opens: 3,000 of 76,279. Not a pair.
	"indent.lastIndentLevel==indentLevel": 3000,
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
			"%s disagreed %d times of %d, and the ledger says %d: %v",
			name, inv.Failed, inv.Tested, want, inv.Samples)
	}

	for name := range stateLedger {
		assert.Containsf(t, checks, name, "%s is in the ledger and nothing checks it", name)
	}
}

// TestBufferHoldsTwoTokens holds what the scanner keeps for the caller while it
// reads a document through NextToken, which is the call the parser makes.
//
// Context.blocks is a hand-over buffer, not a store. One step of scan can
// produce more than one token -- scanMapDelim cuts the key it had been reading
// and then emits the ':', a block scalar header emits the header and the
// comment on its line -- and the caller takes them one at a time, so the extras
// wait somewhere. rewind empties the blocks between steps without giving the
// room back, so the buffer settles at what one step ever produced.
//
// Two, over every document in the workloads. The parser's arena holds the
// document; this holds the overflow of one step of the scan.
//
//	go test -tags yamlprobe -run TestBufferHoldsTwoTokens ./internal/scanner/
func TestBufferHoldsTwoTokens(t *testing.T) {
	for _, src := range workloadDocs(t) {
		probe.Reset()

		var s scanner.Scanner
		s.Init([]byte(src))
		for {
			if _, ok := s.NextToken(); !ok {
				break
			}
		}

		counts := probe.Counts()
		assert.LessOrEqualf(t, counts["buffer.heldAtOnce"], int64(2),
			"the scanner held %d tokens at once, where one step of scan makes at most two",
			counts["buffer.heldAtOnce"])
		// 32 is tokenBlockSizes[0], the first block the buffer takes. It never
		// needs a second.
		assert.LessOrEqualf(t, counts["buffer.roomTaken"], int64(32),
			"the buffer took room for %d tokens, where it never needs past the first block",
			counts["buffer.roomTaken"])
	}
}
