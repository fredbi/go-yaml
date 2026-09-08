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
	// Pin names the test in defects_test.go that reproduces this entry with one
	// document, deterministically.
	//
	// Required, and TestEveryLedgerEntryNamesItsPin holds it to a test that
	// exists. It is what the tally's staleness check sends the reader to: an
	// entry drawn past suspectAfter times with no divergence is either fixed or
	// matching a family it is not in, and the count alone cannot say which --
	// the pin can, because it runs the one document the entry was written for.
	Pin string
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
	// StreamDecode: the documents of a stream read back as the values they
	// were written from.
	//
	// A property of its own because a stream is not a document: the separator,
	// the scope of what a document declares, and the count are questions a
	// single document cannot ask. An entry claiming it is consulted by
	// TestAStreamReadsBackAsItsDocuments and by nothing else, so a shape that
	// only goes wrong in a stream excuses nothing anywhere else.
	StreamDecode
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
	if p&StreamDecode != 0 {
		names = append(names, "stream-decode")
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
		Name: "parse/a-tab-before-an-anchored-block-scalar-scans-its-content-as-plain",
		Pin:  "TestDefectATabBeforeAnAnchoredBlockScalarScansItsContentAsPlain",
		Reason: "`&a1\t>-` over ` , a` is refused with `a plain scalar cannot begin with \",\"`. The " +
			"content of a block scalar is not a plain scalar and may begin with anything, so the " +
			"header is not being taken.\n\n" +
			"Four things narrow it, and each is a document that reads: the same with a space, " +
			"`&a1 >-`, reads \", a\"; the same with a *tag*, `!!str\t>-`, reads it too; content that " +
			"could begin a plain scalar, `&a1\t>-` over ` x`, reads; and more indentation does not " +
			"help. So it is an anchor, a tab, and a block scalar together.\n\n" +
			"grammar.NewRecognizer accepts the document, the reference parser passes it and " +
			"go.yaml.in/yaml/v3 v3.0.5 reads \", a\".\n\n" +
			"⚠️ **Not a residue of a0182a6**, which is the attribution to avoid: at 1c59c89, the " +
			"commit before it, this document is refused with the identical message. What a0182a6 " +
			"changed among these four is the tag row alone -- `!!str\t>-` went from refused to " +
			"reading. So it fixed a sibling and left this one, which is what made the gap visible. " +
			"Calling it a residue would send a bisect at the wrong commit.\n\n" +
			"The cause, from the token stream: `&a1\t>-` scans as Anchor `&` then String `a1>-`, the " +
			"header joined to the anchor's name, where `&a1 >-` gives Anchor, String `a1`, Folded " +
			"`>-`. So the question of whether a tab ends a property is never asked on this path.\n\n" +
			"It claims every property, because a document that does not parse answers none.",
		Property: Parses | Decode | Render | Settle | CommentsKept | RenderValid | DecodeTyped,
		Match:    writesATabBeforeAnAnchoredBlockScalar,
	},
	{
		Name: "parse/a-collection-key-written-alone-in-flow-is-refused",
		Pin:  "TestDefectACollectionKeyWrittenAloneInFlowIsRefused",
		Reason: "`{{\"\": 0}}` is refused with `could not find flow map content`. 7.4.2 lets a flow " +
			"mapping entry be a key with no value, and lets that key be any flow node -- a mapping " +
			"or a sequence included.\n\n" +
			"`{{a: 0}: v}`, the same key with a value, parses here and reads {map[a:0]: v}, so it " +
			"is the missing value and nothing else.\n\n" +
			"⚠️ libfyaml 1.0.0b1 cannot answer: it refuses all three of `{{\"\": 0}}`, `{[a]}` and " +
			"`{{a: 0}: v}` with a Python traceback, and the last is a document this library reads " +
			"correctly -- the binding cannot hash a collection as a dict key. A traceback after a " +
			"parse is a construction refusal, not a verdict on the syntax. So the two " +
			"grammar-derived oracles answer and both accept.\n\n" +
			"The one-document record with the whole measurement is yamlgen.Strict's entry of the " +
			"same name; this is what excuses the drawn documents.\n\n" +
			"Found on 2026-09-07, when Keys began drawing a collection: Style.FlowEmpty writes an " +
			"entry with no value and a collection key under it is this document.",
		Property: Parses | Decode | Render | Settle | CommentsKept | RenderValid | DecodeTyped,
		Match:    writesACollectionKeyAloneInFlow,
	},
	{
		Name: "render/a-key-written-below-its-indicator-loses-its-indentation",
		Pin:  "TestDefectAKeyBelowItsIndicatorLosesItsIndentation",
		Reason: "A collection key written below its `?` comes back at column 1, where it is no longer " +
			"the key. `?` over a blank line over ` \"\": 0` over `: v` renders to `? ` over " +
			"`\"\": 0` over `: v` -- three entries where there was one, and the value changes with " +
			"the shape.\n\n" +
			"A head comment is what puts the blank line there, and the blank line is the whole " +
			"trigger: `?` over ` a: 0` over `: v` renders to `? a: 0` over `: v`, which is a " +
			"different spelling of the same document and settles.\n\n" +
			"It claims Settle and RenderValid rather than Decode: the first read is right, and it is " +
			"the text written back that stops being the document.\n\n" +
			"Reached on 2026-09-07, when Keys began drawing a collection -- the key had to be one " +
			"that cannot go on the `?`s own line before anything could be written below it.",
		Property: Settle | RenderValid,
		Match:    writesACollectionKeyUnderAHeadComment,
	},
	{
		Name: "parse/an-explicit-key-inside-an-explicit-key-is-refused",
		Pin:  "TestDefectAnExplicitKeyInsideAnExplicitKeyIsRefused",
		Reason: "`?` over `  ? a` over `  : 0` over `: v` is refused with `unexpected scalar value " +
			"type`. The key of an explicit entry is s-l+block-indented(n, block-out), which is any " +
			"block node -- a mapping written the long way included.\n\n" +
			"The same key written any other way reads: `?` over `  a: 0` over `: v` gives " +
			"{\"map[a:0]\": \"v\"}, and so do `? {a: 0}` and a sequence below the indicator. So it is " +
			"the nesting of the two `?` and nothing else.\n\n" +
			"Reached on 2026-09-07, when Keys began drawing a collection. Before that no generated " +
			"document held a collection key at all, and yamlcorpus's census reports the YAML Test " +
			"Suite holds no nested explicit key either -- so nothing on either side had provoked it.",
		Property: Parses | Decode | Render | Settle | CommentsKept | RenderValid | DecodeTyped,
		Match:    writesAnExplicitKeyInsideAnExplicitKey,
	},
	{
		Name: "decode/a-merge-key-written-the-long-way-does-not-merge",
		Pin:  "TestDefectAMergeKeyWrittenTheLongWayDoesNotMerge",
		Reason: "A `<<` entry written `? <<` over `: *a` is read as an ordinary key named `<<`, where " +
			"the same entry written `<<: *a` merges. The 1.1 merge type names the key and says nothing " +
			"about how it is written, and the two are the same key node.\n\n" +
			"go.yaml.in/yaml/v3 v3.0.5 merges both, in block and in flow. libfyaml 1.0.0b1 is not an " +
			"oracle here: it resolves no merge at all and hands `<<` back as a member name.\n\n" +
			"A tag on the mapping makes no difference -- `!foo` and `!!map` over the plain form both " +
			"merge, over the long form neither does -- so it is the key's presentation and nothing else.\n\n" +
			"And in flow the two decode paths disagree, which is the worse half. `{? <<: {x: 1}, y: 2}` " +
			"reads {\"<<\": {x: 1}, y: 2} into an `any` and {x: 1, y: 2} into a typed map: the walk does " +
			"not merge it and the tree does. In block, `? <<` over `: {x: 1}`, both agree and neither " +
			"merges. So a caller's answer depends on the destination they chose, which is why this " +
			"claims DecodeTyped as well.\n\n" +
			"⚠️ **codec.ToJSON writes malformed output for it**, which is sharper than the " +
			"disagreement. It walks, so it does not merge -- and it emits the key with no value at " +
			"all: `{? <<: {x: 1}, y: 2}` converts to `{\"\",\"y\":2}`, which no JSON parser reads. " +
			"Found by TestToJSONMatchesTheValueConverter once the corpus grew to 3,000 documents.\n\n" +
			"Found on 2026-09-07 by the merge axis on its first run; the flow half by " +
			"TestDecodingIntoAGoTypeGivesTheSameValue rather than by the value properties.",
		Property: Decode | DecodeTyped,
		Match:    writesAMergeKeyTheLongWay,
	},
	{
		Name: "parse/a-document-suffix-mishandles-a-propertied-block-scalar",
		Pin:  "TestDefectADocumentSuffixMishandlesAPropertiedBlockScalar",
		Reason: "A bare document after a `...` suffix, whose root is a block scalar carrying an anchor " +
			"or a tag and written with an indentation indicator, loses one column of its content. " +
			"`a: 1` over `...` over `&a1 |2-` over two spaces reads \"\" where the same document " +
			"after `---` reads \" \", and `&a1 |2-` over `  x` reads \"x\" where `---` gives " +
			"\" x\".\n\n" +
			"Three things are needed. The suffix: the `---` spelling is right. The property: " +
			"`...` over `|2-` over two spaces reads \" \" correctly, and an anchor or a tag in front " +
			"of the scalar is what loses the column. The indicator: `|-` reads the same both ways.\n\n" +
			"The reference parser settles which side is right, and it is `---`: it emits " +
			"`=VAL &a1 | ` for both separators, the content being everything after the `|`. That is " +
			"8.1.1.1 with l-bare-document's n of -1, so `|2-` at the root puts its content at " +
			"column 1.\n\n" +
			"⚠️ Do not reach for libfyaml or go.yaml.in/yaml/v3 here. Both strip one column too many " +
			"from *every* root block scalar with an indicator -- `|2-` over `  x` reads \"x\" in " +
			"both, where the reference parser and this library read \" x\" -- so on this question " +
			"they agree with each other and with neither the grammar nor the specification. It is a " +
			"content question, and the reference parser is the source that answers one.\n\n" +
			"A second symptom, same suffix and same property: a valid stream is refused. " +
			"`&a3 a: 1` over `...` over `&a1 >-` over ` -` reports `value is not allowed in this " +
			"context` at the block scalar's content, and the `---` spelling of it reads. It takes " +
			"an anchor on each side -- `a: 1` over `...` over `&a1 >-` reads, and so does " +
			"`&a3 a: 1` over `...` over `>-` -- and the second one has to stand on a block scalar, " +
			"since `&a1 x` reads. The reference parser emits " +
			"`+MAP =VAL &a3 :a =VAL :1 -MAP -DOC ... +DOC =VAL &a1 >-` and grammar.NewRecognizer " +
			"accepts it.\n\n" +
			"Found on 2026-09-13 by the stream axis, on its first deep run.",
		Property: StreamDecode,
		Match:    writesAPropertiedRootBlockScalarAfterASuffix,
	},
	{
		Name: "render/a-blank-line-before-a-comment-survives-one-rendering-and-not-the-next",
		Pin:  "TestDefectABlankLineBeforeACommentDoesNotSettle",
		Reason: "A blank line written before a comment is kept by the first rendering and dropped by " +
			"the second, so the rendering never settles. `a:` over ` - x` over a blank line over " +
			"`# c` over `b: 1` renders to `a:` over `- x` over a blank over `# c` over `b: 1`, and " +
			"that renders again without the blank.\n\n" +
			"Three things are needed. The nested sequence has to be written at an indentation the " +
			"renderer does not use -- already at column 1 it settles on the first pass, dropping the " +
			"blank straight away. A nested mapping in the same place settles, re-indented and blank " +
			"kept. And an entry has to follow the comment: without `b: 1` it settles.\n\n" +
			"Only the rendering wobbles: the value is the same every time, and no comment is lost -- " +
			"just the blank line before one. So this claims Settle alone.\n\n" +
			"The predicate is narrower than the defect: it matches the shape Style.Chomping's padding " +
			"reaches, which is how it was found, and a document that writes a blank line some other " +
			"way would fail the property rather than be excused. Widen it then.",
		Property: Settle,
		Match:    writesABlankLineBeforeAComment,
	},
	{
		Name: "parse/a-comment-on-an-explicit-keys-colon-line-is-dropped",
		Pin:  "TestDefectASecondCommentOnAnExplicitKeysColonLineIsDropped",
		Reason: "A comment written on the `:` line of an entry written the long way, with the value " +
			"below it, is lost.\n\n" +
			"Nothing reaches the tree: codec.CommentToMap comes back empty for `? a` over `: # c3` over " +
			"`  v`, where every shape that keeps the comment fills one -- `a: # c3` gives $.a, " +
			"`? a # c3` gives $, `: v # c3` gives $.a. So this is the parse and not the renderer, which " +
			"is why the name says parse.\n\n" +
			"⚠️ **Narrower since 2026-09-12, and the entry is kept for what is left.** " +
			"newMappingValueNode returned early for every explicit key, on the reading that a comment " +
			"on the token it was handed was the key's own. That holds where parseMapKeyValue hands the " +
			"key's own last token over; it does not where the `:` is a token of its own. `? a` over " +
			"`: # c3` over `  v` now renders `? a` over `: v # c3`, which is where the short form puts " +
			"the same comment, and TestFixedACommentOnAnExplicitKeysColonLineIsKept holds it.\n\n" +
			"What still diverges is a *second* comment: one on the `:` line and a head comment under " +
			"it. `? a` over `: # c4` over `  # c5` over `  - 1` keeps c4 and loses c5, and so does the " +
			"same document with a scalar value. A nested mapping keeps both. The `:` line comment goes " +
			"on the value now and the head comment has nowhere left to go, so this is what the fix " +
			"leaves rather than what it missed.\n\n" +
			"TestRenderKeepsEveryComment draws it 19 times in 327, so the predicate stays as it was: " +
			"it matched the family and one member of it is closed.\n\n" +
			"The value reads correctly in every case, so this claims CommentsKept alone.",
		Property: CommentsKept,
		Match:    writesACommentOnAnExplicitColonLine,
	},
}

// writesAPropertiedRootBlockScalarAfterASuffix reports whether the style
// separates a stream with "..." and the value is a root block scalar carrying a
// property.
//
// It asks blockScalarIn whether the string becomes a block scalar rather than
// approximating it. The indicator is not required: it decides which of the two
// symptoms shows -- a column dropped with one, a refusal without -- and both
// are this entry.
func writesAPropertiedRootBlockScalarAfterASuffix(v Value, st Style) bool {
	if !st.DocumentSuffix {
		return false
	}

	var propertied bool

	for {
		switch n := v.(type) {
		case Anchored:
			propertied, v = true, n.V
		case Tagged:
			propertied, v = true, n.V
		default:
			text, isText := v.(Str)

			return propertied && isText && blockScalarIn(text.V, st)
		}
	}
}

// peelProperties returns the node an anchor and a tag decorate.
func peelProperties(v Value) Value {
	for {
		switch n := v.(type) {
		case Anchored:
			v = n.V
		case Tagged:
			v = n.V
		default:
			return v
		}
	}
}

// writesABlankLineBeforeAComment reports whether st pads a block scalar with
// blank lines in a document that also writes comments above its entries.
func writesABlankLineBeforeAComment(v Value, st Style) bool {
	if st.Chomping != ChompPadded {
		return false
	}

	if st.Comments != HeadComments && st.Comments != AllComments {
		return false
	}

	return holdsAPaddedBlockScalar(v, st)
}

func holdsAPaddedBlockScalar(v Value, st Style) bool {
	switch n := v.(type) {
	case Str:
		// Only "-" and clip are padded, and "+" is what a value with two or
		// more trailing breaks needs.
		return blockScalarIn(n.V, st) && len(n.V)-len(strings.TrimRight(n.V, "\n")) < 2
	case Seq:
		return slices.ContainsFunc(n.Items, func(item Value) bool {
			return holdsAPaddedBlockScalar(item, st)
		})
	case Map:
		for _, p := range n.Pairs {
			if holdsAPaddedBlockScalar(p.Val, st) {
				return true
			}
		}
	case Anchored:
		return holdsAPaddedBlockScalar(n.V, st)
	case Alias:
		return holdsAPaddedBlockScalar(n.V, st)
	case Tagged:
		return holdsAPaddedBlockScalar(n.V, st)
	}

	return false
}

// writesACommentOnAnExplicitColonLine reports whether st writes an entry the
// long way, with a line comment, over a value that may go on its own line.
func writesACommentOnAnExplicitColonLine(v Value, st Style) bool {
	if !st.ExplicitKeys || (st.Comments != LineComments && st.Comments != AllComments) {
		return false
	}

	return holdsAValueBelowItsColon(v, st)
}

func holdsAValueBelowItsColon(v Value, st Style) bool {
	switch n := v.(type) {
	case Map:
		for _, p := range n.Pairs {
			if goesOnItsOwnLine(p.Val, st) || holdsAValueBelowItsColon(p.Val, st) {
				return true
			}
		}
	case Seq:
		return slices.ContainsFunc(n.Items, func(item Value) bool {
			return holdsAValueBelowItsColon(item, st)
		})
	case Anchored:
		return holdsAValueBelowItsColon(n.V, st)
	case Alias:
		return holdsAValueBelowItsColon(n.V, st)
	case Tagged:
		return holdsAValueBelowItsColon(n.V, st)
	}

	return false
}

// goesOnItsOwnLine reports the values that leave the ":" line empty behind
// them: a collection with something in it, a block scalar, and a null the style
// spells as nothing at all.
func goesOnItsOwnLine(v Value, st Style) bool {
	switch n := v.(type) {
	case Null:
		return st.NullSpelling == ""
	case Seq:
		return len(n.Items) > 0
	case Map:
		return len(n.Pairs) > 0
	case Str:
		return blockScalarIn(n.V, st)
	case Anchored:
		return goesOnItsOwnLine(n.V, st)
	case Alias:
		return goesOnItsOwnLine(n.V, st)
	case Tagged:
		return goesOnItsOwnLine(n.V, st)
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

// holdsAMergeKey reports whether v writes a "<<" entry anywhere.
func holdsAMergeKey(v Value) bool {
	switch n := v.(type) {
	case Map:
		for _, p := range n.Pairs {
			if _, isMerge := p.Key.(MergeKey); isMerge {
				return true
			}

			if holdsAMergeKey(p.Key) || holdsAMergeKey(p.Val) {
				return true
			}
		}
	case Seq:
		return slices.ContainsFunc(n.Items, holdsAMergeKey)
	case Anchored:
		return holdsAMergeKey(n.V)
	case Alias:
		return holdsAMergeKey(n.V)
	case Tagged:
		return holdsAMergeKey(n.V)
	}

	return false
}

// writesAMergeKeyTheLongWay reports whether a "<<" entry is written "? <<" over
// ": *a" rather than "<<: *a".
//
// Style.ExplicitKeys decides it for every entry of the document, so the two
// halves are the style asking for the long form and the value holding a merge.
func writesAMergeKeyTheLongWay(v Value, st Style) bool {
	return st.ExplicitKeys && holdsAMergeKey(v)
}

// sharesAKey reports whether a "<<" value is a sequence holding two mappings
// that name the same key.
func sharesAKey(v Value) bool {
	seq, isSeq := v.(Seq)
	if !isSeq {
		return false
	}

	seen := make(map[string]struct{})

	for _, item := range seq.Items {
		for _, one := range mergedMappings(item) {
			for k := range one {
				if _, held := seen[k]; held {
					return true
				}

				seen[k] = struct{}{}
			}
		}
	}

	return false
}

// holdsA reports whether a value of kind T stands anywhere in v.
func holdsA[T Value](v Value) bool {
	if _, is := v.(T); is {
		return true
	}

	switch n := v.(type) {
	case Map:
		for _, p := range n.Pairs {
			if holdsA[T](p.Key) || holdsA[T](p.Val) {
				return true
			}
		}
	case Seq:
		return slices.ContainsFunc(n.Items, holdsA[T])
	case Anchored:
		return holdsA[T](n.V)
	case Alias:
		return holdsA[T](n.V)
	case Tagged:
		return holdsA[T](n.V)
	}

	return false
}

// writesAnExplicitKeyInsideAnExplicitKey reports whether a mapping stands as a
// key while the style writes every entry the long way.
//
// Both halves are needed. A collection key takes the explicit form whatever the
// style says, so the outer "?" is always there; the inner one is
// Style.ExplicitKeys, which decides how the mapping *inside* the key is written.
// A sequence key nests no "?" and reads correctly.
func writesAnExplicitKeyInsideAnExplicitKey(v Value, st Style) bool {
	return st.ExplicitKeys && holdsAMappingAsAKey(v)
}

// holdsAMappingAsAKey reports whether a mapping stands as a mapping's key.
func holdsAMappingAsAKey(v Value) bool {
	switch n := v.(type) {
	case Map:
		for _, p := range n.Pairs {
			if _, isMap := p.Key.(Map); isMap {
				return true
			}

			if holdsAMappingAsAKey(p.Key) || holdsAMappingAsAKey(p.Val) {
				return true
			}
		}
	case Seq:
		return slices.ContainsFunc(n.Items, holdsAMappingAsAKey)
	case Anchored:
		return holdsAMappingAsAKey(n.V)
	case Alias:
		return holdsAMappingAsAKey(n.V)
	case Tagged:
		return holdsAMappingAsAKey(n.V)
	}

	return false
}

// writesACollectionKeyUnderAHeadComment reports whether a collection key is
// written below its "?" with a head comment above it.
//
// Both halves: the key has to be a collection, or it goes on the "?"s own line
// and there is nothing below the indicator; and the head comment is what puts a
// blank line between the two, which is what the renderer loses the indentation
// over.
func writesACollectionKeyUnderAHeadComment(v Value, st Style) bool {
	return st.Comments.head() && holdsACollectionKey(v)
}

// holdsACollectionKey reports whether a mapping's key is a collection anywhere
// in v.
func holdsACollectionKey(v Value) bool {
	switch n := v.(type) {
	case Map:
		for _, p := range n.Pairs {
			if isCollection(p.Key) {
				return true
			}

			if holdsACollectionKey(p.Key) || holdsACollectionKey(p.Val) {
				return true
			}
		}
	case Seq:
		return slices.ContainsFunc(n.Items, holdsACollectionKey)
	case Anchored:
		return holdsACollectionKey(n.V)
	case Alias:
		return holdsACollectionKey(n.V)
	case Tagged:
		return holdsACollectionKey(n.V)
	}

	return false
}

// writesACollectionKeyAloneInFlow reports whether a flow entry with no value
// carries a collection as its key.
//
// Three halves, and all three are needed: the style has to write a flow
// collection at all, it has to spell an empty value as the key alone, and the
// value has to hold a collection key for there to be one.
func writesACollectionKeyAloneInFlow(v Value, st Style) bool {
	return st.Flow && st.FlowEmpty == FlowNullKeyAlone && holdsACollectionKey(v)
}

// writesATabBeforeAnAnchoredBlockScalar reports whether an anchor is separated
// from a block scalar by a tab.
//
// Three halves: the style separates with tabs, it writes block scalars at all,
// and the value carries an anchor for one to stand on. Wider than the defect,
// which also needs the scalar's content to begin with a character a plain
// scalar may not -- narrowing it that far would mean reproducing the scanner's
// plain-scalar rule in a predicate, and the pin carries the precision.
func writesATabBeforeAnAnchoredBlockScalar(v Value, st Style) bool {
	return st.TabSeparation && (st.Literal || st.Folded) && holdsA[Anchored](v)
}
