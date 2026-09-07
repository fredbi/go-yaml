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
		Name: "parse/a-local-tag-before-an-anchor-does-not-type-its-scalar",
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
		Name: "render/a-comment-on-an-explicit-keys-colon-line-is-dropped",
		Reason: "A comment written on the `:` line of an entry written the long way, with the value " +
			"below it, is lost. `? a` over `: # c3` over `  v` renders back as `? a` over `: v`.\n\n" +
			"The short form keeps it: `a: # c3` over `  v` renders `a: v # c3`, moved but not lost. " +
			"So does a comment after the value, `: v # c3`, and one on the `?` line, `? a # c3`. It " +
			"is the `:` line of the long form and nowhere else.\n\n" +
			"Only the comments are lost -- the value reads correctly -- which is why this claims " +
			"CommentsKept alone. Found on 2026-09-11 by Style.ExplicitKeys.\n\n" +
			"The shape is a `:` with nothing after it on its line, so an empty value counts as much " +
			"as a collection: `?` over `: #c1` loses the comment too. The predicate does not work " +
			"out whether a collection really lands below rather than in flow, since that depends on " +
			"the value's depth and on Style.FlowFrom, so it reports more draws than divergences.",
		Property: CommentsKept,
		Match:    writesACommentOnAnExplicitColonLine,
	},
	{
		Name: "parse/an-anchor-alone-after-an-explicit-key-swallows-what-follows",
		Reason: "An entry written the long way whose value is an anchor and nothing else takes the " +
			"entries after it into itself. `? a` over `: &a1` over `? b` over `: &a2` reads " +
			"{\"a\": {\"b\": null}}, and the renderer writes the nesting back out indented.\n\n" +
			"Where the swallowed entry aliases the anchor the parse stops instead: `? a` over `: &a1` " +
			"over `? b` over `: *a1` reports `alias \"a1\" names an anchor that is not resolved yet`, " +
			"because the anchor is inside the node that is still being built.\n\n" +
			"Three things make it the anchor and the long form together. The short form reads: " +
			"`a: &a1` over `b: &a2` gives two entries. The long form without anchors reads: `? a` " +
			"over `:` over `? b` over `:` gives two entries. And giving the value content reads, " +
			"`: &a1 x`.\n\n" +
			"This is the same shape as decode/a-tag-on-an-empty-value, one property over: a node " +
			"property standing alone after a `:` makes the parser expect a block collection under " +
			"it. libfyaml 1.0.0b1 gives the flat mapping, the reference parser passes it, and " +
			"grammar.NewRecognizer accepts it. Found on 2026-09-11 by Style.ExplicitKeys.\n\n" +
			"The predicate does not ask whether anything follows the anchored entry, so it reports " +
			"more draws than divergences.",
		Property: Parses | Decode | DecodeTyped | Render | Settle | CommentsKept,
		Match:    writesAnAnchorAloneAfterAnExplicitKey,
	},
	{
		Name: "parse/a-quoted-explicit-key-refuses-a-block-scalar-value",
		Reason: "A mapping entry written the long way -- `? key` over `: value` -- is refused when the " +
			"key is quoted and the value is a block scalar. `? \"a\"` over `: >-` over `  x` reports " +
			"`value is not allowed in this context`; `? a` over the same two lines reads, and so does " +
			"the short form `\"a\": >-`.\n\n" +
			"Nothing else narrows it. Single quotes and double quotes both do it, `|` and `>` both do " +
			"it, and it happens at the document root, inside a mapping and inside a sequence entry. " +
			"An anchor or a tag on the value makes no difference. A flow collection, a quoted scalar " +
			"and a plain scalar after the same quoted key all read.\n\n" +
			"Where the block scalar's content begins with a `:` it is worse than refused. `? \"\"` " +
			"over `: >-` over ` : a` reads {\"\": {\"\": \"a\"}} -- a nested mapping, where libfyaml " +
			"1.0.0b1 gives {\"\": \": a\"} -- because the parser takes the `>-` for a plain scalar " +
			"key rather than a block scalar header. A plain scalar cannot begin with `>`, so the tree " +
			"holds a node no document can spell, and the renderer writes `>-: a` back out: text the " +
			"recognizer refuses. That is why this claims RenderValid as well.\n\n" +
			"libfyaml 1.0.0b1 reads every one of them, the reference parser passes them, and " +
			"grammar.NewRecognizer accepts them. Found on 2026-09-11 by Style.ExplicitKeys, on its " +
			"first deep run.\n\n" +
			"The predicate asks the emitter's own blockScalarIn whether the value becomes one, so the " +
			"only place it is wider than the defect is the key: Style.Quoting quotes every string, " +
			"and not every quoted key is one the parser then cannot place.",
		Property: Parses | Decode | Render | Settle | CommentsKept | RenderValid,
		Match:    writesAQuotedExplicitKeyOverABlockScalar,
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
		Name: "decode/a-binary-tag-cannot-be-read-into-a-go-byte-slice",
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

// writesAQuotedExplicitKeyOverABlockScalar reports whether st writes a mapping
// entry the long way, with a quoted key and a value the emitter may write as a
// block scalar.
//
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

// writesAnAnchorAloneAfterAnExplicitKey reports whether st writes an entry the
// long way whose value reaches the document as an anchor and nothing else.
//
// Only a bare Null does, and only when the style spells null as nothing.
func writesAnAnchorAloneAfterAnExplicitKey(v Value, st Style) bool {
	if !st.ExplicitKeys || st.NullSpelling != "" {
		return false
	}

	return holdsAnAnchoredNullValue(v)
}

func holdsAnAnchoredNullValue(v Value) bool {
	switch n := v.(type) {
	case Map:
		for _, p := range n.Pairs {
			if anchoredNull(p.Val) || holdsAnAnchoredNullValue(p.Val) {
				return true
			}
		}
	case Seq:
		return slices.ContainsFunc(n.Items, holdsAnAnchoredNullValue)
	case Anchored:
		return holdsAnAnchoredNullValue(n.V)
	case Alias:
		return holdsAnAnchoredNullValue(n.V)
	case Tagged:
		return holdsAnAnchoredNullValue(n.V)
	}

	return false
}

// anchoredNull reports an anchor standing directly on a node that writes
// nothing. A tag between the two writes itself, so the anchor is no longer
// alone.
func anchoredNull(v Value) bool {
	n, anchored := v.(Anchored)
	if !anchored {
		return false
	}

	_, empty := n.V.(Null)

	return empty
}

// It asks blockScalarIn rather than approximating it, so the only place it is
// wider than the defect is the key: a key the style quotes is not always a key
// the defect needs, since Style.Quoting quotes every string and the defect
// wants one the parser then cannot place.
func writesAQuotedExplicitKeyOverABlockScalar(v Value, st Style) bool {
	if !st.ExplicitKeys {
		return false
	}

	return holdsAQuotedKeyOverAString(v, st)
}

func holdsAQuotedKeyOverAString(v Value, st Style) bool {
	switch n := v.(type) {
	case Map:
		for _, p := range n.Pairs {
			key, text := p.Key.(Str)
			if text && quotedIn(key.V, st) && writesABlockScalar(p.Val, st) {
				return true
			}

			if holdsAQuotedKeyOverAString(p.Val, st) {
				return true
			}
		}
	case Seq:
		return slices.ContainsFunc(n.Items, func(item Value) bool {
			return holdsAQuotedKeyOverAString(item, st)
		})
	case Anchored:
		return holdsAQuotedKeyOverAString(n.V, st)
	case Alias:
		return holdsAQuotedKeyOverAString(n.V, st)
	case Tagged:
		return holdsAQuotedKeyOverAString(n.V, st)
	}

	return false
}

// quotedIn reports whether a key reaches the document in quotes.
func quotedIn(key string, st Style) bool {
	return st.Quoting != QuotePlain || !canPlain(key, false)
}

// writesABlockScalar reports whether the emitter would write v as a block
// scalar under this style, properties and all.
func writesABlockScalar(v Value, st Style) bool {
	switch n := v.(type) {
	case Str:
		return blockScalarIn(n.V, st)
	case Anchored:
		return writesABlockScalar(n.V, st)
	case Alias:
		return writesABlockScalar(n.V, st)
	case Tagged:
		return writesABlockScalar(n.V, st)
	default:
		return false
	}
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

// writesALocalTagBeforeAnAnchor reports whether v holds a node carrying both a
// local or non-specific tag and an anchor, with [TagFirst] writing the tag
// first.
//
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

// writesANonStringKey reports whether v holds a mapping keyed by anything other
// than a string, which is a key no struct tag names.
//
// The style decides one case. A document declaring "%YAML 1.1" resolves "yes:"
// to the boolean key true, so a Str holding one of 1.1's boolean words is a
// non-string key there and a string key everywhere else.
func writesANonStringKey(v Value, st Style) bool {
	switch n := v.(type) {
	case Map:
		for _, p := range n.Pairs {
			if nonStringKey(p.Key, st) || writesANonStringKey(p.Val, st) {
				return true
			}
		}
	case Seq:
		return slices.ContainsFunc(n.Items, func(item Value) bool {
			return writesANonStringKey(item, st)
		})
	case Anchored:
		return writesANonStringKey(n.V, st)
	case Alias:
		return writesANonStringKey(n.V, st)
	case Tagged:
		return writesANonStringKey(n.V, st)
	}

	return false
}

// nonStringKey reports whether a key reaches the decoder as something other
// than a Go string.
func nonStringKey(k Value, st Style) bool {
	s, text := k.(Str)
	if !text {
		return true
	}

	if st.Version != Reading11Version || st.Quoting != QuotePlain {
		return false
	}

	_, legacy := legacyBooleans[s.V]

	return legacy
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
