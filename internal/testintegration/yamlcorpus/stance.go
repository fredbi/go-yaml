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
	Requires: AnchorRules(),
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
		Pattern:      "an alias to an anchor in an earlier document",
		Kind:         Verdict,
		Observed:     `the second document resolves *x to the first document's anchor, and no error is raised`,
		Because:      "3.2.2.2: anchor names are local to a document, so the alias names nothing and the stream is in error",
		Corroborated: "PyYAML 6.0.1 raises ComposerError, found undefined alias 'x'",
	},
	{
		Pattern:  "a sequence holding an alias to itself",
		Kind:     Value,
		Observed: "the document is read and the cycle decodes to nil, so &x [ *x ] becomes a one-element list holding nothing",
		Because: "3.2.1: the representation is a graph and the alias resolves to the node it is inside; " +
			"a model that cannot hold that has to say so rather than substitute a value the document never had",
	},
}
