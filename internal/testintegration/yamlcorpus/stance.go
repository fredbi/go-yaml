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

		// Changed 2026-08-27, and it is the entry this table exists for.
		//
		// A cycle used to be read without complaint and come back with nil
		// where the cycle was, which is neither holding the cycle nor refusing
		// it. The decoder now refuses: an anchor is entered in its books only
		// once the node it names is resolved, so an alias standing inside that
		// node has nothing to resolve to and says so.
		//
		// Declarable here rather than a defect, because whether a cycle can be
		// held is the consumer's position and not the language's -- see
		// TagCyclicMeaning. The parse is unaffected and still accepts the
		// document, which is what TagAliasRecursive requires; GoYAMLParser
		// below is the table that scores it.
		//
		// libfyaml 1.0.0b1 and go.yaml.in/yaml/v3 both refuse these documents
		// too. PyYAML 6.0.1 accepts and builds the cycle.
		TagCyclicMeaning: stance.Refuses,

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

		// Merge keys, measured: this library implements the YAML 1.1 merge in
		// full. It merges, a local key wins over a merged one, a sequence
		// merges in order, and quoting suppresses the whole thing.
		//
		// The refusal is the interesting entry. Merging obliges a parser to
		// reject "<<: 1", because there is no operation that merges a scalar --
		// so implementing an extension costs documents that a parser without it
		// reads happily. libfyaml, which implements no merge, accepts them.
		// Both are conformant and the corpus scores both.
		TagMergeKey:        stance.Accepts,
		TagMergeSequence:   stance.Accepts,
		TagMergeNonMapping: stance.Refuses,
		TagMergeQuoted:     stance.Accepts,

		// Directives, measured, and one of them is a defect rather than a
		// position. See Departures: this library reads a document with one
		// directive and refuses a document with two, so a %YAML beside a %TAG
		// -- the commonest prelude YAML has -- is a document it cannot read.
		//
		// The minor-version entry is a genuine position. The spec only *should*
		// have a processor accept a version beyond its own, so refusing 1.9 is
		// a choice, and libfyaml makes the same one.
		TagYAMLMinorVersion: stance.Refuses,
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
// None of them appears anywhere in the four hundred documents of the YAML Test
// Suite. That is the argument for the patterns in one sentence.
//
// Two entries left on 2026-08-27, both fixed rather than argued away: a cycle
// decoding to nil, and an alias resolving to an earlier document's anchor. The
// third arrived on 2026-09-03 from the generator rather than from a pattern:
// Style.FlowEmpty started writing "{a}" and the duplicate-key check turned out
// not to see it.
var Departures = []Departure{
	{
		Pattern:  "a version directive and a tag directive together",
		Kind:     Verdict,
		Observed: "a document carrying more than one directive is refused: unexpected directive value",
		Because: "6.8: nothing limits a document to one directive, and a %YAML beside a %TAG is the ordinary " +
			"prelude -- so this is not an exotic shape but the commonest one there is",
		Corroborated: "libfyaml 1.0.0a8 reads it, and reads two %TAG handles together as well",
	},
	{
		Pattern:  "the same key twice, one of them written as a key alone",
		Kind:     Verdict,
		Observed: `"{a, a: 1}" and "{a, a}" are read; "{a: 1, a: 2}" and "{a: , a: 1}" are refused`,
		Because: "3.2.1.1: a flow mapping entry may be a key with no value, and it is an entry like any " +
			"other -- so its key counts when the mapping is checked for duplicates",
		Corroborated: "",
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
