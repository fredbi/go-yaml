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
// smoke tier's refused documents. The parser's whole vocabulary, and the part
// of it nothing reaches, are measured by TestTheParserVocabularyGapIsMeasured
// rather than counted by hand here
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
//
// So a refusal message must not contain an apostrophe. The quoted-run step
// reads it as opening a quoted span and eats everything to the next one: `a
// block scalar's content is not indented as far as its header's indentation
// indicator states` signs as `a block scalar_s indentation indicator states`.
// Single quotes in a message are placeholders for what the document
// contributed, and an apostrophe is an accidental one.
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
		// Eight complaints nothing in the corpus reached, provoked on purpose
		// on 2026-09-10. The parser can say ninety-odd things and the generated
		// documents draw sixty of them; these are the ones a document could be
		// written for. See TestTheParserVocabularyGapIsMeasured, which reports
		// what is left.
		{
			Name: "a byte order mark inside a line",
			Src:  "a: \ufeffb\n", Says: "byte order mark inside a line",
		},
		{
			Name: "a byte order mark where no document begins",
			Src:  "---\na: 1\n\ufeffb: 2\n", Says: "byte order mark where no document begins",
		},
		{
			Name: "a value after a document separator",
			Src:  "--- a\n--- b: 1\n", Says: "cannot be placed after document separator",
		},
		{
			// The escape is two hex digits and this has one, so the parser
			// reads the closing quote as the second and complains about it
			// rather than about the length.
			Name: "an eight-bit escape with a digit missing",
			Src:  `"\x1"` + "\n", Says: "is not a hexadecimal digit",
		},
		{
			Name: "a surrogate pair cut short",
			Src:  `"\uD800\uD"` + "\n", Says: "not enough length for escaped",
		},
		{
			Name: "a low surrogate whose digits are not hexadecimal",
			Src:  `"\uD800\uDCZZ"` + "\n", Says: "not a hexadecimal digit in the low surrogate",
		},
		{
			// A high surrogate has to be followed by a low one, and this is
			// followed by another character in the basic plane.
			//
			// WellFormed, and that is the point: pairing surrogates is a rule
			// about what the escapes denote, which no production expresses. The
			// message is the only statement anywhere that this library applies
			// it.
			Name: "a high surrogate followed by something that is not a low one",
			Src:  `"\uD800\uAAAA"` + "\n", Says: "after high surrogate",
			WellFormed: true,
		},
		{
			// Also well formed. The grammar takes a directive it does not
			// recognize as a reserved one and moves on, so the shape of a %TAG
			// is a rule stated in 6.8.2.2 and nowhere in the productions.
			Name: "a TAG directive with no prefix",
			Src:  "%TAG !e!\n---\nx\n", Says: "unexpected format TAG directive",
			WellFormed: true,
		},
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
		{
			// The three outside sources split on this one, and the split is
			// about layers rather than about the rule. The reference parser
			// refuses it and go.yaml.in/yaml/v3 v3.0.5 wants the ':';
			// libfyaml 1.0.0b1 reads {"a": null} and drops the anchor. The
			// recognizer refuses it, which is what this corpus goes by.
			Name: "an anchor standing alone where an entry should be",
			Src:  "a:\n&x\n", Says: "anchor is not allowed in this context",
		},
		{
			// Both halves of validateAnchorName's second case, which the
			// corpus reached only by luck and stopped reaching when the
			// chomping axis shifted the draws: an "&" or a "*" needs
			// s-separate between its name and whatever follows.
			Name: "an alias running straight into a flow collection",
			Src:  "a: *x{}\n", Says: "an alias must be separated from the node that follows it",
		},
		{
			Name: "an anchor running straight into a flow collection",
			Src:  "a: &x[]\n", Says: "an anchor must be separated from the node that follows it",
		},
		{
			// 6.9.2 gives a node one anchor, and the generator writes one per
			// node by construction, so nothing it draws reaches this.
			Name: "two anchors on one node",
			Src:  "a: &x\n  &y 1\n", Says: "anchors cannot be used consecutively",
		},
	}
}
