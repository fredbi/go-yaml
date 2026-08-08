// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus

import (
	"encoding/json"
	"slices"

	"github.com/go-openapi/go-yaml/internal/testintegration/grammar"
	"github.com/go-openapi/go-yaml/internal/testintegration/stance"
	"github.com/go-openapi/go-yaml/internal/testintegration/suite"
)

// Reading names the resolution the corpus states a meaning under.
//
// The specification's own default, not ours. A consumer implementing the JSON
// schema or YAML 1.1 disagrees about some of these and is told which by the
// tags, rather than by the corpus trying to enumerate every implementation's
// answer.
const Reading = "yaml-1.2-core"

// Cases turns the enumerated families into corpus cases.
//
// Both families here are ones a verdict cannot express, which is why they are
// the first to be built rather than the last. An anchor pattern is a document
// the grammar accepts and a conforming consumer may have to refuse; a schema
// shape is a document every reading accepts and they disagree about what it
// says. Neither is reachable by generating documents and asking the oracle, so
// neither would ever appear in a corpus that only did that.
//
// The verdict is still recorded, and it is still the grammar's. What the
// construction adds is the label the grammar could not produce.
func Cases() []suite.Case {
	out := make([]suite.Case, 0, len(Patterns())+len(Resolutions()))

	rec := grammar.NewRecognizer(4096)
	a := Corpus()

	for i, p := range Patterns() {
		src := p.build(a)

		out = append(out, suite.Case{
			Name:       "shape/anchor/" + p.Name,
			Src:        src,
			WellFormed: rec.Stream(src).OK,
			Tags:       names(p.Exhibits),
			Meaning:    meaningOfPattern(p),
			Origin:     suite.Origin{Document: i, Mutation: "enumerated"},
		})
	}

	for i, r := range Resolutions() {
		src := []byte("k: " + r.Scalar + "\n")

		out = append(out, suite.Case{
			Name:       "shape/schema/" + r.Scalar,
			Src:        src,
			WellFormed: rec.Stream(src).OK,
			Tags:       names(r.Exhibits),
			Meaning:    meaningOfResolution(r),
			Origin:     suite.Origin{Document: i, Mutation: "enumerated"},
		})
	}

	return out
}

// meaningOfPattern states what an anchor pattern denotes, where it denotes
// anything.
//
// A pattern the specification says must be refused denotes nothing, so it
// carries no meaning -- recording one would be recording what a document would
// have meant had it been legal. A cyclic one carries the flag and no JSON,
// because there is no JSON to carry.
func meaningOfPattern(p Pattern) *suite.Meaning {
	if !p.Valid {
		return nil
	}

	if slices.Contains(p.Exhibits, TagCyclicMeaning) {
		return &suite.Meaning{Under: Reading, Cyclic: true}
	}

	return nil
}

// meaningOfResolution states what a plain scalar denotes under the core schema.
//
// This is the whole point of the schema family reaching the artifact. The
// document is valid under every reading, so the verdict says nothing at all,
// and the disagreement is entirely in this field.
func meaningOfResolution(r Resolution) *suite.Meaning {
	value, ok := coreValue(r)
	if !ok {
		return nil
	}

	encoded, err := json.Marshal(map[string]any{"k": value})
	if err != nil {
		return nil
	}

	return &suite.Meaning{Under: Reading, JSON: encoded}
}

// coreValue is what YAML 1.2's core schema resolves a scalar to.
//
// Written out rather than computed, because computing it would mean
// implementing the core schema here and then testing our implementation of it
// against itself. These are the specification's answers, read off its regular
// expressions by hand, and a reader who disagrees with one has something
// concrete to disagree with.
//
// The floats that JSON cannot write are absent rather than approximated:
// there is no JSON for infinity or a NaN, so a meaning stated in JSON has
// nothing to say about them and says nothing instead.
func coreValue(r Resolution) (any, bool) {
	switch r.Scalar {
	case "yes", "no", "on", "off", "y", "Yes":
		// Strings under 1.2. Booleans under 1.1, which is what the tag says.
		return r.Scalar, true
	case "0777":
		// Core's integer is decimal here: octal is spelled 0o777, and a
		// leading zero buys nothing. 1.1 read it as 511.
		return 777, true
	case "1_000", "0b1010", "1:30", "12:34:56", "2001-12-14":
		// None of these is a number in 1.2. All of them were in 1.1.
		return r.Scalar, true
	case "0x1A":
		return 26, true
	case "0o17":
		return 15, true
	case "1e3":
		// A float under core and under JSON, whatever 1.1 said.
		return 1000.0, true
	default:
		return nil, false
	}
}

// Corpus is the material the anchor patterns are composed around.
//
// Fixed for now, and deliberately not minimal: a pattern composed around a
// one-line document would not demonstrate that composing leaves the document
// alone. It becomes generated once the YAML generator can hand over a block
// mapping written at column zero, and the patterns do not have to change when
// it does.
func Corpus() Around {
	return Around{
		Document:   []byte("kept:\n  nested: 1\n  list:\n    - a\n    - b\nalso: |\n  literal\n  content\n"),
		Scalar:     []byte("plain text"),
		Other:      []byte("\"quoted\""),
		Collection: []byte("[1, 2]"),
	}
}

func names(tags []stance.Tag) []string {
	if len(tags) == 0 {
		return nil
	}

	out := make([]string, 0, len(tags))
	for _, t := range tags {
		out = append(out, string(t))
	}

	return out
}
