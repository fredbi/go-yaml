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
//
// One entry, found by generating anchors: every other shape the generator has
// drawn holds all four properties. That is a claim the property tests re-earn
// on every run rather than a note about how things once stood -- a new
// divergence fails because it is missing from here, and a fixed one fails
// because it is still listed.
var Ledger = []Divergence{
	{
		Name: "comment-after-an-anchored-empty-entry-swallows-the-rest",
		// The value changes, and it changes structurally: entries that were
		// siblings come back nested. Settle joins it because the document that
		// comes back is not the one that went in and rendering it again moves
		// it further, which is measured rather than assumed -- the comment case
		// alone settles perfectly well while meaning something else.
		Property: Render | Settle,
		Reason: "an entry whose line ends on an anchor that named nothing -- an " +
			"anchored empty node -- followed by a comment line and then another " +
			"entry, makes the renderer indent the comment under that entry and " +
			"every entry after it with the comment, so [nil, x] comes back as " +
			"[[x]] and two mapping entries come back as one nested in the " +
			"other. Every part is needed: without the anchor it round trips, and " +
			"so does an anchored collection or block scalar, whose value gives " +
			"the comment something to sit after, and so does an anchored empty " +
			"node that is the last entry, with nothing left to swallow. " +
			"Sequence and mapping entries fail alike, which places it in how an " +
			"anchor ends a line rather than in either collection. The entry is " +
			"tight on Render, at roughly half the documents it draws, and loose " +
			"on Settle, at nearer one in thirty: only the dedent trigger fails " +
			"to settle, while the comment trigger produces a wrong document that " +
			"is perfectly stable. Splitting the two triggers into separate " +
			"entries is the next thing worth doing to it",
		Match: func(v Value, st Style) bool {
			// Comments are one of the two ways for something to follow, not a
			// precondition: the predicate decides which one applies.
			if st.Flow {
				return false
			}

			return hasAnchoredEntryLeavingItsLineEmpty(v, st)
		},
	},
}

// hasAnchoredEntryLeavingItsLineEmpty reports whether the document has an entry
// whose line ends on an anchor that named nothing, with something after it that
// the renderer will pull into it.
//
// Two ways for something to follow, because the swallowing is the same either
// way and only what gets swallowed differs:
//
//   - a comment on the next line, which is then indented under the anchor and
//     takes every later entry with it;
//   - the end of a nested collection, where what follows belongs to an outer
//     one. The renderer writes a block sequence under a mapping key at the
//     key's own column, so a following key lands level with the entry that
//     ended on the anchor and is read as part of it.
//
// A sibling in the same collection is not enough on its own: `- &a1` followed
// by `- x` round trips. That is what makes this about an anchor ending a line
// with something at a *different* level after it, rather than about anchors
// having neighbors.
func hasAnchoredEntryLeavingItsLineEmpty(v Value, st Style) bool {
	swallows, _ := scanForAnchoredEmpty(v, st)

	return swallows
}

// scanForAnchoredEmpty reports whether v swallows something, and how deep below
// v the last node of its subtree ends on a bare anchor: 0 when v is that node
// itself, one more for each level up, and -1 when it does not end on one.
//
// The depth is what separates the two cases. A child that *is* the bare anchor
// has its following sibling written at the same indentation, and that is fine.
// A child that merely *ends* with one, deeper down, has the following sibling
// written at an outer level, and that is what gets absorbed.
func scanForAnchoredEmpty(v Value, st Style) (bool, int) {
	if endsOnItsAnchor(v, st) {
		return false, 0
	}

	switch n := v.(type) {
	case Anchored:
		return scanForAnchoredEmpty(n.V, st)
	case Alias:
		// Written at the anchor, which this walk reaches on its own.
		return false, -1
	case Seq:
		return scanChildren(len(n.Items), func(i int) Value { return n.Items[i] }, st)
	case Map:
		return scanChildren(len(n.Pairs), func(i int) Value { return n.Pairs[i].Val }, st)
	default:
		return false, -1
	}
}

func scanChildren(count int, at func(int) Value, st Style) (bool, int) {
	swallows := false
	last := -1

	for i := range count {
		s, depth := scanForAnchoredEmpty(at(i), st)
		swallows = swallows || s

		if depth >= 0 && i < count-1 {
			// A comment lands between the two whatever the levels are; a
			// dedent only bites when the anchor is deeper than this collection.
			if st.Comments != NoComments || depth >= 1 {
				swallows = true
			}
		}

		if i == count-1 {
			last = depth
		}
	}

	if last < 0 {
		return swallows, -1
	}

	return swallows, last + 1
}

// endsOnItsAnchor reports whether an entry's line ends on the anchor itself,
// with no value of any kind after it.
//
// Only an empty node does that. An anchored collection puts its first entry on
// the next line and an anchored block scalar puts its header on this one, and
// both of those round trip -- so what the renderer cannot place is a comment
// under an anchor that named nothing.
func endsOnItsAnchor(v Value, st Style) bool {
	a, ok := v.(Anchored)
	if !ok {
		return false
	}
	_, empty := a.V.(Null)

	return empty && st.NullSpelling == ""
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
