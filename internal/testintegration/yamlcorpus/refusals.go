// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus

import (
	"regexp"
	"strings"
)

// Comparing a refusal by what it says, not only by the fact of it.
//
// # Why the fact of a refusal is not enough
//
// Everything else in this package asks whether a document is refused. Six of
// the eight defects found across the parser rewrite lived in *why*: six
// documents that had been refused with `'@' is a reserved character` came back
// refused with `found an invalid token`, and no verdict anywhere changed. A
// corpus of verdicts is blind to that by construction, and so is the comparison
// against a second parser -- it ignores wording on purpose, because two parsers
// word things differently.
//
// # Two ways to hold a message, and what each is for
//
// [Refusals] pins a phrase to a document. It is hand-written and small, it
// needs no corpus, and the failure hands a fixer a document to paste. Use it
// for the complaints worth naming.
//
// [RefusalSignature] is the other half and the one that crosses with the rest
// of the generator. It reduces a message to the parser's own words -- position
// gone, quoted text gone, numbers gone -- so that the same complaint about
// different documents is one signature. Counting the distinct signatures a
// corpus provokes measures how much the parser can still tell you: 54 over the
// smoke tier's 3,908 refused documents, against 93 message templates in the
// parser and scanner sources. A parser that got vaguer would lose signatures
// while every verdict stayed exactly where it was.
//
// # What a signature deliberately throws away
//
// The wording. `sequence end token ']' not found` and `sequence end token '}'
// not found` are one signature, and rephrasing a message does not change its
// signature at all. That is the point: a message is a stronger claim than a
// verdict and drifts faster, so pinning whole messages would fail every time
// somebody improved one. What survives is how many different things the parser
// can say, and a collapse in that number is the regression worth catching.

// RefusalSignature reduces an error to the parser's own words.
//
// The substitutions run in order and each one removes something the document
// contributed rather than the parser: the position, then any quoted run, then a
// tag handle, then a number, then the text after a colon, then anything glued
// to a substitution that the earlier steps could not reach -- a mapping key
// holding a quote of its own, which is what `mapping key "a"b" already defined`
// leaves behind.
func RefusalSignature(err error) string {
	if err == nil {
		return ""
	}

	msg, _, _ := strings.Cut(err.Error(), "\n")

	msg = refusalPosition.ReplaceAllString(msg, "")
	msg = refusalSingle.ReplaceAllString(msg, "_")
	msg = refusalDouble.ReplaceAllString(msg, "_")
	msg = refusalHandle.ReplaceAllString(msg, "_")
	msg = refusalNumber.ReplaceAllString(msg, "N")

	// The parser's phrasing comes before the colon and the document's text
	// after it, so the colon is where a message stops being about the parser.
	if head, _, found := strings.Cut(msg, ": "); found {
		msg = head
	}

	msg = refusalJunk.ReplaceAllString(msg, "")
	msg = refusalGlued.ReplaceAllString(msg, "_")

	return strings.TrimSpace(refusalSpace.ReplaceAllString(msg, " "))
}

var (
	refusalPosition = regexp.MustCompile(`\[\d+:\d+\]`)
	refusalSingle   = regexp.MustCompile(`'(?:[^']|\\.)*'`)
	refusalDouble   = regexp.MustCompile(`"(?:[^"]|\\.)*"`)
	refusalHandle   = regexp.MustCompile(`![^\s!]*!`)
	refusalNumber   = regexp.MustCompile(`\b\d+\b`)
	refusalJunk     = regexp.MustCompile("[\"'`]|[^\\x20-\\x7e]")
	refusalGlued    = regexp.MustCompile(`\S*_\S*`)
	refusalSpace    = regexp.MustCompile(`\s+`)
)

// Refusal is a document this library must refuse, and the phrase that says why.
type Refusal struct {
	// Name says what the document does wrong.
	Name string
	// Src is the document.
	Src string
	// Says is the part of the message that carries the reason.
	//
	// A substring rather than the whole message, and rather than a
	// [RefusalSignature]. The words around it get rephrased and a signature
	// throws them away, so both of those would let `'@' is a reserved
	// character` decay into `found an invalid token` unnoticed. This is the
	// noun the message would lose.
	Says string
	// WellFormed says whether YAML 1.2 accepts the document.
	//
	// Where it does, the refusal is this library applying a rule the grammar
	// cannot express -- an undeclared tag handle, a version it does not
	// implement -- and the message is the only place that rule is stated.
	WellFormed bool
}

// Refusals are the complaints worth naming.
//
// Chosen for what a vaguer parser would lose rather than for coverage: each one
// names a character, a construct or a rule that a generic `invalid token` would
// throw away. [TestTheCorpusDrawsEveryComplaint] is the wider net.
func Refusals() []Refusal {
	return []Refusal{
		{
			Name: "a reserved character opening a document",
			Src:  "@ x\n", Says: "is a reserved character",
		},
		{
			Name: "a reserved character opening a value",
			Src:  "k: @v\n", Says: "is a reserved character",
		},
		{
			// '@' and '`' are the two, and a parser that named only one would
			// still pass the entry above.
			Name: "the other reserved character",
			Src:  "` x\n", Says: "is a reserved character",
		},
		{
			Name: "an anchor with no name",
			Src:  "& x\n", Says: "anchor must be followed by a name",
		},
		{
			Name: "an alias with no name",
			Src:  "* x\n", Says: "alias must be followed by a name",
		},
		{
			Name: "a tag handle no directive declared",
			Src:  "!n!t x\n", Says: "is not defined by a TAG directive", WellFormed: true,
		},
		{
			Name: "a version this processor does not implement",
			Src:  "%YAML 1.9\n---\nx\n", Says: "unknown YAML version", WellFormed: true,
		},
		{
			Name: "a tab where a token should start",
			Src:  "- a\n\tb: 1\n", Says: "cannot start any token",
		},
		{
			Name: "a tab where a block scalar wants indentation",
			Src:  "a: |\n\tx\n", Says: "tab character",
		},
		{
			Name: "a flow sequence that is never closed",
			Src:  "a: [1\n", Says: "sequence end token",
		},
		{
			Name: "a flow mapping that is never closed",
			Src:  "a: {b: 1\n", Says: "flow mapping end token",
		},
		{
			Name: "a character a YAML stream may not hold",
			Src:  "\x00\n", Says: "that a YAML stream may not hold",
		},
		{
			Name: "an escape the specification does not define",
			Src:  "a: \"\\q\"\n", Says: "unknown escape character",
		},
		{
			Name: "a non-hexadecimal digit in a UTF-16 escape",
			Src:  "a: \"\\u00zz\"\n", Says: "hexadecimal digit",
		},
		{
			Name: "a plain scalar opening with a flow indicator",
			Src:  "a: }\n", Says: "plain scalar cannot begin with",
		},
	}
}
