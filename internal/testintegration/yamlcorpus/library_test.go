// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	yaml "github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/testintegration/stance"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlcorpus"
	"github.com/go-openapi/go-yaml/parser"
)

// reads reports whether the library reads a whole stream, which is the question
// the corpus asks at construct.
//
// A stream and not a document: yaml.Unmarshal into one value stops at the first
// document and reports success, so it would have said nothing at all about the
// pattern that puts an anchor in one document and an alias in the next. That
// mistake was made once here before the ledger below was written, which is why
// it is spelled out rather than left to whoever reads this next.
func reads(src []byte) error {
	dec := yaml.NewDecoder(bytes.NewReader(src))

	for {
		var v any

		err := dec.Decode(&v)
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return err
		}
	}
}

// TestTheLibraryMatchesItsDeclaredStance runs every pattern through the library
// and holds the result to what the rules require and the stance declares.
//
// This is the measurement the stance table exists to be checked against. A
// table nobody re-measures is a claim about a library as it once was, and the
// whole point of writing it down was that a corpus scored against a wish
// reports the wish's failures as the library's.
func TestTheLibraryMatchesItsDeclaredStance(t *testing.T) {
	known := map[string]yamlcorpus.Departure{}

	for _, d := range yamlcorpus.Departures {
		if d.Kind == yamlcorpus.Verdict {
			known[d.Pattern] = d
		}
	}

	var agreed []string

	for _, p := range yamlcorpus.Patterns() {
		src := build(t, p.Name)

		doc := stance.Doc{
			Name:       p.Name,
			Src:        src,
			WellFormed: true, // asserted separately: the grammar accepts every pattern
			Tags:       p.Exhibits,
		}

		want, why := yamlcorpus.GoYAML.Expect(doc)
		got := reads(src)

		matches := (want == stance.Accept && got == nil) || (want == stance.Reject && got != nil)

		departure, declared := known[p.Name]

		switch {
		case matches && declared:
			agreed = append(agreed, p.Name+" -- "+departure.Observed)
		case matches, declared:
		default:
			t.Errorf("%s\n  expected %s, because %s\n  library: %v\n"+
				"  this is either a defect nobody has written down or a stance that has drifted",
				p.Name, want, why, got)
		}
	}

	// A departure that has stopped departing is a stale entry, and a stale
	// entry describes a defect somebody has already fixed.
	for _, name := range agreed {
		t.Errorf("no longer departs, so its ledger entry is stale: %s", name)
	}
}

// TestEveryPatternParses separates the two things the library could be doing.
//
// The departures below are about composing and constructing, not about syntax,
// and that only means anything if the parser reads all twelve. A pattern the
// parser refuses would be a grammar disagreement wearing an anchor's clothes.
func TestEveryPatternParses(t *testing.T) {
	for _, p := range yamlcorpus.Patterns() {
		src := build(t, p.Name)

		if _, err := parser.ParseBytes(src, parser.ParseComments); err != nil {
			t.Errorf("%s: the parser refuses it, so the pattern tests the wrong layer: %v", p.Name, err)
		}
	}
}

// TestTheValueDeparturesAreStillThere measures the ones a verdict cannot see.
//
// A cycle is read without complaint and comes back with nil where the cycle
// was. The verdict is right -- the document is valid and the library accepts it
// -- and the value is wrong, so no corpus of accept-or-refuse will ever find
// this. It is measured directly instead, and the measurement is the only thing
// standing between the ledger and fiction.
func TestTheValueDeparturesAreStillThere(t *testing.T) {
	var v any
	if err := yaml.Unmarshal([]byte("recursive: &x [ *x ]\n"), &v); err != nil {
		t.Fatalf("the cycle is now refused, so the ledger entry is stale: %v", err)
	}

	top, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("decoded to %T, and the ledger describes a mapping", v)
	}

	seq, ok := top["recursive"].([]any)
	if !ok || len(seq) != 1 {
		t.Fatalf("decoded to %#v, and the ledger describes a one-element sequence", top["recursive"])
	}

	if seq[0] != nil {
		t.Errorf("the cycle decodes to %#v, and the ledger says nil -- the entry needs rewriting", seq[0])
	}
}

// TestEveryDepartureNamesAPattern keeps the ledger anchored to something that
// can be run, rather than to prose about a document nobody has.
func TestEveryDepartureNamesAPattern(t *testing.T) {
	names := map[string]bool{}
	for _, p := range yamlcorpus.Patterns() {
		names[p.Name] = true
	}

	for _, d := range yamlcorpus.Departures {
		if !names[d.Pattern] {
			t.Errorf("%q is not a pattern, so nothing reproduces this departure", d.Pattern)
		}
	}
}

func build(t *testing.T, name string) []byte {
	t.Helper()

	for _, s := range yamlcorpus.Shapes(around()) {
		if s.Name == name {
			return s.Src
		}
	}

	t.Fatalf("no pattern named %q", name)

	return nil
}
