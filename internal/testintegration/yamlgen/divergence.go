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
	// DecodeTyped: reading the emitted document into a Go type built from the
	// value gives what reading it into an `any` gives.
	//
	// A separate property because it is a separate path through codec. Reading
	// into an `any` walks the token stream; reading into a Go type gathers a
	// tree and fills fields by reflection, and since the two stopped sharing
	// code they can disagree silently. The comparison is against the `any`
	// answer rather than against Value.Decoded, so a defect on both paths
	// cancels out and only the destination is left as the variable. See
	// [TargetFor].
	DecodeTyped
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
	if p&DecodeTyped != 0 {
		names = append(names, "decode-typed")
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
		Name: "decode/an-int-tag-cannot-be-read-into-a-go-integer",
		Reason: "`!!int` on a value cannot be read into any Go integer -- a struct field, a slice " +
			"element or a map value. `n: !!int 5` into a struct with an int64 field is refused with " +
			"`cannot unmarshal int into Go struct field box.N of type int64`, and so are int and " +
			"uint64 fields, `- !!int 5` into a []int64 and `n: !!int 5` into a map[string]int64.\n\n" +
			"Four things say it is the tag and nothing else. The same document untagged reads into " +
			"all of them. `!!str`, `!!bool` and `!!float` on their matching fields read. `!!int` into " +
			"an `any` field reads. And go.yaml.in/yaml/v3 v3.0.5 reads every one of them.\n\n" +
			"[Tagged.Decoded] says where to look: an untagged non-negative integer comes back as a " +
			"uint64 and a negative one as an int64, and `!!int` overrides both with a plain int. The " +
			"reflection path has a case for the first two and none for int.\n\n" +
			"Every spelling of the tag does it, the verbatim and handle forms included.",
		Property: DecodeTyped,
		Match:    writesAnIntTag,
	},
	{
		Name: "decode/one-non-string-key-zeroes-a-whole-struct",
		Reason: "One key a struct cannot name leaves every field at its zero value and reports " +
			"nothing -- including the fields whose keys are strings, and whether the offending key " +
			"stands before them or after. `1: a` beside `name: x` into a struct with a Name field " +
			"gives an empty Name and no error.\n\n" +
			"In keyToNodeMap, `key, ok := keyVal.(string)` falls to `return nil, err` where err is " +
			"nil, so the whole key map comes back nil. go.yaml.in/yaml/v3 v3.0.5 reads the document " +
			"and so does this library's own `any` path; only the struct path drops it.\n\n" +
			"⏸ Parked deliberately -- Fred, 2026-09-06 -- until decodeStruct is inverted, which " +
			"deletes the line it lives on. Fixing it in keyToNodeMap first would be work thrown " +
			"away.\n\n" +
			"The predicate matches any mapping with a key that is not a Str, which is what a struct " +
			"tag cannot name.",
		Property: DecodeTyped,
		Match:    writesANonStringKey,
	},
	{
		Name: "decode/a-key-after-a-long-tag-on-an-empty-value-is-not-resolved",
		Reason: "An entry whose value is a tag written in full with nothing after it stops the key on " +
			"the next line from resolving. `a: !<tag:yaml.org,2002:null>` over `False: 1` reads the " +
			"key \"False\"; `a: !!null` over the same line reads \"false\".\n\n" +
			"Four things narrow it. Only the key immediately after -- `NULL: 2` and `0x10: 4` further " +
			"down the same mapping resolve correctly. Only in block context: " +
			"`{a: !<tag:yaml.org,2002:null>, False: 1}` resolves it. Only with nothing after the tag: " +
			"`a: !<tag:yaml.org,2002:null> null` resolves it. And only the long spellings, the handle " +
			"form `!e!null` included -- which is what makes this the same root cause as " +
			"parse/a-tag-not-written-as-a-shorthand-does-not-type-its-scalar.\n\n" +
			"libfyaml 1.0.0b1, go.yaml.in/yaml/v3 v3.0.5 and the reference parser all read the key as " +
			"the boolean's name.\n\n" +
			"The predicate asks only whether a long-spelled tag stands on a node written with nothing " +
			"after it, so it reports more draws than divergences: the same tag on the last entry of a " +
			"mapping has no next key to lose.",
		Property: Decode | Render,
		Match:    writesALongTagOnAnEmptyNode,
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

// writesAnIntTag reports whether v carries `!!int` anywhere.
func writesAnIntTag(v Value, _ Style) bool {
	switch n := v.(type) {
	case Tagged:
		return n.Tag == TagInt || writesAnIntTag(n.V, Style{})
	case Anchored:
		return writesAnIntTag(n.V, Style{})
	case Alias:
		return writesAnIntTag(n.V, Style{})
	case Seq:
		return slices.ContainsFunc(n.Items, func(item Value) bool {
			return writesAnIntTag(item, Style{})
		})
	case Map:
		for _, p := range n.Pairs {
			if writesAnIntTag(p.Key, Style{}) || writesAnIntTag(p.Val, Style{}) {
				return true
			}
		}
	}

	return false
}

// writesANonStringKey reports whether v holds a mapping keyed by anything other
// than a string, which is a key no struct tag names.
func writesANonStringKey(v Value, _ Style) bool {
	switch n := v.(type) {
	case Map:
		for _, p := range n.Pairs {
			if _, text := p.Key.(Str); !text {
				return true
			}

			if writesANonStringKey(p.Val, Style{}) {
				return true
			}
		}
	case Seq:
		return slices.ContainsFunc(n.Items, func(item Value) bool {
			return writesANonStringKey(item, Style{})
		})
	case Anchored:
		return writesANonStringKey(n.V, Style{})
	case Alias:
		return writesANonStringKey(n.V, Style{})
	case Tagged:
		return writesANonStringKey(n.V, Style{})
	}

	return false
}

// writesALongTagOnAnEmptyNode reports whether v writes a tag in one of the long
// spellings over a node that reaches the document as nothing.
//
// Only a bare Null does, and only when the style spells null as nothing.
func writesALongTagOnAnEmptyNode(v Value, st Style) bool {
	if st.TagSpelling == SpellShorthand || st.NullSpelling != "" {
		return false
	}

	return holdsATaggedNull(v)
}

func holdsATaggedNull(v Value) bool {
	switch n := v.(type) {
	case Tagged:
		if _, empty := n.V.(Null); empty {
			return true
		}

		return holdsATaggedNull(n.V)
	case Anchored:
		return holdsATaggedNull(n.V)
	case Seq:
		return slices.ContainsFunc(n.Items, holdsATaggedNull)
	case Map:
		for _, p := range n.Pairs {
			if holdsATaggedNull(p.Key) || holdsATaggedNull(p.Val) {
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
