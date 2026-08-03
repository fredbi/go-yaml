// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"regexp"
	"strings"
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
	{
		Name:  "a-tab-where-indentation-belongs",
		Src:   "\t\"\": a\n",
		Rule:  "s-indent(n) ::= s-space x n, so indentation is spaces and a tab is not one",
		Reads: map[string]any{"": "a"},
		Match: tabInIndentation,
	},
	{
		Name:  "an-anchor-with-no-name",
		Src:   "& e\n",
		Rule:  "c-ns-anchor-property ::= \"&\" ns-anchor-name, and ns-anchor-name is one character or more",
		Reads: nil,
		// ns-anchor-char is ns-char less the flow indicators, so a name is
		// ended -- and with nothing in it, missing -- by whitespace, a break,
		// end of input, or one of , [ ] { }.
		Match: func(src string) bool {
			for i := range len(src) {
				if src[i] != '&' && src[i] != '*' {
					continue
				}
				if i+1 == len(src) || strings.IndexByte(" \t\r\n,[]{}", src[i+1]) >= 0 {
					return true
				}
			}

			return false
		},
	},
	{
		Name:  "a-plain-scalar-opening-on-an-indicator",
		Src:   "}\n",
		Rule:  "ns-plain-first(c) excludes c-indicator, of which } is one",
		Reads: "}",
		// A whole document that is one indicator and a line break. Narrow
		// enough to be exact: the same character with anything after it is a
		// different question, and most of the set is refused anyway.
		Match: func(src string) bool {
			return len(src) == 2 && src[1] == '\n' && strings.IndexByte(indicators, src[0]) >= 0
		},
	},
	{
		Name: "a-node-touching-the-anchor-in-front-of-it",
		Src:  "&a[]\n",
		Rule: "c-flow-indicator ends ns-anchor-name, and what follows a " +
			"c-ns-properties has to be separated from it by s-separate. \"&a []\" " +
			"is valid and reads as the empty sequence; without the space the " +
			"sequence is read as nothing at all, so this one loses a value rather " +
			"than merely admitting a document",
		Reads: nil,
		Match: anchorTouchingAFlowIndicator,
	},
	{
		Name: "two-chomping-indicators",
		Src:  "|--\n",
		Rule: "c-b-block-header(m,t) takes one indentation indicator and one " +
			"chomping indicator, in either order, and not two of either",
		Reads: "",
		Match: twoChomping.MatchString,
	},
	{
		Name: "a-comment-with-nothing-in-front-of-it",
		Src:  "|-#\n",
		Rule: "s-b-comment requires s-separate-in-line before c-nb-comment-text. " +
			"A bare | or > is refused, so it is the indicator that lets the " +
			"comment run into the header",
		Reads: "",
	},
}

// indicators is YAML 1.2's c-indicator: the characters a plain scalar may not
// begin with.
const indicators = "-?:,[]{}#&*!|>'\"%@`"

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

// twoChomping matches a block scalar header carrying more than one chomping
// indicator. Anchored to the whole document because a longer one is refused,
// and a predicate wider than the finding would be claiming more than was seen.
var twoChomping = regexp.MustCompile(`\A[|>][-+]{2,}\n\z`)

// tabInIndentation reports whether any line's leading whitespace holds a tab.
func tabInIndentation(src string) bool {
	for line := range strings.SplitSeq(src, "\n") {
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if strings.ContainsRune(indent, '\t') {
			return true
		}
	}

	return false
}

// anchorTouchingAFlowIndicator reports whether an anchor or alias name runs
// straight into one of the characters that ends it.
func anchorTouchingAFlowIndicator(src string) bool {
	for i := range len(src) {
		if src[i] != '&' && src[i] != '*' {
			continue
		}

		j := i + 1
		for j < len(src) && strings.IndexByte(" \t\r\n,[]{}", src[j]) < 0 {
			j++
		}

		if j > i+1 && j < len(src) && strings.IndexByte(",[]{}", src[j]) >= 0 {
			return true
		}
	}

	return false
}
