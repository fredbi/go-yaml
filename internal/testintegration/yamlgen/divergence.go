// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"regexp"
	"strings"
)

// Divergence is a shape of document where this library disagrees with YAML 1.2.
//
// The generator keeps producing these rather than steering around them. Steering
// around a defect makes the harness quieter and blinder: the shape stops being
// generated, and nobody notices when it is fixed or when it spreads.
//
// Match describes the shape rather than naming a document, because there is no
// document to name -- every run draws different ones.
type Divergence struct {
	// Name is short and stable, so a count can be reported against it.
	Name string
	// Reason says what the library does and what YAML 1.2 says instead.
	Reason string
	// Property is which question this shape fails to answer. A shape that
	// reads back correctly but renders wrongly excuses one property and not
	// the other, and conflating them would let a decode defect hide behind a
	// render defect.
	Property Property
	// Match reports whether this pairing has the shape.
	Match func(Value, Style) bool
}

// Property names the questions the generated documents are put to. It is a set,
// because one root cause can fail more than one: a value that is already wrong
// when read is still wrong after being written out again.
type Property int

const (
	// Decode: reading the emitted document gives back the value.
	Decode Property = 1 << iota
	// Render: parsing the emitted document and writing it out again gives a
	// document that still means the same thing.
	Render
	// Settle: rendering reaches a fixed point after one cycle.
	//
	// Separate from Render because the two fail independently. A comment can
	// move about without any value changing, and an entry that excused both
	// would be tolerating documents that are fine on one of them.
	Settle
	// CommentsKept: rendering keeps every comment the document had.
	//
	// Nothing else notices a lost comment. Comments carry no meaning, so the
	// value is unaffected and rendering still settles -- the document is simply
	// poorer than the one that went in, which for a library that offers to
	// preserve them is the whole failure.
	CommentsKept
	// Parses: the emitted document is one the library will read at all.
	//
	// Weaker than Decode and worth separating: a document that is rejected
	// outright is a different failure from one that is read as the wrong
	// value, and it is the only one where the library and the grammar can be
	// asked the same question.
	Parses
)

func (p Property) String() string {
	names := []string{}
	if p&Decode != 0 {
		names = append(names, "decode")
	}
	if p&Render != 0 {
		names = append(names, "render")
	}
	if p&Settle != 0 {
		names = append(names, "settle")
	}
	if p&CommentsKept != 0 {
		names = append(names, "comments")
	}
	if p&Parses != 0 {
		names = append(names, "parses")
	}

	return strings.Join(names, "|")
}

// Ledger records every shape known to diverge.
//
// An entry is not an excuse. It is a measurement with a name attached, and the
// property test reports which entries were exercised and which of those
// actually diverged -- so an entry that has been fixed shows up as one that no
// longer diverges, rather than sitting here forever.
//
// Three entries, all of them block scalars. Every other shape the generator
// draws holds all five properties, which is a claim the property tests re-earn
// on every run rather than a note about how things once stood: a new divergence
// fails because it is missing from here, and a fixed one fails because it is
// still listed.
//
// Every entry here is a parser or renderer defect rather than an open question.
// The emitter is gated against the YAML 1.2 grammar, so each of these documents
// is known to be one the library is obliged to read.
var Ledger = []Divergence{
	{
		Name: "a-trailing-blank-line-under-a-stated-indent-is-rejected",
		// The document is not read at all, so there is no value to compare and
		// nothing to render: it fails the two questions asked before those.
		Property: Parses | Decode,
		Reason: "a block scalar whose header states its indentation is rejected " +
			"when it ends on a blank line that is not padded out to the stated " +
			"width. An empty line is allowed to have less indentation than the " +
			"header states -- l-empty admits s-indent(<n), and the grammar " +
			"accepts every one of these -- and the same document is accepted " +
			"with the indicator dropped, with the blank line padded out, or with " +
			"the blank line anywhere but the end. Chomping has nothing to do " +
			"with it: |2 and |2+ are both rejected, and both are accepted once " +
			"the line is padded. It is only the emitter that ties the two " +
			"together, since a trailing blank line is what keep chomping is for",
		Match: func(v Value, st Style) bool {
			if st.Flow || !st.BlockIndicator {
				return false
			}

			return anyValueString(v, func(s string) bool {
				// Keep chomping is what the emitter picks for more than one
				// trailing newline, and it is what writes them out.
				return writtenAsBlockScalar(s, st) && len(s)-len(strings.TrimRight(s, "\n")) >= 2
			})
		},
	},
	{
		Name:     "keep-chomping-loses-its-blank-lines-when-folded",
		Property: Render,
		Reason: "a folded block scalar with keep chomping (>+) is written back " +
			"without the trailing blank lines it exists to preserve, so " +
			"\"trail\\n\\n\" comes back as \"trail\\n\". Reading it is correct, and " +
			"the same value in a literal scalar (|+) round trips, so this is the " +
			"one of the two block styles the fix did not reach",
		Match: func(v Value, st Style) bool {
			if st.Flow || !st.Folded {
				return false
			}

			return anyValueString(v, func(s string) bool {
				return canFolded(s) && len(s)-len(strings.TrimRight(s, "\n")) >= 2
			})
		},
	},
	{
		Name: "a-stated-indent-is-not-updated-when-the-content-is-re-indented",
		// Reading is correct; the renderer writes the content at its own indent
		// and leaves the header saying what the old one was. Settle joins it
		// because the mismatch grows by a column on every cycle rather than
		// reaching a fixed point.
		Property: Render | Settle,
		Reason: "a block scalar that states its indentation is re-indented to " +
			"the renderer's own width without the indicator being updated, so " +
			"the value gains a leading space on every cycle. A scalar with no " +
			"indicator round trips, because then the renderer is free to choose " +
			"the width and the content says what it is. Where the two widths " +
			"already agree nothing moves, which is why the entry draws more " +
			"documents than it diverges on",
		Match: func(v Value, st Style) bool {
			if st.Flow || !st.BlockIndicator {
				return false
			}

			return anyValueString(v, func(str string) bool {
				return writtenAsBlockScalar(str, st)
			})
		},
	},
}

// anyValueString reports whether any string in a value position satisfies pred.
//
// Mapping keys are excluded: a key is always written on one line, so the
// presentation choices that apply to a value do not apply to it.
func anyValueString(v Value, pred func(string) bool) bool {
	switch n := v.(type) {
	case Anchored:
		return anyValueString(n.V, pred)
	case Alias:
		// Written at the anchor, which this walk reaches on its own.
		return false
	case Str:
		return pred(n.V)
	case Seq:
		for _, item := range n.Items {
			if anyValueString(item, pred) {
				return true
			}
		}

		return false
	case Map:
		for _, p := range n.Pairs {
			if anyValueString(p.Val, pred) {
				return true
			}
		}

		return false
	default:
		return false
	}
}

// Known returns the ledger entry describing this pairing for the given
// property, or nil.
func Known(p Property, v Value, st Style) *Divergence {
	for i := range Ledger {
		if Ledger[i].Property&p != 0 && Ledger[i].Match(v, st) {
			return &Ledger[i]
		}
	}

	return nil
}

// Entries returns the ledger entries for one property.
func Entries(p Property) []Divergence {
	var out []Divergence
	for _, d := range Ledger {
		if d.Property&p != 0 {
			out = append(out, d)
		}
	}

	return out
}

// CommentsIn returns the comment markers a generated document carries, in the
// order they appear.
//
// Emit numbers its comments, so a lost one is identifiable rather than merely
// countable, and a moved one can be told from a dropped one.
func CommentsIn(src string) []string {
	return commentMarker.FindAllString(src, -1)
}

var commentMarker = regexp.MustCompile(`#\s*c\d+`)

// writtenAsBlockScalar reports whether st writes s as a block scalar, in
// whichever of the two styles applies.
//
// It mirrors the emitter's own choice rather than restating it: a predicate
// that guessed differently would excuse documents that were never drawn and
// leave drawn ones unaccounted for.
func writtenAsBlockScalar(s string, st Style) bool {
	return (st.Folded && canFolded(s)) || (st.Literal && canLiteral(s, st.BlockIndicator))
}
