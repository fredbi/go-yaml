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

	return strings.Join(names, "|")
}

// Ledger records every shape known to diverge.
//
// An entry is not an excuse. It is a measurement with a name attached, and the
// property test reports which entries were exercised and which of those
// actually diverged -- so an entry that has been fixed shows up as one that no
// longer diverges, rather than sitting here forever.
var Ledger = []Divergence{
	{
		Name: "comment-on-a-nested-sequence-entry-moves-or-is-lost",
		// One root cause with two symptoms, which is why it is one entry: the
		// comment is sometimes relocated and sometimes dropped, and both are
		// the renderer failing to keep it attached to the entry it came from.
		Property: Settle | CommentsKept,
		Reason: "a comment on a sequence entry that has no scalar on its line -- " +
			"the value is a collection, or an empty node -- is not kept where it " +
			"was. It moves onto the entry's line and then out to the head of the " +
			"document, so rendering takes two passes to settle, and where the " +
			"head is already taken the comment is dropped altogether. An entry " +
			"with a scalar on its line keeps its comment, and so does a mapping " +
			"entry. The shape is wider than either symptom: about half the " +
			"documents drawn from it fail to settle and slightly fewer drop a " +
			"comment, so the condition that separates moving one from dropping " +
			"it -- and either from leaving it alone -- is not yet pinned down. " +
			"Tightening this entry is the next thing worth doing to it: until " +
			"then it excuses roughly as many sound documents as unsound ones",
		Match: func(v Value, st Style) bool {
			if st.Flow || st.Comments == NoComments {
				return false
			}

			return hasSeqEntryWithoutInlineScalar(v, st)
		},
	},
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

// hasSeqEntryWithoutInlineScalar reports whether any sequence in the value has
// an entry that leaves its own line empty -- because the value is a collection
// written below it, or an empty node written as nothing at all.
//
// That is the shape whose comment has nothing to attach to. An entry with a
// scalar on its line is unaffected, and so is a collection nested under a
// mapping key, which is what keeps this from matching every commented document.
func hasSeqEntryWithoutInlineScalar(v Value, st Style) bool {
	switch n := v.(type) {
	case Seq:
		for _, item := range n.Items {
			if leavesItsLineEmpty(item, st) || hasSeqEntryWithoutInlineScalar(item, st) {
				return true
			}
		}

		return false
	case Map:
		for _, p := range n.Pairs {
			if hasSeqEntryWithoutInlineScalar(p.Val, st) {
				return true
			}
		}

		return false
	default:
		return false
	}
}

func leavesItsLineEmpty(v Value, st Style) bool {
	switch n := v.(type) {
	case Seq:
		return len(n.Items) > 0
	case Map:
		return len(n.Pairs) > 0
	case Null:
		// The empty spelling of null writes nothing, so the line holds only the
		// dash. Every other spelling puts a scalar there.
		return st.NullSpelling == ""
	case Str:
		// A literal block scalar puts its content below, not on the line.
		return st.Literal && canLiteral(n.V)
	default:
		return false
	}
}
