// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus

import "github.com/go-openapi/go-yaml/internal/testintegration/stance"

// Node properties: tags, and the handles that abbreviate them.
//
// # Why this is a family and not a style
//
// A tag is not presentation. "!!str 1" and "1" are different documents that
// denote different things, so a tag cannot be an axis of how a value is written
// -- it is part of what the value is. That is also why the generator does not
// write them: yamlgen emits a value in many styles and asks that they all read
// back the same, and a tag would break the premise rather than exercise it.
//
// So tags are enumerated, like the anchors and the schema spellings, and for
// the same reason: the questions they raise are ones a grammar cannot answer
// and a generator cannot stumble into.
//
// # What the grammar can and cannot say here
//
// It can say a great deal about the *syntax* of a tag: the handles, the URI
// characters, the percent escapes. That is nine productions the corpus reached
// through nothing else, which is what brought this family forward.
//
// It cannot say whether a handle was ever declared. "!e!x" is a well-formed
// shorthand whatever %TAG directives precede it, so an undeclared handle is a
// document the grammar accepts and the specification calls an error -- the same
// shape as an undefined alias, and it gets a rule for the same reason.
const (
	// TagSecondary is a "!!" shorthand, which resolves through the secondary
	// handle to tag:yaml.org,2002: unless a %TAG says otherwise.
	TagSecondary stance.Tag = "tag/secondary"
	// TagVerbatim is a tag written out in full between angle brackets, which
	// resolves through nothing at all.
	TagVerbatim stance.Tag = "tag/verbatim"
	// TagLocal is a "!" shorthand, whose meaning is the application's.
	TagLocal stance.Tag = "tag/local"
	// TagNamedHandle is a shorthand using a handle a %TAG directive declared.
	TagNamedHandle stance.Tag = "tag/named-handle"
	// TagUndeclaredHandle is a shorthand whose handle no %TAG declares.
	//
	// Well formed, and an error: the shorthand cannot be resolved, and a
	// resolution is not something a production can attempt.
	TagUndeclaredHandle stance.Tag = "tag/undeclared-handle"
	// TagPercentEscape is a tag URI carrying a percent escape, which is the
	// only route to a hex digit outside a quoted scalar.
	TagPercentEscape stance.Tag = "tag/percent-escape"
	// TagNonSpecific is a bare "!", which suppresses resolution rather than
	// naming a type.
	TagNonSpecific stance.Tag = "tag/non-specific"
	// TagYAMLDirective is a %YAML directive stating the version.
	TagYAMLDirective stance.Tag = "directive/yaml-version"
)

// TagRules is what the specification settles about the above.
//
// One rejection, and it is the same shape as the alias rules: resolving a
// shorthand needs the table of handles the document declared, and a grammar has
// no table. Everything else is a construct the language allows and says nothing
// more about, so the meaning is the application's and the corpus keeps out of
// it.
func TagRules() stance.Rules {
	return stance.Rules{
		{
			Tag:     TagUndeclaredHandle,
			Because: "6.8.2.2: a tag shorthand must use a handle a %TAG directive declared, and this one does not",
			Then:    stance.Reject,
		},
		{
			Tag:     TagSecondary,
			Because: "6.8.2.2: the secondary handle is declared by default as tag:yaml.org,2002:",
			Then:    stance.Accept,
		},
		{
			Tag:     TagVerbatim,
			Because: "6.8.2.1: a verbatim tag is used as presented and resolves through nothing",
			Then:    stance.Accept,
		},
		{
			Tag:     TagNonSpecific,
			Because: "6.9.1: a non-specific tag is legal, and asks that resolution be suppressed",
			Then:    stance.Accept,
		},
	}
}

// TagVocabulary places the tag questions.
//
// The syntax of a tag is settled while parsing; whether a handle resolves is a
// composing question, since it needs the directives of the document the
// shorthand appears in; and what a resolved tag *means* is the application's,
// which is construction.
func TagVocabulary() stance.Vocabulary {
	return stance.Vocabulary{
		TagYAMLDirective:    stance.Parse,
		TagPercentEscape:    stance.Parse,
		TagUndeclaredHandle: stance.Compose,
		TagNamedHandle:      stance.Compose,
		TagSecondary:        stance.Compose,
		TagVerbatim:         stance.Compose,
		TagNonSpecific:      stance.Compose,
		TagLocal:            stance.Construct,
	}
}

// TagShapes are the documents.
//
// Written out rather than composed around a generated document, unlike the
// anchor patterns. A tag attaches to one node and says what that node is, so
// there is nothing for the surrounding document to interact with -- where an
// anchor's whole interest is what it refers to and from where.
func TagShapes() []stance.Shape {
	return []stance.Shape{
		{
			Name:   "a secondary tag shorthand",
			Src:    []byte("k: !!str 1\n"),
			Intent: []stance.Tag{TagSecondary},
		},
		{
			Name:   "a verbatim tag",
			Src:    []byte("k: !<tag:example.com,2011:x> v\n"),
			Intent: []stance.Tag{TagVerbatim},
		},
		{
			Name:   "a local tag",
			Src:    []byte("k: !local v\n"),
			Intent: []stance.Tag{TagLocal},
		},
		{
			Name:   "a handle declared by a directive",
			Src:    []byte("%TAG !e! tag:example.com,2011:\n---\nk: !e!x v\n"),
			Intent: []stance.Tag{TagNamedHandle},
		},
		{
			Name:   "a handle no directive declares",
			Src:    []byte("k: !e!x v\n"),
			Intent: []stance.Tag{TagUndeclaredHandle},
		},
		{
			Name:   "a percent escape in a tag URI",
			Src:    []byte("k: !<tag:%41> v\n"),
			Intent: []stance.Tag{TagVerbatim, TagPercentEscape},
		},
		{
			// At the root rather than as a value, because a hex digit is
			// reached in block-in only here -- a mapping value is block-out.
			Name:   "a percent escape in a tag on the root node",
			Src:    []byte("!<tag:%41> v\n"),
			Intent: []stance.Tag{TagVerbatim, TagPercentEscape},
		},
		{
			Name:   "a non-specific tag",
			Src:    []byte("k: ! v\n"),
			Intent: []stance.Tag{TagNonSpecific},
		},
		{
			Name:   "a version directive",
			Src:    []byte("%YAML 1.2\n---\nk: v\n"),
			Intent: []stance.Tag{TagYAMLDirective},
		},
	}
}
