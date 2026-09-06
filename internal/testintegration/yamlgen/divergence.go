// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"math"
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

// Ledger records every shape known to diverge, and is empty.
//
// An entry is not an excuse. It is a measurement with a name attached, and the
// property test reports which entries were exercised and which of those
// actually diverged -- so an entry that has been fixed shows up as one that no
// longer diverges, rather than sitting here forever. Every entry it has held
// left that way, each named in a TestFixed* in fixed_test.go with the
// assertions inverted.
//
// Two were opened by [Style.Break], the axis that writes the same document with
// LF, CRLF and a lone CR, and the rest by [Tagged], [Style.PropertyOrder] and
// [Style.PropertyLine]. The ledger was empty before either; every entry in it
// came from an axis nobody had crossed, which is the argument for the axes in
// one sentence.
//
// An entry here is a parser or renderer defect rather than an open question.
// The emitter is gated against the YAML 1.2 grammar, so each of these documents
// is one the library is obliged to read. Add one when a property test finds a
// shape that diverges and the fix is not immediate; take it out with the fix.
var Ledger = []Divergence{
	{
		Name: "parse/a-mapping-key-written-empty-is-refused",
		Reason: "A mapping entry whose key is written empty -- `: 2` rather than `k: 2` -- is " +
			"refused in several positions. YAML 1.2 accepts every one of them and the grammar " +
			"agrees.\n\n" +
			"Three read correctly, which is what makes this a defect rather than a library that " +
			"does not implement empty keys: `: a` on its own, `a: 1` then `: 2`, and " +
			"`- k: 1` over `  : 2`. The last two were refused before 2026-09-10 and are fixed.\n\n" +
			"Three do not, with two different messages, so there is more than one fault behind " +
			"the shape. `a:` then `: 2` reports `unexpected scalar value`. `k: &a1` then `: 1` " +
			"and `false: !!bool false` then `: &a1 !!null` both report `mapping value is not " +
			"allowed in this context`, and both name the *earlier* line -- so what precedes the " +
			"empty key matters, and an entry whose value carries properties or is written empty " +
			"is what precedes it in each.\n\n" +
			"The predicate asks only whether an empty key is written at all. Narrowing it would " +
			"mean reproducing the parser's property handling inside a predicate, and one that " +
			"tracked the defect that closely would drift from it; the pinned cases in " +
			"defects_test.go carry the precision. It reports fewer divergences than draws.\n\n" +
			"It claims every property, because a document that does not parse answers none.",
		Property: Parses | Decode | Render | Settle | CommentsKept,
		Match:    writesAnEmptyKey,
	},
	{
		Name: "parse/a-float-tag-on-a-number-past-float64-is-not-read",
		Reason: "`!!float` written on a number a float64 cannot hold fails two ways, where the same " +
			"number untagged reads correctly as a big.Float.\n\n" +
			"Large, the parse stops: `!!float 1e+310` reports `cannot read \"1e+310\" as !!float`. " +
			"Small, it is worse than refused: `!!float 1e-400` comes back as the float64 **zero**, with " +
			"nothing reported.\n\n" +
			"Untagged, `1e+310` and `1e-400` are both read as a big.Float, and `!!int` on an integer " +
			"past a machine word reads as a big.Int -- so the wide types are built, and it is the float " +
			"tag alone that does not know about them.\n\n" +
			"It claims every property: the large form does not parse, and a document that does not parse " +
			"answers none of them.",
		Property: Parses | Decode | Render | Settle | CommentsKept,
		Match:    writesFloatTaggedWideNumber,
	},
	{
		Name: "parse/a-tag-not-written-as-a-shorthand-does-not-type-its-scalar",
		Reason: "The scanner types the scalar under a `!!` tag and leaves the one under the same tag " +
			"written any other way as a string. `!!float 7` parses to an ast.IntegerNode under its " +
			"ast.TagNode and `!!float 1e3` to an ast.FloatNode; `!<tag:yaml.org,2002:float> 7` and " +
			"`!e!float 1e3` both parse to an ast.StringNode. The tag's URI is the same in every case, " +
			"so the tree a consumer walks depends on how the tag was spelled.\n\n" +
			"For most values nothing further goes wrong -- the string is read against the tag " +
			"afterwards and `7`, `0x1f`, `1e3`, `true` and `null` all come back right. The exception is " +
			"the three specials: `!<tag:yaml.org,2002:float> .inf` decodes to the float64 **zero** with " +
			"nothing reported, where `!!float .inf` decodes to +Inf. `-.inf` and `.nan` go the same way.\n\n" +
			"codec.ToJSON loses the same value and loses a refusal with it. `!!float .inf` is refused " +
			"with `JSON has no number for .inf`, which is right -- JSON has no infinity -- while the " +
			"verbatim spelling writes `0.0` and reports nothing.\n\n" +
			"A collection tag is unaffected: `!<tag:yaml.org,2002:seq> [1, .inf]` reads +Inf, because " +
			"the scalar inside it carries no tag of its own.\n\n" +
			"The predicate matches only the shape that loses a value, so a document merely carrying a " +
			"tag written out in full is still held to every property.\n\n" +
			"Decode and Render, and Render because the value is already wrong when it is first read: " +
			"the rendering is byte-identical to the document that went in.",
		Property: Decode | Render,
		Match:    writesASpecialFloatUnderALongTag,
	},
	{
		Name: "decode/a-tagged-block-mapping-does-not-resolve-its-keys",
		Reason: "A tag on a block mapping leaves every key as the text that was written, where " +
			"an untagged one names it by the canonical spelling of its type. `!foo` over " +
			"`False: 1` reads the key \"False\"; without the tag it reads \"false\".\n\n" +
			"Three things narrow it, and each is what makes this a defect rather than a rule " +
			"about tagged nodes. `!!map` over the same mapping resolves the keys, so it is not " +
			"that a tag suppresses resolution -- one spelling of the same tag behaves and the " +
			"others do not. A flow mapping resolves them under any tag, `!foo {False: 1}` " +
			"reading \"false\". And an anchor makes no difference either way.\n\n" +
			"That first sentence holds for the `!!map` shorthand alone. Written in full as " +
			"`!<tag:yaml.org,2002:map>`, or through a declared handle as `!e!map`, the same tag " +
			"leaves the keys as text like any other -- which is the same root cause as " +
			"parse/a-tag-not-written-as-a-shorthand-does-not-type-its-scalar, seen on a mapping " +
			"instead of on a scalar. The predicate takes Style.TagSpelling for that reason.\n\n" +
			"Only visible where a key's text and its canonical name differ, which since the " +
			"naming rule landed means the booleans and the nulls: `!foo` over `1.0: a` reads " +
			"\"1.0\" correctly, because that is the text as well as the name.",
		Property: Decode | Render,
		Match:    writesTaggedBlockMappingKeys,
	},
}

// writesFloatTaggedWideNumber reports whether v carries `!!float` on a number
// no float64 holds.
func writesFloatTaggedWideNumber(v Value, _ Style) bool {
	switch n := v.(type) {
	case Tagged:
		if _, wide := n.V.(BigFloat); wide && n.Tag == TagFloat {
			return true
		}

		return writesFloatTaggedWideNumber(n.V, Style{})
	case Anchored:
		return writesFloatTaggedWideNumber(n.V, Style{})
	case Seq:
		return slices.ContainsFunc(n.Items, func(item Value) bool {
			return writesFloatTaggedWideNumber(item, Style{})
		})
	case Map:
		for _, p := range n.Pairs {
			if writesFloatTaggedWideNumber(p.Val, Style{}) {
				return true
			}
		}
	}

	return false
}

// writesASpecialFloatUnderALongTag reports whether v writes an infinity or a
// NaN under a float tag that st does not spell as a `!!` shorthand.
func writesASpecialFloatUnderALongTag(v Value, st Style) bool {
	if st.TagSpelling == SpellShorthand {
		return false
	}

	return holdsASpecialFloatUnderAFloatTag(v)
}

func holdsASpecialFloatUnderAFloatTag(v Value) bool {
	switch n := v.(type) {
	case Tagged:
		if f, number := n.V.(Float); number && n.Tag == TagFloat && isSpecial(f.V) {
			return true
		}

		return holdsASpecialFloatUnderAFloatTag(n.V)
	case Anchored:
		return holdsASpecialFloatUnderAFloatTag(n.V)
	case Seq:
		return slices.ContainsFunc(n.Items, holdsASpecialFloatUnderAFloatTag)
	case Map:
		for _, p := range n.Pairs {
			if holdsASpecialFloatUnderAFloatTag(p.Key) || holdsASpecialFloatUnderAFloatTag(p.Val) {
				return true
			}
		}
	}

	return false
}

// isSpecial reports the three floats YAML spells with a leading '.'.
func isSpecial(f float64) bool { return math.IsInf(f, 0) || math.IsNaN(f) }

// writesTaggedBlockMappingKeys reports whether emitting v writes a mapping that
// carries a tag other than `!!map`.
//
// Wider than the defect twice over, and both are the usual trade. It does not
// ask whether any key's text differs from its canonical name, so a mapping
// keyed by ordinary words matches and reads back correctly. And it does not ask
// whether the mapping lands in block context, because that depends on its depth
// and on Style.FlowFrom.
//
// The one tag that behaves is `!!map`, and only while it is written as that
// shorthand: st decides, so the same tag written out in full matches here.
func writesTaggedBlockMappingKeys(v Value, st Style) bool {
	switch n := v.(type) {
	case Tagged:
		if _, keyed := n.V.(Map); keyed && (n.Tag != TagMap || st.TagSpelling != SpellShorthand) {
			return true
		}

		return writesTaggedBlockMappingKeys(n.V, st)
	case Anchored:
		return writesTaggedBlockMappingKeys(n.V, st)
	case Seq:
		return slices.ContainsFunc(n.Items, func(item Value) bool {
			return writesTaggedBlockMappingKeys(item, st)
		})
	case Map:
		for _, p := range n.Pairs {
			if writesTaggedBlockMappingKeys(p.Val, st) {
				return true
			}
		}
	}

	return false
}

// writesAnEmptyKey reports whether emitting v in st writes a mapping key with
// no text at all.
//
// Only a bare Null reaches the document as nothing, and only when the style
// spells null as nothing. A tag or an anchor in front of one writes itself.
func writesAnEmptyKey(v Value, st Style) bool {
	return st.NullSpelling == "" && holdsAnEmptyKey(v)
}

func holdsAnEmptyKey(v Value) bool {
	switch n := v.(type) {
	case Map:
		for _, p := range n.Pairs {
			if isNullNode(p.Key) || holdsAnEmptyKey(p.Val) {
				return true
			}
		}
	case Seq:
		return slices.ContainsFunc(n.Items, holdsAnEmptyKey)
	case Anchored:
		return holdsAnEmptyKey(n.V)
	case Tagged:
		return holdsAnEmptyKey(n.V)
	}

	return false
}

// isNullNode reports whether v writes nothing when the style spells null as
// nothing.
func isNullNode(v Value) bool {
	_, empty := v.(Null)

	return empty
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
