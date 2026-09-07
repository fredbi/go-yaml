// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/go-openapi/go-yaml/internal/testintegration/yamlcorpus"
	"github.com/go-openapi/go-yaml/internal/yamltestsuite"
)

// bom is U+FEFF, written as an escape because Go source may not hold one.
const bom = "\ufeff"

// shape is one structural thing a document may contain, recognized in the bytes.
type shape struct {
	Name string
	Has  func(src string) bool
}

func lines(src string) []string {
	return strings.FieldsFunc(src, func(r rune) bool { return r == '\n' || r == '\r' })
}

// trimmed is a line with a byte order mark and leading indentation removed.
func trimmed(line string) string {
	return strings.TrimLeft(strings.TrimPrefix(line, bom), " \t")
}

func anyLine(src string, ok func(string) bool) bool {
	for _, line := range lines(src) {
		if ok(trimmed(line)) {
			return true
		}
	}

	return false
}

var shapes = []shape{
	{"an explicit key", func(s string) bool {
		return anyLine(s, func(l string) bool { return l == "?" || strings.HasPrefix(l, "? ") })
	}},
	{"an explicit key alone on its line", func(s string) bool {
		return anyLine(s, func(l string) bool { return l == "?" })
	}},
	{"an explicit key's value line", func(s string) bool {
		return anyLine(s, func(l string) bool { return l == ":" || strings.HasPrefix(l, ": ") })
	}},
	{"an explicit key inside a flow collection", func(s string) bool {
		return strings.Contains(s, "{?") || strings.Contains(s, ", ?") || strings.Contains(s, "[?")
	}},
	{"a block literal", func(s string) bool { return strings.Contains(s, "|") }},
	{"a block folded", func(s string) bool { return strings.Contains(s, ">") }},
	{"an indentation indicator", func(s string) bool {
		for _, c := range []string{"|1", "|2", "|3", ">1", ">2", ">3"} {
			if strings.Contains(s, c) {
				return true
			}
		}

		return false
	}},
	{"a chomping indicator", func(s string) bool {
		for _, c := range []string{"|-", "|+", ">-", ">+"} {
			if strings.Contains(s, c) {
				return true
			}
		}

		return false
	}},
	{"a flow mapping", func(s string) bool { return strings.Contains(s, "{") }},
	{"a flow sequence", func(s string) bool { return strings.Contains(s, "[") }},
	{"an anchor", func(s string) bool { return strings.Contains(s, "&") }},
	{"an alias", func(s string) bool { return strings.Contains(s, "*") }},
	{"a merge key", func(s string) bool { return strings.Contains(s, "<<") }},
	{"a secondary tag", func(s string) bool { return strings.Contains(s, "!!") }},
	{"a verbatim tag", func(s string) bool { return strings.Contains(s, "!<") }},
	{"a comment", func(s string) bool { return strings.Contains(s, "#") }},
	{"a document marker", func(s string) bool {
		return anyLine(s, func(l string) bool { return l == "---" || strings.HasPrefix(l, "--- ") })
	}},
	{"a document suffix", func(s string) bool {
		return anyLine(s, func(l string) bool { return l == "..." })
	}},
	{"a YAML directive", func(s string) bool { return strings.Contains(s, "%YAML") }},
	{"a TAG directive", func(s string) bool { return strings.Contains(s, "%TAG") }},
	{"a single-quoted scalar", func(s string) bool { return strings.Contains(s, "'") }},
	{"a double-quoted scalar", func(s string) bool { return strings.Contains(s, `"`) }},
	{"a block sequence entry", func(s string) bool {
		return anyLine(s, func(l string) bool { return l == "-" || strings.HasPrefix(l, "- ") })
	}},
	{"a tab", func(s string) bool { return strings.Contains(s, "\t") }},
	{"a byte order mark", func(s string) bool { return strings.Contains(s, bom) }},
	{"a carriage return", func(s string) bool { return strings.Contains(s, "\r") }},
	{"an empty flow collection", func(s string) bool {
		return strings.Contains(s, "{}") || strings.Contains(s, "[]")
	}},
	{"a zero-indented sequence under a key", func(s string) bool {
		ls := lines(s)
		for i := 0; i+1 < len(ls); i++ {
			key := trimmed(ls[i])
			next := ls[i+1]
			if strings.HasSuffix(key, ":") && len(next) > 0 && next[0] == '-' {
				return true
			}
		}

		return false
	}},
	{"an explicit key whose content is on the line below the '?'", func(s string) bool {
		// The ':' value line does not count. A "?" alone above ": v" is an entry
		// whose key is the empty node, which the generator writes often; the
		// shape here is 8.2.2's s-l+block-indented placing the KEY below its
		// indicator, which is a different thing and the one the suite has.
		ls := lines(s)
		for i := 0; i+1 < len(ls); i++ {
			next := trimmed(ls[i+1])
			if trimmed(ls[i]) != "?" || next == "" {
				continue
			}

			if next == ":" || strings.HasPrefix(next, ": ") {
				continue
			}

			return true
		}

		return false
	}},
	{"a nested explicit key", func(s string) bool { return strings.Contains(s, "? ?") }},
}

// TestTheGeneratedCorpusIsWiderThanTheSuite is the census.
//
// The generated corpus is meant to be strictly wider than the official YAML
// Test Suite: every structural thing a suite document contains, some generated
// document should contain too. Where it does not, the generator has a shape it
// cannot write, and no amount of drawing will reach it.
//
// # Why the grammar coverage number cannot say this
//
// TestTheCorpusReachesMostOfTheGrammar reports 605 of 605 production-by-context
// buckets entered, and is blind to this class by construction. An explicit key
// whose content is on the next line enters the same productions as one whose
// content is on the same line -- s-l+block-indented either way. The difference
// is which characters, not which rule, so a saturated bucket count and a missing
// shape sit together comfortably. This measures what a document *contains*.
//
// # What this cannot say
//
// The list below is hand-maintained, so the census has the same kind of gap it
// exists to find: a shape nobody wrote a predicate for is invisible here. That
// is worth stating rather than hiding. And the dangerous direction is the
// opposite of the usual one: a predicate that is too BROAD reports a shape as
// covered when it is not, which is a census that passes and says nothing. The
// first draft of "an explicit key whose content is on the line below" matched
// the ':' value line and reported 24 generated documents for a shape the
// generator cannot write at all.
// knownGaps are the shapes the suite has and the generator cannot write, with
// why, so that this test fails on a NEW gap rather than on the standing ones.
//
// The same shape as yamlcorpus.parserComplaints and yamlgen.Lax: what must hold
// is asserted, and the direction that means progress is logged. Closing one is
// reported rather than failed, so a fix does not go red.
//
// Both were found by this census on the run it was written, which is the
// argument for having it: the grammar coverage number was at 605 of 605 and
// said nothing about either.
var knownGaps = map[string]string{
	"an explicit key whose content is on the line below the '?'": "emit.go's explicitKey writes keyIn(k, false), " +
		"which returns a single-line scalar, so the key always lands on the '?'s own line; and Keys() draws only " +
		"scalars, so a collection key -- which is what the suite's two documents put below the '?' -- has no value " +
		"to draw from. Two changes, neither small.",
	"a tab": "no axis writes one. A tab is s-white and not s-indent, so it separates where it may not indent, " +
		"and 56 suite documents turn on that distinction. The generator writes spaces everywhere and reaches " +
		"none of it. Found by this census on 2026-09-07; nothing else had noticed.",
}

func TestTheGeneratedCorpusIsWiderThanTheSuite(t *testing.T) {
	suites, err := yamltestsuite.TestSuites()
	if err != nil {
		t.Skipf("the YAML Test Suite is not available: %v", err)
	}

	_, cases, err := yamlcorpus.SmokeSuite()
	if err != nil {
		t.Fatal(err)
	}

	inSuite := map[string]int{}
	for _, s := range suites {
		for _, sh := range shapes {
			if sh.Has(string(s.InYAML)) {
				inSuite[sh.Name]++
			}
		}
	}

	// Counted three ways, and the split is the whole point. A shape the
	// generator draws is one it can write. A shape only an enumerated case
	// holds was written by hand, which is a legitimate answer to a gap and not
	// the same claim. A shape only a byte MUTANT holds is covered by accident:
	// a mutation corrupted a document into it, so the corpus carries the bytes
	// and the generator still cannot produce the shape. The first draft counted
	// all three together and reported the explicit-key gap as covered, on the
	// strength of two "insert an indicator" mutants and one deleted byte.
	drawn, byHand, mutated := map[string]int{}, map[string]int{}, map[string]int{}

	for _, c := range cases {
		into := drawn

		switch {
		case c.Origin.Mutation != "" && c.Origin.Mutation != "enumerated":
			into = mutated
		case strings.HasPrefix(c.Name, "shape/"):
			into = byHand
		}

		for _, sh := range shapes {
			if sh.Has(string(c.Src)) {
				into[sh.Name]++
			}
		}
	}

	var missing []string
	for _, sh := range shapes {
		t.Logf("%-58s suite %4d   drawn %6d   by hand %3d   mutated %5d",
			sh.Name, inSuite[sh.Name], drawn[sh.Name], byHand[sh.Name], mutated[sh.Name])

		if inSuite[sh.Name] == 0 || drawn[sh.Name] > 0 || byHand[sh.Name] > 0 {
			continue
		}

		if why, known := knownGaps[sh.Name]; known {
			t.Logf("KNOWN GAP %q: %s", sh.Name, why)

			continue
		}

		missing = append(missing, fmt.Sprintf(
			"%q: %d suite documents hold it, no generated or enumerated case does, and %d mutants hold it by accident",
			sh.Name, inSuite[sh.Name], mutated[sh.Name]))
	}

	for name, why := range knownGaps {
		if drawn[name] > 0 || byHand[name] > 0 {
			t.Logf("CLOSED %q is reached now -- drop it from knownGaps (%s)", name, why)
		}
	}

	sort.Strings(missing)
	for _, line := range missing {
		t.Errorf("%s", line)
	}

	t.Logf("%d shapes, %d suite documents, %d corpus cases", len(shapes), len(suites), len(cases))
}
