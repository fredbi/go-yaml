// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus_test

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/go-openapi/go-yaml/internal/testintegration/grammar"
	"github.com/go-openapi/go-yaml/internal/testintegration/stance"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlcorpus"
)

// around stands in for what the generator will supply, and is deliberately not
// minimal: a pattern composed around a one-line document would not show that
// composing preserves the document it was composed around.
func around() yamlcorpus.Around {
	return yamlcorpus.Around{
		Document:   []byte("kept:\n  nested: 1\n  list:\n    - a\n    - b\nalso: |\n  literal\n  content\n"),
		Scalar:     []byte("plain text"),
		Other:      []byte("\"quoted\""),
		Collection: []byte("[1, 2]"),
	}
}

// TestTheGrammarCannotSeeAnyOfThis is the measurement that says why this file
// exists, and it is the one worth failing loudly.
//
// Every pattern -- the three violations included -- is a document the YAML 1.2
// grammar accepts. No production can consult a table of the anchors it has
// already passed, so "*x" is recognized wherever an alias may appear and the
// grammar has no opinion at all about whether x was ever anchored.
//
// That is the entire argument for enumerating these and labeling them by
// construction. A corpus caching the oracle's verdict would record all nine as
// well formed, and three of them would then accuse a correct parser of a defect
// for refusing exactly what the specification tells it to refuse.
func TestTheGrammarCannotSeeAnyOfThis(t *testing.T) {
	rec := grammar.NewRecognizer(1024)

	for _, s := range yamlcorpus.Shapes(around()) {
		t.Run(s.Name, func(t *testing.T) {
			if got := rec.Stream(s.Src); !got.OK {
				t.Fatalf("the grammar refuses this document, so the pattern is malformed rather than "+
					"illustrative:\n%s", strconv.Quote(string(s.Src)))
			}
		})
	}
}

// TestThePatternsDisagreeWithTheGrammarExactlyWhereTheyShould pins which of
// them are violations.
//
// The count matters as much as the membership. Three of nine patterns are
// documents a conforming parser must refuse and the grammar accepts, and six
// are documents both accept -- and the six are not filler. A parser is at least
// as likely to wrongly refuse a legal anchor as to wrongly accept an illegal
// alias, and only one of those two mistakes is the one everybody thinks to test
// for.
func TestThePatternsDisagreeWithTheGrammarExactlyWhereTheyShould(t *testing.T) {
	var violations, legal []string

	for _, p := range yamlcorpus.Patterns() {
		if p.Valid {
			legal = append(legal, p.Name)

			continue
		}

		violations = append(violations, p.Name)
	}

	if len(violations) != 3 {
		t.Errorf("%d patterns are violations, expected 3: %v", len(violations), violations)
	}

	if len(legal) != 6 {
		t.Errorf("%d patterns are legal, expected 6: %v", len(legal), legal)
	}
}

// TestEveryRuleIsSettledOrDeliberatelyNot checks the two halves line up.
//
// A tag a pattern puts on a document and no rule settles is a document nobody
// can score, which is the failure mode this whole layer was built to avoid. The
// one exception is stated rather than tolerated: whether a parser can represent
// a cycle is a property of its data model, not of YAML.
func TestEveryRuleIsSettledOrDeliberatelyNot(t *testing.T) {
	rules := yamlcorpus.AnchorRules()

	open := map[stance.Tag]bool{yamlcorpus.TagAliasRecursive: true}

	for _, p := range yamlcorpus.Patterns() {
		for _, tag := range p.Exhibits {
			if _, settled := rules.Of(tag); settled || open[tag] {
				continue
			}

			t.Errorf("%q exhibits %s, which no rule settles and nothing declares open", p.Name, tag)
		}
	}
}

// TestAStanceCannotVoteAwayARule is the property [stance.Rule] exists for.
//
// A parser declaring that it accepts undefined aliases is not taking a
// position, it is describing a defect, and a corpus that let the declaration
// stand would score the defect as conformance. The declaration is ignored and
// reported instead.
func TestAStanceCannotVoteAwayARule(t *testing.T) {
	rules := yamlcorpus.AnchorRules()

	wishful := stance.Table{
		Name:     "a parser that would rather not check",
		Requires: rules,
		Stands:   map[stance.Tag]stance.Stand{yamlcorpus.TagAliasUndefined: stance.Accepts},
	}

	doc := stance.Doc{
		Name:       "an undefined alias",
		WellFormed: true, // what the grammar says, and it is not enough
		Tags:       []stance.Tag{yamlcorpus.TagAliasUndefined},
	}

	if out, why := wishful.Expect(doc); out != stance.Reject {
		t.Errorf("a stance voted a settled rule away: %s (%s)", out, why)
	}

	if got := rules.Contradicted(wishful); len(got) != 1 {
		t.Errorf("the contradiction went unreported: %v", got)
	}
}

// TestAParserMayDeclineACheckWithoutFailingIt is the other side of the same
// property.
//
// A parser that never resolves aliases genuinely cannot answer this, and
// scoring it as wrong would be as dishonest as scoring it as right. Declining
// leaves the document unscored, which is the only truthful third option.
func TestAParserMayDeclineACheckWithoutFailingIt(t *testing.T) {
	honest := stance.Table{
		Name:     "an event-level parser",
		Requires: yamlcorpus.AnchorRules(),
		Stands:   map[stance.Tag]stance.Stand{yamlcorpus.TagAliasUndefined: stance.Either},
	}

	doc := stance.Doc{
		Name:       "an undefined alias",
		WellFormed: true,
		Tags:       []stance.Tag{yamlcorpus.TagAliasUndefined},
	}

	if out, why := honest.Expect(doc); out != stance.Undecided {
		t.Errorf("declining a check should leave a document unscored, got %s (%s)", out, why)
	}
}

// TestASettledRuleNeedsNoDeclaring checks that a table is not reported as
// incomplete for failing to restate what the language already says.
func TestASettledRuleNeedsNoDeclaring(t *testing.T) {
	table := stance.Table{
		Name:     "a parser that says nothing at all",
		Requires: yamlcorpus.AnchorRules(),
	}

	docs := []stance.Doc{{
		Name: "an undefined alias",
		Tags: []stance.Tag{yamlcorpus.TagAliasUndefined},
	}}

	if missing := table.Undeclared(docs); len(missing) > 0 {
		t.Errorf("a settled rule was reported as an undeclared gap: %v", missing)
	}
}

// TestAPatternKeepsTheDocumentItWasBuiltAround is what makes these patterns
// rather than fixtures.
//
// Each one is a real document with one reference added, so the rule is
// exercised alongside whatever else the generator drew rather than on its own.
// A pattern that dropped its document would still pass every test above and
// would have stopped being worth generating.
func TestAPatternKeepsTheDocumentItWasBuiltAround(t *testing.T) {
	a := around()

	for _, s := range yamlcorpus.Shapes(a) {
		t.Run(s.Name, func(t *testing.T) {
			if !bytes.Contains(s.Src, a.Document) {
				t.Errorf("the document it was built around is not in the result:\n%s",
					strconv.Quote(string(s.Src)))
			}
		})
	}
}
