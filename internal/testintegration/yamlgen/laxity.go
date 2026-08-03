// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"unicode/utf8"
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
		Name: "a-character-the-spec-forbids",
		Src:  "\x00\n",
		Rule: "c-printable, which admits x09, x0A, x0D and x20-x7E and no other " +
			"character below xA0",
		Reads: "\x00",
		Match: func(src string) bool {
			if !utf8.ValidString(src) {
				return false
			}

			for _, r := range src {
				if !printable(r) {
					return true
				}
			}

			return false
		},
	},
	{
		Name: "bytes-that-are-not-text",
		Src:  "\xbf\n",
		Rule: "a YAML stream is Unicode; xBF is a continuation byte with nothing " +
			"to continue, so it is not a character and c-printable cannot admit " +
			"it. It is read as U+FFFD, so the byte is gone and nothing said so",
		Reads: "�",
		Match: func(src string) bool { return !utf8.ValidString(src) },
	},
}

// printable is YAML 1.2's c-printable, which is the set of characters a stream
// may contain at all.
func printable(r rune) bool {
	switch {
	case r == 0x09 || r == 0x0A || r == 0x0D:
		return true
	case r >= 0x20 && r <= 0x7E:
		return true
	case r == 0x85:
		return true
	case r >= 0xA0 && r <= 0xD7FF:
		return true
	case r >= 0xE000 && r <= 0xFFFD:
		return true
	default:
		return r >= 0x10000 && r <= 0x10FFFF
	}
}
