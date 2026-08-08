// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus

import "github.com/go-openapi/go-yaml/internal/testintegration/stance"

// GoYAML is what this library does with the questions YAML leaves open, at the
// stage its decoder works at.
//
// Measured rather than declared, like the JSON table it follows. A stance
// written from what a library ought to do is a wish, and a corpus scored
// against a wish reports the wish's failures as the library's.
//
// Only genuinely open questions are here. Where the specification settles
// something, [AnchorRules] carries it and this table gets no vote -- so a
// difference between what the rules require and what the library does is a
// defect, and it goes in [Departures] rather than being written in here as
// though it were a position.
var GoYAML = stance.Table{
	Name:     "go-openapi/go-yaml decoder into any",
	Because:  "decodes into Go values, which hold more shapes than JSON does and fewer than YAML admits",
	At:       stance.Construct,
	Speaks:   Vocabulary(),
	Requires: allRules(),
	Stands: map[stance.Tag]stance.Stand{
		// Measured: "first: &x [1, 2]\n*x : keyed\n" decodes, with the sequence
		// as a key. A Go map key may be any comparable value and the decoder
		// does not insist on a string, so a document JSON could not hold is one
		// this library reads.
		TagKeyNotAScalar: stance.Accepts,

		// Measured: a cycle is read without complaint, and this is the one
		// entry that is uncomfortable. The document is accepted, so Accepts is
		// what the verdict says -- but the value that comes back has nil where
		// the cycle was, which is neither holding the cycle nor refusing it.
		// See Departures: the verdict is right and the value is not, and this
		// table can only speak about verdicts.
		TagCyclicMeaning: stance.Accepts,

		// Measured on the tag family. Everything the specification leaves to
		// the application, this library reads: a local tag, a handle a %TAG
		// declared, a percent escape in a tag URI, a version directive. None of
		// them is a position anybody would call surprising, and the value of
		// writing them down is that a change to any of them fails a test rather
		// than surprising a consumer.
		TagLocal:         stance.Accepts,
		TagNamedHandle:   stance.Accepts,
		TagPercentEscape: stance.Accepts,
		TagYAMLDirective: stance.Accepts,
	},
}

// Kind says what part of a library's behavior a departure is about.
type Kind uint8

const (
	// Verdict is a document read that should have been refused, or refused
	// that should have been read. A corpus of verdicts finds these.
	Verdict Kind = iota
	// Value is a document read correctly and turned into the wrong thing. A
	// corpus of verdicts is blind to these by construction, which is why they
	// are worth naming separately rather than counting together.
	Value
)

func (k Kind) String() string {
	if k == Value {
		return "value"
	}

	return "verdict"
}

// Departure is a measured difference between what YAML 1.2 requires and what
// this library does.
//
// An entry is not an excuse and not a fix: this branch builds the tools and
// does not touch the parser. It is a measurement with a name attached, asserted
// exactly, so that a departure which gets fixed fails this package rather than
// sitting here describing a world that has moved on.
type Departure struct {
	// Pattern is the [Pattern] that exposes it.
	Pattern string
	// Kind is whether the verdict or the value is wrong.
	Kind Kind
	// Observed is what the library does.
	Observed string
	// Because is what the specification requires instead.
	Because string
	// Corroborated names an independent implementation that agrees with the
	// specification and not with us, where one has been consulted. A departure
	// resting only on our own reading of the prose is a weaker claim and should
	// say so by leaving this empty.
	Corroborated string
}

// Departures is what the anchor patterns found, on first contact.
//
// Both were found the day the patterns were written, and neither appears
// anywhere in the four hundred documents of the YAML Test Suite. That is the
// argument for the patterns in one sentence.
var Departures = []Departure{
	{
		Pattern:  "an alias to an anchor in an earlier document",
		Kind:     Verdict,
		Observed: `the second document resolves *x to the first document's anchor, and no error is raised`,
		Because:  "3.2.2.2: anchor names are local to a document, so the alias names nothing and the stream is in error",
		// ⚠ Contested, and recorded as such rather than quietly kept.
		//
		// PyYAML 6.0.1 raises ComposerError on these bytes, which is what this
		// entry was written on. libfyaml 1.0.0a8 reads the stream and resolves
		// the alias exactly as this library does. Two conforming
		// implementations disagree, so one witness is not enough and this is
		// not yet a defect anybody should act on.
		//
		// The remaining doubt is whether libfyaml's binding shares one anchor
		// table across a stream by its own choice rather than the library's,
		// which nothing available here can separate.
		Corroborated: "contested: PyYAML 6.0.1 refuses, libfyaml 1.0.0a8 accepts",
	},
	{
		Pattern:  "two keys alike in text and different once resolved",
		Kind:     Verdict,
		Observed: `"1: x" and "\"1\": y" in one mapping are refused as a duplicate key`,
		Because: "3.2.1.1: keys are equal when they resolve to the same node, and these resolve to an integer " +
			"and a string, so they are two keys and the document is valid",
		Corroborated: "libfyaml 1.0.0a8 keeps both, and merges 1 with !!int 1 -- so its key identity is " +
			"resolution and not spelling",
	},
	{
		Pattern:  "a sequence holding an alias to itself",
		Kind:     Value,
		Observed: "the document is read and the cycle decodes to nil, so &x [ *x ] becomes a one-element list holding nothing",
		Because: "3.2.1: the representation is a graph and the alias resolves to the node it is inside; " +
			"a model that cannot hold that has to say so rather than substitute a value the document never had",
	},
}

// GoYAMLParser is the same library asked the question it actually answers at
// parsing: is this a document.
//
// Two tables for one library, and the corpus is built to make that ordinary
// rather than awkward. The decoder above reads a whole stream into Go values
// and cannot say at which stage it stopped, so a document it refuses may have
// failed to parse, to compose, or to construct. The parser only parses, so its
// refusal is a statement about syntax and can be compared against a verdict
// that is also about syntax.
//
// Getting this wrong is not a small error and it does not announce itself. A
// mutant's acceptance is evidence at parsing and nowhere else; scored against
// the decoder it is either an accusation -- if the corpus claims construction
// it has no right to -- or nothing at all, if the corpus is honest and the
// consumer is the wrong one. Both were tried here before this table existed.
var GoYAMLParser = stance.Table{
	Name:     "go-openapi/go-yaml parser",
	Because:  "answers whether a document is well formed, and nothing about what it means",
	At:       stance.Parse,
	Speaks:   Vocabulary(),
	Requires: allRules(),
	Stands:   GoYAML.Stands,
}
