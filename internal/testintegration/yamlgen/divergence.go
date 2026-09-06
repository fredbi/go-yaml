// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"regexp"
	"slices"
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
	// RenderValid: the text the renderer wrote is a YAML 1.2 document.
	//
	// Only the grammar can answer this. Re-reading the rendering, which is what
	// Render and Settle do, cannot: a renderer and a parser that make the same
	// mistake agree with each other while the file on disk is one no other tool
	// will read.
	RenderValid
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
	if p&RenderValid != 0 {
		names = append(names, "render-valid")
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
// Two of these were opened by [Style.Break], the axis that writes the same
// document with LF, CRLF and a lone CR, and the rest by [Tagged],
// [Style.PropertyOrder] and [Style.PropertyLine]. The ledger was empty before
// either; every entry in it came from an axis nobody had crossed.
//
// An entry here is a parser or renderer defect rather than an open question.
// The emitter is gated against the YAML 1.2 grammar, so each of these documents
// is one the library is obliged to read.
var Ledger = []Divergence{
	{
		Name: "render/a-comment-above-a-property-line-moves-onto-the-last-entry",
		Reason: "A comment on the line that introduces a node, where the node's properties " +
			"are written on the next line, comes back attached to that node's last entry. " +
			"`k: # c1` then `  &a2` then `  - 1` renders as `k: &a2` then `- 1 # c1`. It needs " +
			"only an anchor, so this is about where the properties sit rather than about " +
			"tags.\n\n" +
			"Harmless where the last entry is a plain scalar and not harmless at all where " +
			"it is a block scalar: the comment lands inside the content, and " +
			"`  - |-` then `    trailing ` comes back as \"trailing  # c1\" instead of " +
			"\"trailing \". So the entry claims Render as well as CommentsKept, and " +
			"Settle besides -- a comment that has moved onto a line whose own comment is " +
			"still there renders differently again next time. It reports fewer " +
			"divergences than draws on all three.",
		Property: Render | Settle | CommentsKept,
		Match: func(v Value, st Style) bool {
			return st.Comments.line() && writesPropertyLine(v, st)
		},
	},
	{
		Name: "parse/a-tag-before-an-anchor-is-dropped",
		Reason: "YAML 1.2 lets a node's tag and anchor appear in either order and means the " +
			"same by both. Written second the tag holds; written first it is dropped from " +
			"the node the anchor names. A scalar parses and the anchor resolves to the " +
			"untagged value, so `a: !!str &a1 5` reads \"5\" at a and 5 at `b: *a1` -- one " +
			"node, two types. `!!null &a1 null` parses and a later `*a1` reports " +
			"`could not find alias`.\n\n" +
			"A collection tag was a third kind and is fixed: `!!seq &a1 [1]` and " +
			"`!!map &a1 {b: 1}` did not parse at all, and read correctly since " +
			"2026-09-07. See TestFixedACollectionTagBeforeAnAnchorParses.\n\n" +
			"On an empty node the tag swallows what follows: `- !!null &a1` over `- x` " +
			"takes the entry below it, and so do `- !!str &a1` over `- x` and " +
			"`k: !!null &a1` over `j: x`. Anchor first reads all of them correctly. " +
			"The swallowing is still here; what it does is now reported. It used to " +
			"come back as a one-item sequence with no error at all, and the load " +
			"refuses it since 2026-09-07, because the entry the tag swallowed turns the " +
			"node into a collection and ast.TagNode.Resolve reports a scalar tag " +
			"standing on one.\n\n" +
			"So two shapes fail two ways: a tag on an empty node eats what follows, and " +
			"any other tag is dropped so quietly that nothing notices until an alias asks " +
			"the anchor what it names. The predicate asks for one of the two.\n\n" +
			"It claims all five properties, which no other entry does and this one has " +
			"earned: a node that eats the rest of the document takes the comments with " +
			"it, moves what it swallowed -- `a: !!null &a1` over `b: 1` comes back with b " +
			"indented under a -- and a document whose entries have shifted does not render " +
			"the same way twice.",
		Property: Parses | Decode | Render | Settle | CommentsKept,
		Match:    writesBrokenTaggedAnchor,
	},
	{
		Name: "render/a-comment-after-a-line-ending-tag-is-dropped",
		Reason: "A comment sitting after a tag that is the last thing on its line does not " +
			"survive a render. `!!null # c1` and `&a1 !!null # c1` come back as `!!null` " +
			"and `&a1 !!null`; so does `- &a2 !!seq # c2` with its entries on the lines " +
			"below. The anchor on its own keeps the comment -- `&a1 # c1` renders " +
			"unchanged -- and so does the same tag once anything follows it on the line, " +
			"as in `!!null null # c1`.",
		Property: CommentsKept,
		Match: func(v Value, st Style) bool {
			return st.Comments.line() && writesTagAtLineEnd(v, st)
		},
	},
	{
		Name: "render/a-str-tagged-null-spelling-is-lowercased",
		Reason: "The renderer writes a `!!str`-tagged null spelling back in lower case. " +
			"`k: !!str Null` and `k: !!str NULL` both come back as `k: !!str null`, and " +
			"under that tag `null` is the four letters, so \"Null\" has become \"null\" " +
			"and the value is gone.\n\n" +
			"The decode is right and only the render is wrong, which is what pins this on " +
			"the renderer: `!!str Null` reads \"Null\" today, and reading it was the half " +
			"of this fixed on the parser branch. `!!str True` renders unchanged, so it is " +
			"the null spellings and not the resolving spellings in general.\n\n" +
			"It claims Render and Settle: the second cycle reads \"null\" and writes it " +
			"again, so the document has stopped moving only after the value went.",
		Property: Render | Settle,
		Match:    writesStrTaggedNullSpelling,
	},
}

// strTaggedNullSpellings are the plain spellings the renderer lower-cases when
// `!!str` stands in front of them.
//
// The null words alone. `True` and `TRUE` carry the same tag through a render
// unchanged, which is what makes this the renderer resolving the scalar it was
// told not to resolve rather than a general case-folding.
var strTaggedNullSpellings = map[string]struct{}{
	"Null": {}, "NULL": {},
}

// writesStrTaggedNullSpelling reports whether emitting v in st writes a `!!str`
// tag over a null spelling.
//
// Quoting is the only style axis consulted, and the predicate is wider than the
// defect on purpose. The spelling has to reach the document unquoted, which
// canPlain allows only under this tag; whether a given node is written plain or
// as a block scalar depends on its depth and on Style.FlowFrom, and a predicate
// that recomputed the emitter's decision would drift from it. So a folded style
// draws some documents that render correctly, and the tally reports fewer
// divergences than draws -- the same trade the comment-above entry makes.
func writesStrTaggedNullSpelling(v Value, st Style) bool {
	return st.Quoting == QuotePlain && holdsStrTaggedNull(v)
}

func holdsStrTaggedNull(v Value) bool {
	switch n := v.(type) {
	case Tagged:
		if inner, ok := n.V.(Str); ok && n.Tag == TagStr {
			if _, spelt := strTaggedNullSpellings[inner.V]; spelt {
				return true
			}
		}

		return holdsStrTaggedNull(n.V)
	case Anchored:
		return holdsStrTaggedNull(n.V)
	case Seq:
		return slices.ContainsFunc(n.Items, holdsStrTaggedNull)
	case Map:
		for _, pair := range n.Pairs {
			if holdsStrTaggedNull(pair.Val) {
				return true
			}
		}
	}

	return false
}

// writesBrokenTaggedAnchor reports whether emitting v in st writes a tag before
// an anchor in a way this library gets wrong.
//
// Three cases, measured. `!!seq` and `!!map` written ahead of an anchor stop
// the parse whatever else the document does. Any tag written that way on an
// empty node swallows the rest of the document. Otherwise the tag is silently
// dropped from the node the anchor names, which changes nothing until an alias
// resolves to it -- so the third case asks whether one does.
func writesBrokenTaggedAnchor(v Value, st Style) bool {
	e := &emitter{st: st}
	e.root(v)

	// collectionTagAnchors is deliberately not asked for: a collection tag
	// written before an anchor was the third shape of this defect and it was
	// fixed on 2026-09-07. See TestFixedACollectionTagBeforeAnAnchorParses.
	if e.emptyTagAnchors > 0 {
		return true
	}
	if len(e.taggedAnchorNames) == 0 {
		return false
	}

	named := aliasNames(v)
	for _, name := range e.taggedAnchorNames {
		if named[name] {
			return true
		}
	}

	return false
}

// aliasNames returns the anchors that something in v refers to.
func aliasNames(v Value) map[string]bool {
	out := map[string]bool{}
	var walk func(Value)

	walk = func(v Value) {
		switch n := v.(type) {
		case Alias:
			out[n.Name] = true
		case Anchored:
			walk(n.V)
		case Tagged:
			walk(n.V)
		case Seq:
			for _, item := range n.Items {
				walk(item)
			}
		case Map:
			for _, p := range n.Pairs {
				walk(p.Val)
			}
		}
	}

	walk(v)

	return out
}

// writesPropertyLine reports whether emitting v in st puts a node's properties
// on a line of their own.
func writesPropertyLine(v Value, st Style) bool {
	e := &emitter{st: st}
	e.root(v)

	return e.propertyLines > 0
}

// writesTagAtLineEnd reports whether emitting v in st writes a tag as the last
// thing on its line.
func writesTagAtLineEnd(v Value, st Style) bool {
	e := &emitter{st: st}
	e.root(v)

	return e.taggedLineEnds > 0
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
