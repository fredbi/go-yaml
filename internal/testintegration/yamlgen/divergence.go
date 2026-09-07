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
		Pin:  "TestDefectAMappingKeyWrittenEmptyIsRefused",
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
		Name: "parse/a-local-tag-before-an-anchor-does-not-type-its-scalar",
		Pin:  "TestDefectALocalTagBeforeAnAnchorDoesNotTypeItsScalar",
		Reason: "A local or non-specific tag written *before* an anchor stops typing its scalar, and " +
			"the scalar resolves by the schema as though it carried no tag. `!foo true` reads " +
			"\"true\"; `!foo &a1 true` reads the boolean true. `!foo &a1 1` reads uint64(1) where " +
			"`!foo 1` reads \"1\".\n\n" +
			"Three things narrow it. The order: `&a1 !foo true` reads \"true\", so writing the " +
			"anchor first keeps the tag. The tag: `!!str &a1 true` reads \"true\", so a `!!` " +
			"shorthand is unaffected -- only a local tag, its verbatim spelling `!<!foo>`, and the " +
			"non-specific `!` lose it. The context: a block mapping, a block sequence and a flow " +
			"mapping all do it, and so does the document root.\n\n" +
			"Same family as parse/a-tag-not-written-as-a-shorthand-does-not-type-its-scalar and the " +
			"tag-before-anchor entries in yamlgen.Strict: which spelling and which order the two " +
			"properties are written in decides whether the tag reaches the scalar.\n\n" +
			"Found on 2026-09-13, when the yamlcorpus/25 draw put Style.PropertyOrder and a local " +
			"tag on one anchored scalar.\n\n" +
			"Decode and Render, and Render because the value is already wrong when it is first read: " +
			"the parse renders the document back byte for byte.",
		Property: Decode | DecodeTyped | Render,
		Match:    writesALocalTagBeforeAnAnchor,
	},
	{
		Name: "parse/a-version-directive-resolves-the-root-block-scalar-it-opens",
		Pin:  "TestDefectAVersionDirectiveResolvesTheRootBlockScalarItOpens",
		Reason: "A `%YAML` directive over a document whose body is a block scalar makes the parse fail " +
			"when the scalar's content is a word the schema would resolve. `%YAML 1.1` over `---` " +
			"over `>-` over ` null` reports `unexpected token. required string token`; the same three " +
			"lines without the directive read the string \"null\".\n\n" +
			"The content decides it, and only when it is one whole token: `null`, `~`, `True`, `yes`, " +
			"`5` and `1.5` are all refused, where `x`, `x y` and `null x` read. The version does not: " +
			"`%YAML 1.2` refuses it too. Nor does any other directive -- a `%TAG` line over the same " +
			"document reads, and so does a bare `---`. And the body has to be the root: `k: >-` over " +
			"`  null` reads under the directive.\n\n" +
			"A block scalar is a string under every schema, so there is nothing here to resolve. " +
			"internal/lab's resolvesDifferentlyOnPurpose describes the machinery: the grouping reads " +
			"one token past the directive to know the directive's own document has ended, and for a " +
			"document whose body is a bare scalar that token is the body. It is cut before the " +
			"directive is read and typed again afterwards -- as a null or a bool or an integer, " +
			"where a block scalar's content is none of those.\n\n" +
			"libfyaml 1.0.0b1 reads every one of them as the string, and the reference parser passes " +
			"them. Found on 2026-09-07 by Style.Version, on its first run.\n\n" +
			"📌 It is the one document that provokes `unexpected token. required string token`, which " +
			"stream 8 lists as unreached. Like `unexpected scalar value`, a Refusals entry for it " +
			"would pin a bug rather than a rule.",
		Property: Parses | Decode | Render | Settle | CommentsKept,
		Match:    writesAResolvingRootBlockScalarUnderADirective,
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
		Pin:  "TestDefectACommentOnAnExplicitKeysColonLineIsDropped",
		Reason: "A comment written on the `:` line of an entry written the long way, with the value " +
			"below it, is lost. `? a` over `: # c3` over `  v` renders back as `? a` over `: v`.\n\n" +
			"The short form keeps it: `a: # c3` over `  v` renders `a: v # c3`, moved but not lost. " +
			"So does a comment after the value, `: v # c3`, and one on the `?` line, `? a # c3`. It " +
			"is the `:` line of the long form and nowhere else.\n\n" +
			"Only the comments are lost -- the value reads correctly -- which is why this claims " +
			"CommentsKept alone. Found on 2026-09-11 by Style.ExplicitKeys.\n\n" +
			"📌 It is lost before the tree, so it is not the renderer. Measured on 2026-09-13 with " +
			"codec.CommentToMap: `? a` over `: # c3` over `  v` fills an empty comment map, where " +
			"`a: # c3` over `  v` gives $.a, `? a # c3` gives $ and `: v # c3` gives $.a. Nothing " +
			"reaches the tree to be written out, so the name of this entry moved from render/ to " +
			"parse/.\n\n" +
			"The shape is a `:` with nothing after it on its line, so an empty value counts as much " +
			"as a collection: `?` over `: #c1` loses the comment too. The predicate does not work " +
			"out whether a collection really lands below rather than in flow, since that depends on " +
			"the value's depth and on Style.FlowFrom, so it reports more draws than divergences.",
		Property: CommentsKept,
		Match:    writesACommentOnAnExplicitColonLine,
	},
	{
		Name: "decode/a-binary-tag-cannot-be-read-into-a-go-byte-slice",
		Pin:  "TestDefectABinaryTagCannotBeReadIntoAGoByteSlice",
		Reason: "`!!binary` cannot be read into a Go []byte -- a struct field, a slice element or a " +
			"map value. `a: !!binary aGVsbG8=` into a struct with a []byte field is refused with " +
			"`string was used where sequence is expected`, and so is a map[string][]byte.\n\n" +
			"The same document into an `any` gives []uint8{'h','e','l','l','o'}, and into a *string* " +
			"field it gives \"hello\" -- the decoded bytes, not the base64 text. So the conversion " +
			"is there and the one Go type the tag names is the one it cannot reach.\n\n" +
			"Sibling of decode/an-int-tag-cannot-be-read-into-a-go-integer, and the same shape: the " +
			"`any` path has a case for the type the tag resolves to and the reflection path does " +
			"not. Unlike that one, go.yaml.in/yaml/v3 v3.0.5 refuses it too, with " +
			"`cannot unmarshal !!binary into []uint8` -- so this is an inconsistency inside the " +
			"library rather than a departure from the field. encoding/json reads a base64 string " +
			"into a []byte.\n\n" +
			"Found on 2026-09-13, on the first deep run of the Binary value kind.",
		Property: DecodeTyped,
		Match:    writesABinaryTag,
	},
	{
		Name: "render/a-block-scalar-in-a-sequence-swallows-an-empty-key",
		Reason: "A block scalar written as a nested sequence entry, with an empty key after it, " +
			"renders to text the parser refuses. `a:` over ` - |1-` over `   ` over `:` renders to " +
			"`a:` over `- |2-    :`, and reading that back reports " +
			"`invalid header option: \"2-    :\"`. The scalar's content and the empty key's `:` are " +
			"both written onto the header line, so a valid document renders to one that is not " +
			"YAML.\n\n" +
			"The empty key is what does it: `b: 1` in its place renders correctly, at the same " +
			"column. Nothing else is needed -- the anchor and the whitespace-only content each drop " +
			"out and it still happens, and `|1` chomping the break goes the same way as `|1-`.\n\n" +
			"It claims RenderValid and Settle: the text it writes does not parse, so nothing can be " +
			"read from it or rendered again. The first read is correct.\n\n" +
			"Found on 2026-09-12 at 30,000 draws of TestRenderWritesValidYAML.\n\n" +
			"The predicate is wider than the defect: it asks that the document hold an empty key " +
			"and a block scalar inside a sequence, not that the two land in that order.",
		Property: RenderValid | Settle,
		Match:    writesABlockScalarBesideAnEmptyKey,
	},
}

// writesAResolvingRootBlockScalarUnderADirective reports whether st writes a
// "%YAML" line over a document whose body is a block scalar the schemas would
// resolve if it were plain.
func writesAResolvingRootBlockScalarUnderADirective(v Value, st Style) bool {
	if st.Version == "" {
		return false
	}

	s, text := peelProperties(v).(Str)

	return text && blockScalarIn(s.V, st) && resolvesAlone(strings.TrimRight(s.V, "\n"))
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

// resolvesAlone reports whether a whole text is a token some schema reads as
// something other than a string.
//
// Two rules rather than a copy of every schema's productions. A scalar that
// resolves to a number, a date or a time begins with a digit, a sign, a dot or
// a tilde -- 5, 1.5, -0x1f, .inf, 12:34:56, 2001-12-14 -- so the first
// character answers for all of them without this having to know 1.1's
// sexagesimals. Everything else that resolves is a word, and the two tables
// hold every word both schemas read.
//
// Generous where it is unsure, which is the right side here: the predicate
// excuses a document rather than accusing one.
func resolvesAlone(text string) bool {
	if text == "" || strings.ContainsAny(text, " \n") {
		return false
	}

	if strings.ContainsAny(text[:1], "0123456789-+.~") {
		return true
	}

	if _, core := resolving[text]; core {
		return true
	}

	_, legacy := legacyBooleans[text]

	return legacy
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

// Wider than the defect: a Str whose text resolves to itself reads the same
// either way, and the shape is still matched. The tally says how often it
// actually diverges.
func writesALocalTagBeforeAnAnchor(v Value, st Style) bool {
	if st.PropertyOrder != TagFirst {
		return false
	}

	return holdsALocalTagOnAnAnchoredScalar(v, false)
}

// holdsALocalTagOnAnAnchoredScalar walks v, carrying whether an [Anchored]
// stands above the node being looked at.
//
// The two properties may be written in either nesting order -- the tagger and
// the aliaser each wrap what they are given -- and the emitter writes both on
// one line whichever way round the tree holds them.
func holdsALocalTagOnAnAnchoredScalar(v Value, anchored bool) bool {
	switch n := v.(type) {
	case Anchored:
		return holdsALocalTagOnAnAnchoredScalar(n.V, true)
	case Tagged:
		if anchored && (n.Tag == TagLocal || n.Tag == TagNone) {
			if _, scalar := n.V.(Str); scalar {
				return true
			}
		}

		return holdsALocalTagOnAnAnchoredScalar(n.V, anchored)
	case Alias:
		return false
	case Seq:
		return slices.ContainsFunc(n.Items, func(item Value) bool {
			return holdsALocalTagOnAnAnchoredScalar(item, false)
		})
	case Map:
		for _, p := range n.Pairs {
			if holdsALocalTagOnAnAnchoredScalar(p.Key, false) ||
				holdsALocalTagOnAnAnchoredScalar(p.Val, false) {
				return true
			}
		}
	}

	return false
}

// writesABinaryTag reports whether v holds a [Binary], whose decoded []byte is
// the destination TargetForDecoded then builds.
func writesABinaryTag(v Value, _ Style) bool {
	switch n := v.(type) {
	case Binary:
		return true
	case Tagged:
		return writesABinaryTag(n.V, Style{})
	case Anchored:
		return writesABinaryTag(n.V, Style{})
	case Alias:
		return writesABinaryTag(n.V, Style{})
	case Seq:
		return slices.ContainsFunc(n.Items, func(item Value) bool {
			return writesABinaryTag(item, Style{})
		})
	case Map:
		for _, p := range n.Pairs {
			if writesABinaryTag(p.Key, Style{}) || writesABinaryTag(p.Val, Style{}) {
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

// writesABlockScalarBesideAnEmptyKey reports whether st writes both an empty
// key and a block scalar inside a sequence.
func writesABlockScalarBesideAnEmptyKey(v Value, st Style) bool {
	return writesAnEmptyKey(v, st) && holdsABlockScalarInASequence(v, st)
}

func holdsABlockScalarInASequence(v Value, st Style) bool {
	switch n := v.(type) {
	case Seq:
		return slices.ContainsFunc(n.Items, func(item Value) bool {
			if s, text := unwrapProperties(item).(Str); text && blockScalarIn(s.V, st) {
				return true
			}

			return holdsABlockScalarInASequence(item, st)
		})
	case Map:
		for _, p := range n.Pairs {
			if holdsABlockScalarInASequence(p.Key, st) || holdsABlockScalarInASequence(p.Val, st) {
				return true
			}
		}
	case Anchored:
		return holdsABlockScalarInASequence(n.V, st)
	case Tagged:
		return holdsABlockScalarInASequence(n.V, st)
	case Alias:
		return holdsABlockScalarInASequence(n.V, st)
	}

	return false
}

// unwrapProperties reaches the node an anchor or a tag stands on.
func unwrapProperties(v Value) Value {
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
