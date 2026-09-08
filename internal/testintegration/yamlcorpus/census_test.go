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
// It is empty as of 2026-09-08, and it started with the two gaps this census
// found on the run it was written -- which is the argument for having it, since
// the grammar coverage number was at 605 of 605 and said nothing about either.
// A tab between a property and its node closed on 2026-09-07 with
// Style.TabSeparation; an explicit key whose content sits below the '?' closed
// here, when Keys() began drawing a collection key and emit.go's explicitKey
// wrote it the long way.
var knownGaps = map[string]string{}

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
		for _, sh := range yamlcorpus.Constructs() {
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

		for _, sh := range yamlcorpus.Constructs() {
			if sh.Has(string(c.Src)) {
				into[sh.Name]++
			}
		}
	}

	var missing []string
	for _, sh := range yamlcorpus.Constructs() {
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

	t.Logf("%d shapes, %d suite documents, %d corpus cases", len(yamlcorpus.Constructs()), len(suites), len(cases))
}
