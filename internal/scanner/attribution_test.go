// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:build yamlprobe

package scanner

import (
	"sort"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/fuzzseeds"
	"github.com/go-openapi/go-yaml/internal/probe"
)

// TestBulkSkipMovesNoInvariant reads the corpus with the bulk skip on and off and
// compares every invariant held by the ledger.
//
// It answers the question: did the scanner move, or did the corpus?
//
// The ledger's counts drift whenever documents are added.
//
// Running both paths in one process settles it. The corpus is the same on both sides, so any difference is the scanner's.
//
// Nothing may raise this: the bulk skip stands in for what the character loop did, so an invariant it moves is one it stands in for wrongly.
func TestBulkSkipMovesNoInvariant(t *testing.T) {
	seeds, err := fuzzseeds.All()
	require.NoError(t, err)
	require.NotEmpty(t, seeds)

	read := func(skip bool) map[string]probe.Invariant {
		alnumFastPath = skip
		defer func() { alnumFastPath = true }()

		probe.Reset()
		for _, src := range seeds {
			var s Scanner
			s.Init([]byte(src))
			for {
				if _, ok := s.NextToken(); !ok {
					break
				}
			}
		}

		out := make(map[string]probe.Invariant, len(probe.Checks()))
		for name, inv := range probe.Checks() {
			out[name] = probe.Invariant{Tested: inv.Tested, Failed: inv.Failed}
		}

		return out
	}

	off, on := read(false), read(true)
	require.NotEmpty(t, off, "the probe recorded nothing; is the yamlprobe tag on?")

	names := make([]string, 0, len(off))
	for name := range off {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		o, n := off[name], on[name]
		assert.Equalf(t, o.Failed, n.Failed,
			"%s: %d disagreements with the bulk skip off, %d with it on", name, o.Failed, n.Failed)
		assert.Equalf(t, o.Tested, n.Tested,
			"%s: tested %d times with the bulk skip off, %d with it on", name, o.Tested, n.Tested)
		t.Logf("%-40s %d of %d, either way", name, n.Failed, n.Tested)
	}
}
