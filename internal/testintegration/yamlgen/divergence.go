// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"reflect"
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
		Name: "parse/a-version-directive-spans-the-whole-stream",
		Pin:  "TestDefectAVersionDirectiveSpansTheWholeStream",
		Reason: "A `%YAML` directive is applied to every document of the stream rather than to the one " +
			"it precedes. `%YAML 1.1` over `---` over `a: yes` over `---` over `b: yes` reads true " +
			"twice, where the second document declares nothing and should be read under the core " +
			"schema -- which reads `yes` as the string.\n\n" +
			"**The library disagrees with itself here.** 3.2.2.2 scopes an anchor to its own document " +
			"and it enforces that: `a: &x 1` over `---` over `b: *x` is refused with " +
			"`could not find alias \"x\"`. A `%TAG` handle is scoped the same way, and all four " +
			"sources agree -- `%TAG !e!` over one document leaves `!e!str` undefined in the next. So " +
			"one declaration is scoped and the other is not.\n\n" +
			"⚖️ Ruled by Fred on 2026-09-13: **documents are independent**, which is the stance this " +
			"package already takes on anchors, so this is a defect rather than a question. The field " +
			"is split -- libfyaml 1.0.0b1 spans as this library does, and other implementations treat " +
			"spanning as a bug -- so a laxer reading is a user option to offer later rather than the " +
			"default to keep. 6.8's wording is the dark corner behind it.\n\n" +
			"The predicate asks whether the value means something different under the two readings, " +
			"since a stream whose documents mean the same either way reads correctly however the " +
			"directive is scoped. That costs two emissions, and only on the draws that write a " +
			"directive without a suffix.",
		Property: StreamDecode,
		Match:    writesAStreamUnderOneDirective,
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

// writesAStreamUnderOneDirective reports whether a stream writes a version
// directive that only its first document carries, over a value the two readings
// disagree about.
//
// Both halves are needed. A document after the first declares nothing unless a
// "..." suffix let it and Style.RedeclareDirectives asked -- 9.1.1 puts a
// directive in l-directive-document, which follows a suffix -- so otherwise the
// documents after the first are core documents. And a value the two schemas
// read alike is read correctly however the directive is scoped, so matching it
// would excuse documents that are fine.
//
// The second half is measured rather than approximated: it writes the value
// twice and compares what each says it means. That is two emissions, and the
// cheap half above keeps them off every draw that does not write a directive.
func writesAStreamUnderOneDirective(v Value, st Style) bool {
	if st.Version == "" || (st.DocumentSuffix && st.RedeclareDirectives) {
		return false
	}

	under, core := st, st
	core.Version = ""

	return !reflect.DeepEqual(Write(v, under).Means, Write(v, core).Means)
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
