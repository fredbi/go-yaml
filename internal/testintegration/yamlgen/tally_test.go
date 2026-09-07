// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"fmt"
	"os"
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
//
// # One thing they do assert
//
// Every tally feeds a package-wide total that TestMain checks after the run.
// An entry drawn many times and never diverging is either fixed or matching a
// family it is not in, and both cost the same thing: every document it matches
// is excused from the property and never compared.
//
// Both were live on 2026-09-13 and both had been logging their zero for days.
// decode/one-non-string-key-zeroes-a-whole-struct was fixed by 7dc4075 on
// 2026-09-07 and its entry stayed, excusing 1,256 documents a run;
// decode/a-key-after-a-long-tag-on-an-empty-value-is-not-resolved matched any
// tagged null anywhere in a tree and excused 1,077, where the defect needs a
// resolving key on the line below. Narrowing it took the draws to 24.
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

	totals.add(c.drawn, c.failed)
}

// suspectAfter is how many draws an entry has to reach before never diverging
// counts against it.
//
// Measured rather than picked. Over a 40,000-draw run on 2026-09-13 the live
// entries that diverge least often are drawn 102 and 141 times, and the two
// that were suppressing coverage were drawn 1,256 and 1,077 -- so the gap is
// wide and 200 sits in it. A short run reaches nothing here and the check stays
// quiet, which is the point: it should fire on a real sweep and never on a
// developer's default `go test`.
const suspectAfter = 200

// totals accumulates every tally in the package, since each property test has
// its own and an entry claims several.
var totals = &tally{drawn: make(map[string]int), failed: make(map[string]int)}

func (c *tally) add(drawn, failed map[string]int) {
	c.mx.Lock()
	defer c.mx.Unlock()

	for name, n := range drawn {
		c.drawn[name] += n
	}

	for name, n := range failed {
		c.failed[name] += n
	}
}

// stale returns the entries drawn past [suspectAfter] that never diverged.
func (c *tally) stale() []string {
	c.mx.Lock()
	defer c.mx.Unlock()

	var out []string

	for name, drawn := range c.drawn {
		if drawn >= suspectAfter && c.failed[name] == 0 {
			out = append(out, fmt.Sprintf("%q: drawn %d times across the properties and never diverged", name, drawn))
		}
	}

	sort.Strings(out)

	return out
}

// TestMain runs the package and then holds the ledger to its own counts.
func TestMain(m *testing.M) {
	code := m.Run()

	if suspect := totals.stale(); len(suspect) > 0 {
		fmt.Fprintln(os.Stderr, "\nledger entries that excuse documents and record nothing:")
		for _, line := range suspect {
			fmt.Fprintln(os.Stderr, "  "+line)
		}
		fmt.Fprintln(os.Stderr,
			"Either the defect is fixed and the entry goes, or the predicate matches a family the\n"+
				"defect is not in and wants narrowing. Every document matched here was excused from\n"+
				"its property and never compared.")

		if code == 0 {
			code = 1
		}
	}

	os.Exit(code)
}
