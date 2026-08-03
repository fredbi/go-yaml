// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"regexp"
	"strings"
)

// Laxity is a document YAML 1.2 refuses that this library reads anyway.
//
// This is the other direction from [Divergence], and it is recorded differently
// because it is found differently. A Divergence names a shape, since every run
// draws different documents and there is nothing to point at. A Laxity names a
// document: the mutants that survive the recognizer are few, so each one can be
// reduced, checked and pinned.
type Laxity struct {
	// Name is short and stable, so a count can be reported against it.
	Name string
	// Src is the document, reduced to the smallest one that still shows it.
	Src string
	// Rule is the production it breaks.
	//
	// Named so the claim can be checked against the spec rather than against
	// the recognizer. That matters more here than anywhere else in this
	// package: a finding in [Ledger] needs the grammar's acceptance to be
	// right, where every one of these needs its refusal to be.
	Rule string
	// Reads is what the library makes of the document.
	//
	// Pinned rather than described, because it is what separates the two
	// severities. Reading an invalid document as the obvious thing is a
	// permissive extension and mostly harmless. Reading it as something else is
	// worse than refusing it, since nothing downstream is in a position to
	// notice -- which is what the entries below that swallow a byte or a value
	// do.
	Reads any
	// Match recognizes other documents of the same class, so that the hunt
	// stops re-reporting one it is standing on. Nil means only Src itself.
	//
	// Left nil unless the class is one a predicate can state exactly. A loose
	// one here is worse than a noisy hunt: it would absorb the next finding
	// silently, and the whole value of this list is that what is in it has been
	// looked at.
	Match func(src string) bool
}

// Covers reports whether src is an instance of this entry.
func (l Laxity) Covers(src string) bool {
	if l.Match != nil {
		return l.Match(src)
	}

	return l.Src == src
}

// KnownlyAccepted returns the entry covering src, or nil.
func KnownlyAccepted(src string) *Laxity {
	for i := range Lax {
		if Lax[i].Covers(src) {
			return &Lax[i]
		}
	}

	return nil
}

// Lax records every document known to be accepted against the grammar.
//
// Each was found by mutating a generated document, keeping what the recognizer
// refused, and asking the library anyway. Each was then checked by hand against
// the production named in Rule, because an entry here is a claim that YAML 1.2
// forbids something, and the recognizer is not evidence for its own verdict.
//
// The list is not a survey. It is what a few hundred thousand mutations turned
// up and a person then confirmed, so absence from it means nothing.
var Lax = []Laxity{
	{
		Name: "a-lone-question-mark-where-only-a-flow-node-fits",
		Src:  "k: ?\n",
		Rule: "ns-plain-first(c) admits '?' only when what follows it is " +
			"ns-plain-safe(c), and a line break is not one. Where a block node " +
			"may appear the '?' is read as c-l-block-map-explicit-key instead, " +
			"which is why \"- ?\", \"?\" and \"k:\\n  ?\" are all documents. A " +
			"value written on its key's line is an ns-flow-node and so is every " +
			"node inside a flow collection, and neither leaves the '?' anything " +
			"to be -- so \"[?]\" goes the same way. The distinction is the " +
			"context the node sits in rather than the characters around it, " +
			"which is why this is recorded rather than refused in the scanner",
		Reads: map[string]any{"k": "?"},
		Match: loneQuestionMark.MatchString,
	},
	{
		Name: "a-byte-order-mark-standing-as-content",
		Src:  "a: \ufeffb\n",
		Rule: "nb-char ::= c-printable - b-char - c-byte-order-mark, so a mark is " +
			"not a character any node may hold. It marks an l-document-prefix and " +
			"nothing else -- which is why one opening the stream is dropped rather " +
			"than refused, and why one anywhere a node may go is neither",
		Reads: map[string]any{"a": "\ufeffb"},
		// Anywhere past the prefix the stream may open with. A mark inside a
		// double-quoted scalar is left out on purpose: nb-double-char reaches
		// it through c-ns-esc-char and the recognizer accepts it, so it is a
		// different question from this one.
		Match: byteOrderMarkAsContent,
	},
	{
		Name: "a-value-level-with-an-empty-key",
		Src:  ":\n1\n",
		Rule: "s-l+block-node(n,c) puts the value of a block mapping entry at " +
			"s-indent(n+1), so a token level with the entry can only open the " +
			"next one -- and \"1\" opens nothing. The same document with a key " +
			"written out, \"k:\\n1\", is refused, and so is the same empty key " +
			"carrying a property, \": &a\\n1\". What is left is the entry whose " +
			"key is e-node and carries nothing: there is no key token to measure " +
			"the column against, and the ':' has not been made to stand for one",
		Reads: map[string]any{"null": uint64(1)},
		// A line that is nothing but ':' followed by one starting hard against
		// the left margin. Narrow on purpose: the indented spellings of the
		// same shape are a separate question, and one of them may well be a
		// document.
		Match: regexp.MustCompile("(?m)^:[ \t]*\n[^ \t\n]").MatchString,
	},
}

// byteOrderMarkAsContent reports whether src holds a byte order mark past the
// prefix the stream may open with, outside a double-quoted scalar.
func byteOrderMarkAsContent(src string) bool {
	const mark = "\ufeff"

	rest := strings.TrimLeft(src, mark)
	if !strings.Contains(rest, mark) {
		return false
	}

	// A line carrying a double quote is left to the entry that owns that
	// question rather than absorbed silently here.
	for line := range strings.SplitSeq(rest, "\n") {
		if strings.Contains(line, mark) && !strings.Contains(line, `"`) {
			return true
		}
	}

	return false
}

// loneQuestionMark matches a '?' with nothing after it standing where only a
// flow node fits: as a value on its key's line, or as an entry of a flow
// collection. Written out rather than described because the two are the whole
// of the class -- everywhere else the same '?' opens an explicit key.
var loneQuestionMark = regexp.MustCompile(`(?m): \?[ \t]*$|[\[{,][ \t]*\?[ \t]*[,\]}]`)
