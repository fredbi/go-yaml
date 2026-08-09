// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus_test

import (
	"testing"

	"github.com/go-openapi/go-yaml/internal/testintegration/grammar"
	"github.com/go-openapi/go-yaml/internal/testintegration/stance"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlcorpus"
)

// TestTheGrammarHasNoOpinionAboutMerge is why the family is enumerated.
//
// Every merge shape is a document the grammar accepts, the ones a merging
// parser must refuse included. "<<" is an ordinary plain scalar to a grammar --
// there is no production for it and there could not be, since 1.2 does not
// define the type at all.
func TestTheGrammarHasNoOpinionAboutMerge(t *testing.T) {
	rec := grammar.NewRecognizer(1024)

	for _, s := range yamlcorpus.MergeShapes() {
		if got := rec.Stream(s.Src); !got.OK {
			t.Errorf("%s: the grammar refuses it, so it is testing syntax: %q", s.Name, string(s.Src))
		}
	}
}

// TestNoMergeQuestionIsSettled is the assertion that keeps this family from
// becoming a judgement.
//
// No version of the specification we test against defines "<<", so a rule here
// would be the corpus picking a version of the language and calling every other
// implementation defective. libfyaml reads "<<" as an ordinary key and this
// library merges; both are right.
func TestNoMergeQuestionIsSettled(t *testing.T) {
	rules := yamlcorpus.KeyRules()
	rules = append(rules, yamlcorpus.AnchorRules()...)
	rules = append(rules, yamlcorpus.TagRules()...)

	for tag := range yamlcorpus.MergeVocabulary() {
		if _, settled := rules.Of(tag); settled {
			t.Errorf("%s is settled by a rule, and merge is not ours to settle", tag)
		}
	}
}

// TestTwoMergeKeysAreAlsoADuplicate checks the families interlock rather than
// sitting side by side.
//
// Two "<<" keys are a duplicate key whatever merge means, so that document
// carries the unique-key rule as well and every conforming consumer refuses it
// -- a merging one because 1.1 allows a single merge key, everyone else because
// the specification forbids two equal keys. One document, two reasons, and the
// corpus can state both.
func TestTwoMergeKeysAreAlsoADuplicate(t *testing.T) {
	for _, s := range yamlcorpus.MergeShapes() {
		if s.Name != "two merge keys in one mapping" {
			continue
		}

		var duplicate bool
		for _, tag := range s.Intent {
			duplicate = duplicate || tag == yamlcorpus.TagDuplicateKey
		}

		if !duplicate {
			t.Error("two merge keys are a duplicate key and the shape does not say so")
		}

		// And the rule that settles it makes the document a rejection for
		// anybody, merging or not.
		doc := stance.Doc{
			Name: s.Name, Src: s.Src, WellFormed: true,
			VerdictAt: stance.Construct, Tags: s.Intent,
		}

		if out, why := yamlcorpus.GoYAML.Expect(doc); out != stance.Reject {
			t.Errorf("expected a rejection, got %s (%s)", out, why)
		}

		return
	}

	t.Error("the shape is gone, so this test is checking nothing")
}
