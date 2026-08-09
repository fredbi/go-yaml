// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus_test

import (
	"strings"
	"testing"

	"github.com/go-openapi/go-yaml/internal/testintegration/grammar"
	"github.com/go-openapi/go-yaml/internal/testintegration/stance"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlcorpus"
)

// TestTheBrokenDocumentsAreInvalidWhereTheGrammarCannotTell is the measurement
// that says why the deleted mutation is worth having back.
//
// Its note recorded that it produced not one document the recognizer refused
// over twenty thousand draws, and deleted it as dead weight. Both halves of
// that are reproduced here, and the second is now the point: the grammar
// accepts every one of these, and every one is invalid.
func TestTheBrokenDocumentsAreInvalidWhereTheGrammarCannotTell(t *testing.T) {
	rec := grammar.NewRecognizer(4096)

	var broken, refused int

	for _, e := range yamlcorpus.Generate(1, 1500, 0) {
		if len(e.Tags) == 0 {
			continue
		}

		broken++

		if !rec.Stream(e.Src).OK {
			refused++
		}
	}

	if broken == 0 {
		t.Fatal("nothing was broken on purpose, so the resurrection did not take")
	}

	t.Logf("%d documents broken on purpose; the grammar refuses %d of them", broken, refused)

	if refused > 0 {
		t.Errorf("%d are refused by the grammar, so they are testing syntax and not the rule they name", refused)
	}
}

// TestEveryBreakIsScoredAsARejection checks the labels reach an expectation.
//
// Being invisible to the grammar was what made these documents worthless
// before. What makes them worth generating now is that the corpus can say what
// they break, so a consumer that reads one is wrong and can be told so.
func TestEveryBreakIsScoredAsARejection(t *testing.T) {
	for _, e := range yamlcorpus.Generate(1, 400, 0) {
		if len(e.Tags) == 0 {
			continue
		}

		doc := stance.Doc{
			Name: e.Name, Src: e.Src, WellFormed: true,
			VerdictAt: stance.Construct, Tags: e.Tags,
		}

		if out, why := yamlcorpus.GoYAML.Expect(doc); out != stance.Reject {
			t.Fatalf("%s is expected to be %s (%s), and it breaks %v", e.Name, out, why, e.Tags)
		}
	}
}

// TestBothBreaksAreDrawn checks neither kind quietly stops happening.
func TestBothBreaksAreDrawn(t *testing.T) {
	seen := map[string]int{}

	for _, e := range yamlcorpus.Generate(1, 1500, 0) {
		if len(e.Tags) > 0 {
			seen[e.Mutation]++
		}
	}

	for _, want := range []string{"alias renamed to nothing", "a key repeated"} {
		if seen[want] == 0 {
			t.Errorf("no document was broken by %q", want)
		}
	}

	t.Logf("%v", seen)
}

// TestABreakIsNotAByteMutation pins the distinction the file rests on.
//
// A byte mutation cannot say what it broke, so its acceptance is claimed for
// parsing and no further. A break made on the value can, so it claims
// construction -- and if that ever stopped being true, these documents would
// start going unscored exactly where they are most useful.
func TestABreakIsNotAByteMutation(t *testing.T) {
	_, cases, err := yamlcorpus.SmokeSuite()
	if err != nil {
		t.Fatal(err)
	}

	var breaks int

	for _, c := range cases {
		if !strings.HasPrefix(c.Name, "generated/") || len(c.Tags) == 0 {
			continue
		}

		// The encoding tags a mutant can carry are about its bytes, not about a
		// rule it broke.
		var rule bool
		for _, tag := range c.Tags {
			rule = rule || strings.HasPrefix(tag, "anchor/") || strings.HasPrefix(tag, "key/")
		}

		if !rule {
			continue
		}

		breaks++

		if c.VerdictAt != stance.Construct.String() {
			t.Errorf("%s breaks a rule and claims only %q", c.Name, c.VerdictAt)
		}
	}

	if breaks == 0 {
		t.Error("no broken document reached the stored corpus")
	}

	t.Logf("%d broken documents in the corpus, all claiming construction", breaks)
}

// TestTheLibraryCatchesTheBreaks is the payoff, and the reason a mutation
// producing no refusals from the oracle was worth keeping.
//
// These are documents the grammar cannot fault and a conforming consumer must
// refuse. Whether a parser catches them is not a question a corpus of grammar
// verdicts could have asked at all.
func TestTheLibraryCatchesTheBreaks(t *testing.T) {
	_, cases, err := yamlcorpus.SmokeSuite()
	if err != nil {
		t.Fatal(err)
	}

	var caught, missed int

	for _, c := range cases {
		if !strings.HasPrefix(c.Name, "generated/") || len(c.Tags) == 0 {
			continue
		}

		doc := stance.Doc{
			Name: c.Name, Src: c.Src, WellFormed: c.WellFormed,
			VerdictAt: stance.Construct, Tags: asTags(c.Tags),
		}

		if want, _ := yamlcorpus.GoYAML.Expect(doc); want != stance.Reject {
			continue
		}

		if _, err := readStream(c.Src); err != nil {
			caught++

			continue
		}

		missed++

		if missed < 4 {
			t.Errorf("%s: read without complaint, and it breaks %v", c.Name, c.Tags)
		}
	}

	t.Logf("of the documents broken on purpose, the library catches %d and misses %d", caught, missed)

	if caught == 0 {
		t.Error("none were caught, which would make the family untested rather than passing")
	}
}
