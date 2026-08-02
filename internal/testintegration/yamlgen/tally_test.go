// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"sort"
	"sync"
	"testing"

	"github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"
)

// tally counts how often each ledger entry was drawn, and how often it actually
// diverged.
//
// It reports rather than asserts, because how often a shape is drawn depends on
// how many cases the run was asked for: at the default hundred a rare shape is
// often not drawn at all, and an entry whose predicate is very slightly wider
// than its defect can be drawn once and not diverge. Failing on either would be
// a test that fails on Tuesdays.
//
// The ratchet lives in defects_test.go instead, where each entry has one pinned
// document that diverges deterministically. What the counts are good for is
// judging a predicate: drawn and diverged should be equal, and a gap between
// them means the entry is tolerating documents that are fine.
type tally struct {
	mx     sync.Mutex
	drawn  map[string]int
	failed map[string]int
}

func newTally() *tally {
	return &tally{drawn: make(map[string]int), failed: make(map[string]int)}
}

func (c *tally) record(name string, diverged bool) {
	c.mx.Lock()
	defer c.mx.Unlock()

	c.drawn[name]++
	if diverged {
		c.failed[name]++
	}
}

func (c *tally) report(t *testing.T, p yamlgen.Property) {
	t.Helper()

	c.mx.Lock()
	defer c.mx.Unlock()

	entries := yamlgen.Entries(p)
	names := make([]string, 0, len(entries))
	for _, d := range entries {
		names = append(names, d.Name)
	}
	sort.Strings(names)

	for _, name := range names {
		drawn, failed := c.drawn[name], c.failed[name]
		if drawn == 0 {
			t.Logf("ledger %q: not drawn in this run", name)

			continue
		}

		t.Logf("ledger %q: drawn %d times, diverged %d", name, drawn, failed)
	}
}
