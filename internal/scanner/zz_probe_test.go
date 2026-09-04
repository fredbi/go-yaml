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
	// notSpaceCharPos marks the buffer's length less the whitespace it ends
	// with. Computing it at the read instead -- a scan back over the buffer,
	// once, rather than a compare and a store for every character written --
	// would give the same answer 137,103 times in 137,129. The 26 that differ
	// are unaccounted for, and 26 is not zero.
	"buf.notSpaceCharPos==trimmed": 26,

	// The indentation counted so far and the column reached are the same number
	// while a line is still opening, but for twelve cases in 34,174.
	"indent.indentNum==column-1": 12,

	// The indent level a token was given and the level the scanner stands at
	// part company where a block opens: 3,000 of 76,277.
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
