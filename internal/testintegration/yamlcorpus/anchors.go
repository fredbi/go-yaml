// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package yamlcorpus builds a YAML 1.2 conformance corpus.
//
// It is the YAML counterpart of the JSON spike, and it starts where the spike
// could not reach: YAML states a great deal outside its grammar, so a corpus
// that carried only the grammar's verdict would be silent about most of what a
// YAML parser can get wrong.
package yamlcorpus

import (
	"bytes"
	"fmt"

	"github.com/go-openapi/go-yaml/internal/testintegration/stance"
)

// Around is what a generated document contributes to the anchor and alias
// patterns.
//
// The patterns are built around real generated material rather than fixed
// literals, so that each one carries a document's shape and differs from it in
// exactly one reference. That is the whole reason they are patterns and not a
// list of hand-written fixtures: a hand-written fixture exercises the rule and
// nothing else, and the interesting failures are the ones where the rule
// interacts with what is around it.
//
// # Why entries and not a whole document
//
// A pattern has to put an anchor somewhere and an alias somewhere else, both at
// the top level of the same document. Splicing them into an arbitrary document
// would mean re-indenting it, and re-indenting YAML is exactly the operation
// this project refuses to guess at -- a block scalar's content, a stated
// indentation indicator and a compact sequence entry all mean something
// different afterwards. So a pattern composes sibling entries at column zero
// instead, where nothing needs moving, and Document supplies the rest.
type Around struct {
	// Document is a generated block mapping written at column zero, with no
	// directives and no markers, so that entries may be written beside it.
	Document []byte
	// Scalar is a generated scalar node, written on one line.
	Scalar []byte
	// Other is a second generated scalar, different from Scalar, for the
	// patterns that need to tell two anchorings apart.
	Other []byte
	// Collection is a generated flow collection, written on one line. Aliasing
	// one is a different question from aliasing a scalar, because what comes
	// back is shared rather than copied.
	Collection []byte
}

// Pattern is one arrangement of an anchor and an alias.
type Pattern struct {
	// Name says what the arrangement is.
	Name string
	// Because is what the specification says about it, and where.
	Because string
	// Exhibits is what the document carries, put on by construction rather than
	// found by inspection. Nothing can inspect for these: that is why they are
	// here.
	Exhibits []stance.Tag
	// Valid is whether a conforming parser reads the result.
	//
	// This is a label, not a measurement, and it is the point of the whole
	// file. The grammar accepts every one of these documents, the violations
	// included, because no production can consult a table of what it has
	// already seen. So the truth has to be carried by whatever built the
	// document -- see TestTheGrammarCannotSeeAnyOfThis.
	Valid bool
	// build writes the document.
	build func(a Around) []byte
}

// The rules YAML 1.2 states about anchors and aliases, none of which its
// grammar can express.
//
// Every one of them needs a table of anchors seen so far, which is not
// something a production has. The grammar recognizes "*x" wherever an alias may
// appear and has no opinion at all about whether x was ever anchored.
const (
	// TagAliasUndefined is an alias naming an anchor that never occurs.
	TagAliasUndefined stance.Tag = "anchor/alias-undefined"
	// TagAliasForward is an alias naming an anchor that occurs later.
	TagAliasForward stance.Tag = "anchor/alias-forward"
	// TagAliasAcrossDocuments is an alias naming an anchor from an earlier
	// document in the same stream.
	TagAliasAcrossDocuments stance.Tag = "anchor/alias-across-documents"
	// TagAnchorRedefined is the same anchor name given to two nodes. Legal, and
	// the later one wins.
	TagAnchorRedefined stance.Tag = "anchor/redefined"
	// TagAnchorUnused is an anchor nothing refers to. Legal, and worth having
	// because it is the shape a parser is most likely to have simply dropped.
	TagAnchorUnused stance.Tag = "anchor/unused"
	// TagAliasRecursive is an alias inside the node its own anchor names.
	TagAliasRecursive stance.Tag = "anchor/alias-recursive"
	// TagAliasAsKey is an alias used as a mapping key.
	TagAliasAsKey stance.Tag = "anchor/alias-as-key"
	// TagAnchorOnEmptyNode is an anchor on a node with no content.
	TagAnchorOnEmptyNode stance.Tag = "anchor/on-empty-node"
)

// AnchorRules is what the specification settles about the above.
//
// The three rejections are the ones that make the whole mechanism necessary. A
// parser accepting an undefined alias is wrong, not differently opinionated, so
// a stance must not be able to vote it into conformance -- see [stance.Rule].
//
// The recursive alias is deliberately not here. Whether a parser can represent
// a cycle is a property of its data model rather than of YAML, and the spec's
// representation is a graph, so that one is a genuine stance and belongs in a
// table.
func AnchorRules() stance.Rules {
	return stance.Rules{
		{
			Tag:     TagAliasUndefined,
			Because: "7.1: an alias refers to the most recent preceding node having the given anchor, and there is none",
			Then:    stance.Reject,
		},
		{
			Tag:     TagAliasForward,
			Because: "7.1: the anchor must precede the alias, so a forward reference names nothing",
			Then:    stance.Reject,
		},
		{
			Tag:     TagAliasAcrossDocuments,
			Because: "3.2.2.2: anchor names are local to a document, so an earlier document's anchor is out of scope",
			Then:    stance.Reject,
		},
		{
			Tag:     TagAnchorRedefined,
			Because: "3.2.2.2: an anchor name may be reused, and the alias takes the most recent",
			Then:    stance.Accept,
		},
		{
			Tag:     TagAnchorUnused,
			Because: "7.1: nothing requires an anchor to be referred to",
			Then:    stance.Accept,
		},
		{
			Tag:     TagAliasAsKey,
			Because: "7.1: an alias is a node, and a node may be a mapping key",
			Then:    stance.Accept,
		},
		{
			Tag:     TagAnchorOnEmptyNode,
			Because: "7.1: the empty node is a node, and may be anchored",
			Then:    stance.Accept,
		},
	}
}

// Patterns is the set of arrangements, in a fixed order.
//
// Small and completely enumerated, for the same reason the encoding shapes are:
// no coverage signal points at any of it. An anchor and an alias reach exactly
// the same productions whether or not the alias resolves, so a corpus selected
// on what the grammar saw would keep one of these and discard the rest as
// duplicates of it.
func Patterns() []Pattern {
	return []Pattern{
		{
			Name:     "an alias to an anchor that is never defined",
			Because:  "the alias names nothing",
			Exhibits: []stance.Tag{TagAliasUndefined},
			Valid:    false,
			build: func(a Around) []byte {
				return join(a.Document, entry("aliased", alias("nowhere")))
			},
		},
		{
			Name:     "an alias written before its anchor",
			Because:  "the anchor exists, and not yet",
			Exhibits: []stance.Tag{TagAliasForward},
			Valid:    false,
			build: func(a Around) []byte {
				return join(
					entry("aliased", alias("x")),
					a.Document,
					entry("anchored", anchor("x", a.Scalar)),
				)
			},
		},
		{
			Name:     "an alias to an anchor in an earlier document",
			Because:  "an anchor does not outlive the document that defined it",
			Exhibits: []stance.Tag{TagAliasAcrossDocuments},
			Valid:    false,
			build: func(a Around) []byte {
				return join(
					[]byte("---\n"),
					entry("anchored", anchor("x", a.Scalar)),
					a.Document,
					[]byte("---\n"),
					entry("aliased", alias("x")),
				)
			},
		},
		{
			Name:     "the same anchor name given to two nodes",
			Because:  "legal, and the alias takes the later one",
			Exhibits: []stance.Tag{TagAnchorRedefined},
			Valid:    true,
			build: func(a Around) []byte {
				return join(
					entry("first", anchor("x", a.Scalar)),
					a.Document,
					entry("second", anchor("x", a.Other)),
					entry("aliased", alias("x")),
				)
			},
		},
		{
			Name:     "an anchor nothing refers to",
			Because:  "legal, and the shape a parser is likeliest to have dropped",
			Exhibits: []stance.Tag{TagAnchorUnused},
			Valid:    true,
			build: func(a Around) []byte {
				return join(a.Document, entry("anchored", anchor("x", a.Scalar)))
			},
		},
		{
			Name:     "an alias inside the collection its own anchor names",
			Because:  "the representation is a graph, so a cycle is a node and not an error",
			Exhibits: []stance.Tag{TagAliasRecursive},
			Valid:    true,
			build: func(a Around) []byte {
				return join(a.Document, entry("recursive", []byte("&x [ *x ]")))
			},
		},
		{
			Name:     "an alias used as a mapping key",
			Because:  "an alias is a node, and a node may be a key",
			Exhibits: []stance.Tag{TagAliasAsKey},
			Valid:    true,
			build: func(a Around) []byte {
				return join(
					entry("anchored", anchor("x", a.Scalar)),
					a.Document,
					concat(alias("x"), []byte(" : keyed\n")),
				)
			},
		},
		{
			Name:     "an anchor on an empty node, then aliased",
			Because:  "the empty node is a node",
			Exhibits: []stance.Tag{TagAnchorOnEmptyNode},
			Valid:    true,
			build: func(a Around) []byte {
				return join(
					[]byte("anchored: &x\n"),
					a.Document,
					entry("aliased", alias("x")),
				)
			},
		},
		{
			Name:     "an unused anchor on a collection, aliased into a key",
			Because:  "aliasing a collection shares it rather than copying it, which is where a key stops being a scalar",
			Exhibits: []stance.Tag{TagAliasAsKey, TagAnchorRedefined},
			Valid:    true,
			build: func(a Around) []byte {
				return join(
					entry("first", anchor("x", a.Collection)),
					entry("second", anchor("x", a.Collection)),
					a.Document,
					concat(alias("x"), []byte(" : keyed\n")),
				)
			},
		},
	}
}

// Shapes builds every pattern around one generated document.
func Shapes(a Around) []stance.Shape {
	patterns := Patterns()
	out := make([]stance.Shape, 0, len(patterns))

	for _, p := range patterns {
		out = append(out, stance.Shape{
			Name:   p.Name,
			Src:    p.build(a),
			Intent: p.Exhibits,
		})
	}

	return out
}

// Valid reports what each pattern's document should be, by name, so that a
// builder can label a case without rebuilding it.
func Valid() map[string]bool {
	out := map[string]bool{}
	for _, p := range Patterns() {
		out[p.Name] = p.Valid
	}

	return out
}

func entry(key string, node []byte) []byte {
	return []byte(fmt.Sprintf("%s: %s\n", key, node))
}

func anchor(name string, node []byte) []byte {
	return concat([]byte("&"+name+" "), node)
}

func alias(name string) []byte { return []byte("*" + name) }

func concat(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

// join concatenates document fragments, making sure each ends its line, so that
// a generated document with or without a trailing newline composes the same
// way.
func join(parts ...[]byte) []byte {
	var out bytes.Buffer

	for _, part := range parts {
		out.Write(part)

		if len(part) > 0 && part[len(part)-1] != '\n' {
			out.WriteByte('\n')
		}
	}

	return out.Bytes()
}
