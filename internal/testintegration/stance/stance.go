// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package stance records what a parser has decided about the things its
// specification leaves open.
//
// # Why a corpus cannot store a verdict
//
// Every specification worth testing against leaves some questions to the
// implementation. RFC 8259 lets a parser limit the range and precision of
// numbers, and calls the meaning of an unpaired surrogate escape
// unpredictable. YAML 1.2 admits three encodings where a given library may
// want one. Neither is a defect in the specification and neither is settled by
// a grammar.
//
// So a corpus that stored "this document must be accepted" would be storing
// one parser's opinion as though it were the language, and would mark a
// perfectly conforming parser wrong for holding a different one.
//
// What a corpus stores instead is the grammar's verdict and a set of [Tag]s
// naming the implementation-defined properties the document exhibits. A [Table]
// turns those into an expectation at the moment of replay. One corpus then
// scores our parser, our parser under different options, and somebody else's
// parser, without any of them being privileged.
//
// # Nothing here is language-specific
//
// The tag vocabulary belongs to the language being tested. The machinery for
// declaring a position and deriving an expectation does not, and lives here so
// that JSON and YAML share it.
package stance

import (
	"fmt"
	"slices"
	"strings"
)

// Tag names one implementation-defined property that a document exhibits.
//
// Tags are namespaced by the kind of question they raise -- "encoding/not-utf8"
// asks what the bytes denote, "number/out-of-range" asks what a value means --
// because the two sit on opposite sides of the grammar and a parser may well
// take opposite positions on them.
type Tag string

// Namespace is the part of a tag before the slash, which says which side of
// the grammar the question falls on.
func (t Tag) Namespace() string {
	before, _, found := strings.Cut(string(t), "/")
	if !found {
		return ""
	}

	return before
}

// Stand is a position a parser takes on one tag.
type Stand uint8

const (
	// Undeclared is the zero value: this parser has said nothing about this
	// property. It is not the same as Either -- silence is a gap in the
	// declaration, and every document carrying such a tag goes unscored and
	// gets reported.
	Undeclared Stand = iota
	// Accepts says the parser reads documents with this property.
	Accepts
	// Refuses says the parser rejects them, deliberately.
	Refuses
	// Either says the parser declines to guarantee, so a document carrying
	// this property is not evidence about it either way.
	Either
)

func (s Stand) String() string {
	switch s {
	case Accepts:
		return "accepts"
	case Refuses:
		return "refuses"
	case Either:
		return "either"
	case Undeclared:
		return "undeclared"
	default:
		return fmt.Sprintf("Stand(%d)", uint8(s))
	}
}

// Table is one parser's position on every property, under one configuration.
//
// A parser with a strict mode and a lenient mode is two Tables over one corpus
// rather than two corpora, which is most of the point.
type Table struct {
	// Name identifies the parser and the configuration, and appears in
	// reports: "go-openapi/json default" rather than "go-openapi/json".
	Name string
	// Because records why the positions are what they are, for the reader who
	// finds a refusal surprising.
	Because string
	// Stands is the position taken on each tag. A tag absent from the map is
	// Undeclared.
	Stands map[Tag]Stand
}

// Stand returns the position taken on a tag.
func (t Table) Stand(tag Tag) Stand { return t.Stands[tag] }

// Outcome is what a parser is expected to do with a document.
type Outcome uint8

const (
	// Undecided means this document is not evidence about this parser: some
	// property it carries is one the parser has not ruled on, or has ruled on
	// in a way the oracle cannot follow.
	Undecided Outcome = iota
	// Accept means a conforming parser under this stance reads the document.
	Accept
	// Reject means it refuses.
	Reject
)

func (o Outcome) String() string {
	switch o {
	case Accept:
		return "accept"
	case Reject:
		return "reject"
	case Undecided:
		return "undecided"
	default:
		return fmt.Sprintf("Outcome(%d)", uint8(o))
	}
}

// Doc is one corpus document: the bytes, what the grammar said, and what
// implementation-defined properties it carries.
type Doc struct {
	// Name identifies the document.
	Name string
	// Src is the document as it is stored, before any normalization.
	Src []byte
	// WellFormed is the grammar's verdict on the document *after* the
	// losslessly normalizable properties have been normalized away -- for JSON,
	// after a leading byte order mark is removed.
	//
	// Normalizing first is what keeps a tolerated property from costing a
	// scoreable case. A parser that skips a byte order mark is reading the
	// bytes after it, so that is what the oracle should have judged; leaving
	// the mark in would make every such document undecidable for that parser,
	// which is a loss of coverage dressed up as caution.
	WellFormed bool
	// Opaque says the oracle could not read these bytes under any policy it
	// knows -- they are not valid UTF-8 even after normalization, so what they
	// denote depends on a decoder this package does not have.
	//
	// It is the honest limit of the mechanism: a parser lenient about encodings
	// needs its input decoded before a grammar can say anything, and pretending
	// otherwise would put a guess in the corpus.
	Opaque bool
	// Tags are the implementation-defined properties the raw bytes exhibit.
	Tags []Tag
}

// Expect says what a parser holding this stance should do with a document, and
// why.
//
// The reasoning, in the order it is applied:
//
//   - A property the parser refuses forces a rejection, whatever else is true.
//     Refusing on purpose is conformant.
//   - A property the parser has not ruled on, or declines to guarantee, makes
//     the document no evidence at all. Silence is reported rather than read as
//     consent.
//   - Bytes the oracle cannot read at all, carrying a property this parser
//     tolerates, are no evidence either: what they denote depends on a decoder
//     this package does not have. A parser lenient about encodings needs its
//     input decoded before a grammar can say anything about it.
//   - Otherwise the grammar decides, on the normalized document.
func (t Table) Expect(d Doc) (Outcome, string) {
	for _, tag := range d.Tags {
		switch t.Stand(tag) {
		case Refuses:
			return Reject, string(tag) + ": " + t.Name + " refuses this on purpose"
		case Undeclared:
			return Undecided, string(tag) + ": " + t.Name + " has not ruled on this"
		case Either:
			return Undecided, string(tag) + ": " + t.Name + " declines to guarantee this"
		case Accepts:
			if d.Opaque {
				return Undecided, string(tag) + ": " + t.Name + " tolerates this, and nothing here " +
					"can say what the bytes denote without the decoder that would read them"
			}
		}
	}

	if d.WellFormed {
		return Accept, "the grammar accepts it and no property this parser refuses is present"
	}

	return Reject, "the grammar refuses it, and no implementation-defined property explains why"
}

// Undeclared lists the tags appearing in docs that this table says nothing
// about, sorted, so a suite can report the gaps in its own declaration rather
// than quietly skipping them.
func (t Table) Undeclared(docs []Doc) []Tag {
	var missing []Tag

	for _, d := range docs {
		for _, tag := range d.Tags {
			if t.Stand(tag) == Undeclared && !slices.Contains(missing, tag) {
				missing = append(missing, tag)
			}
		}
	}

	slices.Sort(missing)

	return missing
}
