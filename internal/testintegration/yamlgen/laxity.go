// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

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
//
// The three entries it held before 2026-09-11 were refused once the byte order
// mark was held to the document prefixes it may open, the '?' was read as the
// explicit key indicator wherever separation follows it, and an entry's value
// was measured against the ':' of a key that was never written. Each left a
// test in parser/ or scanner/ behind it.
var Lax = []Laxity{
	{
		Name: "a block sequence on the same line as its tag",
		Src:  "!foo - 1\n",
		Rule: "8.2.1: s-l+block-collection puts s-l-comments between a node's properties and the " +
			"collection under them, and s-l-comments requires a line break. So a block sequence " +
			"cannot begin on the line its tag was written on. `!foo` over `- 1` on the next line " +
			"is the same document written correctly, and reads the same.",
		Reads: []any{uint64(1)},
	},
	{
		Name: "an explicit key's value in the key's own column",
		Src:  " ?\n 1\n",
		Rule: "8.2.2: l-block-map-explicit-value(n) is `s-indent(n) \":\" s-l+block-indented(n,block-out)`, " +
			"so an explicit entry takes a value only from a line that opens with ':'. Line 2 opens with " +
			"`1`, which makes it a fresh ns-l-block-map-entry needing a ':' of its own -- go.yaml.in/yaml/v3 " +
			"refuses it in those words, `could not find expected ':'`. Nor is the `1` the key's own content: " +
			"c-l-block-map-explicit-key(n) puts that at s-l+block-indented(n), further in than the '?', and " +
			"both sit in column 2.",
		// We read the "1" as the explicit entry's value; libfyaml 1.0.0b1 reads
		// it as the next entry's key and gives {"1": null}. Two lax readers,
		// opposite answers, which is the argument for refusing rather than
		// guessing: there is no obvious thing to read this as. libfyaml's is
		// the more defensible of the two, since a line at the mapping's own
		// indent starts an entry rather than continuing one.
		Reads: map[string]any{"null": uint64(1)},
	},
	{
		Name: "an explicit key's ':' indented past the mapping",
		Src:  "? l\n :\n",
		Rule: "8.2.2: l-block-map-explicit-value(n) opens with s-indent(n), and n here is 0 -- the '?' " +
			"stands in column 1. The ':' is in column 2, so it is not the entry's value line. Nor is it " +
			"content of the key `l`, which sits in column 3, further in than the ':'. libfyaml 1.0.0b1 " +
			"names it exactly: `invalid indentation in mapping` at 2:2, and go.yaml.in/yaml/v3 refuses " +
			"it too.",
		// The pair with the entry above: there the value line carries no ':'
		// and we read the scalar on it, here the ':' is in the wrong column and
		// we read it anyway. Both say the same thing about the library -- the
		// ':' of an explicit entry is never measured against the mapping's own
		// indent -- so a fix for one is likely a fix for both.
		//
		// Kept as two entries with exact Src rather than one Match over the
		// class, because a predicate wide enough to hold both would be wide
		// enough to swallow the next finding, and this list is only worth
		// having while everything in it has been looked at.
		Reads: map[string]any{"l": nil},
	},
	{
		Name: "a byte order mark behind a tab",
		Src:  "\t\ufeff\n",
		Rule: "5.2: l-document-prefix is `c-byte-order-mark? l-comment*`, so the mark stands before " +
			"anything else a document may open with -- a tab in front of it puts it outside the prefix. " +
			"It is not comment text either, which l-comment requires to open with '#'. " +
			"go.yaml.in/yaml/v3 refuses it as `found character that cannot start any token`.",
		// The one entry here that swallows a character. libfyaml 1.0.0b1 reads
		// the document as the string "\ufeff" -- the mark as content, which is
		// the reading that follows from it not being a prefix. We report no
		// error and no content at all, so a caller cannot tell this document
		// from an empty one. Of the three answers ours is the only one that
		// loses the byte.
		Reads: nil,
	},
}
