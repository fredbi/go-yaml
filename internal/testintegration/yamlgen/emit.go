// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// Emit writes v as YAML in the presentation st asks for.
//
// Every style must produce a document that reads back as the same Value. That
// is the property; this is the thing that makes it checkable, and it is
// deliberately not the library's own renderer -- the renderer emits one style,
// and one style cannot demonstrate invariance across styles.
//
// Where a style cannot express a value -- a literal block scalar inside a flow
// collection, a block collection nested in flow -- Emit falls back rather than
// failing, so that every pairing yields a document.
func Emit(v Value, st Style) string {
	// A nil feature set: nothing is recorded, so Emit costs exactly what it did
	// before the labels existed. [Write] is the call that wants them.
	e := &emitter{st: st}

	return e.emit(v)
}

// emit is Emit's body, shared with [Write].
func (e *emitter) emit(v Value) string {
	st := e.st

	// A %TAG directive applies to the document the directives end marker opens,
	// so writing one forces the "---" whatever the style asked for. It is
	// written only when the document has a tag to route through the handle:
	// declaring a handle nothing uses is legal and says nothing.
	if st.TagSpelling == SpellHandle && holdsSecondaryTag(v) {
		e.feat.add(FeatureTagDirective)
		e.buf.WriteString("%TAG !" + st.TagHandle + "! " + secondaryPrefix + "\n")
	}

	if st.Markers || e.buf.Len() > 0 {
		e.feat.add(FeatureDocumentMarker)
		e.buf.WriteString("---\n")
	}
	e.root(v)

	out := e.buf.String()
	if st.Break != "" && st.Break != BreakLF {
		// The emitter writes \n throughout and the break is substituted once
		// at the end. Nothing else in the document can hold a raw \n: a break
		// inside a scalar is either escaped by doubleQuote or written as a real
		// break of the block scalar that carries it, and both are meant to
		// become the document's break.
		if strings.Contains(out, "\n") {
			switch st.Break {
			case BreakCRLF:
				e.feat.add(FeatureBreakCRLF)
			case BreakCR:
				e.feat.add(FeatureBreakCR)
			case BreakLF:
			}
		}

		out = strings.ReplaceAll(out, "\n", string(st.Break))
	}

	return out
}

type emitter struct {
	buf strings.Builder
	st  Style
	// feat collects the constructs this document is written with. Nil when
	// nobody asked, which is how Emit stays as cheap as it was.
	feat features
	// reads records how each scalar the readings disagree about was written.
	// Nil on the same terms as feat.
	reads *readings
	// comments numbers the comments as they are written, so that a test can
	// check the same set came back rather than merely counting them.
	comments int

	// The rest is here for [Ledger] predicates, and none of it is worth
	// re-deriving outside the emitter: whether a tag lands in front of an
	// anchor depends on Style.PropertyOrder, on which node the tagger picked
	// and on the position it is written in, and a predicate that reimplemented
	// the three would drift from the emitter it describes.

	// taggedLineEnds counts the nodes whose tag is the last thing on its line.
	taggedLineEnds int
	// propertyLines counts the nodes whose properties went on a line of their
	// own.
	propertyLines int
	// collectionTagAnchors counts the nodes written with `!!seq` or `!!map` in
	// front of an anchor, emptyTagAnchors those written with any tag in front
	// of an anchor and nothing after it, and taggedAnchorNames names every
	// anchor that had a tag written before it.
	collectionTagAnchors int
	emptyTagAnchors      int
	taggedAnchorNames    []string
	// emptyKeyTagAnchors counts the nodes written with a tag ahead of an
	// anchor whose first mapping key is written empty.
	emptyKeyTagAnchors int
}

// comment returns the next comment body. Comments are numbered rather than
// random so that a document is reproducible and a lost comment is identifiable.
func (e *emitter) comment() string {
	e.comments++

	return fmt.Sprintf("# c%d", e.comments)
}

// headComment writes a comment on its own line, above whatever comes next.
func (e *emitter) headComment(indent int) {
	if !e.st.Comments.head() {
		return
	}
	e.feat.add(FeatureCommentAbove)
	e.pad(indent)
	e.buf.WriteString(e.comment())
	e.buf.WriteString("\n")
}

// lineComment writes a comment at the end of the line just written.
//
// Only after a scalar: after a block scalar header it would be read as part of
// the header, and inside a flow collection it would run to the closing bracket.
func (e *emitter) lineComment() {
	if !e.st.Comments.line() {
		return
	}
	e.feat.add(FeatureCommentInline)
	e.buf.WriteString(" ")
	e.buf.WriteString(e.comment())
}

func (e *emitter) root(v Value) {
	if inline, ok := e.inline(v, e.st.flowAt(0)); ok {
		e.buf.WriteString(inline)
		e.lineComment()
		e.buf.WriteString("\n")

		return
	}

	e.block(v, 0, 0)
}

// props are the anchor and the tag written in front of a node.
//
// YAML calls them node properties and lets them appear in either order, so
// which one is written first is [Style.PropertyOrder] and neither changes what
// the document means. Peeling them off in one place is what keeps the three
// positions a node can occupy -- inline, at the head of a block, after a `-` or
// a `key:` -- from each growing their own copy of the rules.
type props struct {
	anchor string
	tag    string
}

// strip peels the properties off v and returns the node they decorate.
//
// Either nesting order is accepted, because the two generators that put them
// there run independently: withAliases wraps first and withTags wraps the
// result, so a tagged anchored node arrives as Anchored{Tagged{...}}.
func strip(v Value) (props, Value) {
	var p props

	for {
		switch n := v.(type) {
		case Anchored:
			p.anchor = n.Name
			v = n.V
		case Tagged:
			p.tag = n.Tag
			v = n.V
		default:
			return p, v
		}
	}
}

func (p props) none() bool { return p.anchor == "" && p.tag == "" }

// spellTag writes a tag the way the style asks for.
//
// Three spellings of the same tag, and the node carries only the tag. A bare
// "!" is left alone: it names no type, so there is nothing to write out in full
// and no handle to route it through. A local "!foo" takes the verbatim form,
// where the URI is the tag itself, and keeps its shorthand under SpellHandle --
// declaring a handle for it would mean a second %TAG line saying something
// different from the first.
func spellTag(tag string, st Style) string {
	suffix, secondary := strings.CutPrefix(tag, "!!")
	if !secondary {
		if tag == TagNone || tag == "" || st.TagSpelling != SpellVerbatim {
			return tag
		}

		return "!<" + tag + ">"
	}

	switch st.TagSpelling {
	case SpellVerbatim:
		return "!<" + secondaryPrefix + suffix + ">"
	case SpellHandle:
		return "!" + st.TagHandle + "!" + suffix
	case SpellShorthand:
		return tag
	default:
		return tag
	}
}

// holdsSecondaryTag reports whether v carries a tag that SpellHandle would
// route through a declared handle, which is what makes the %TAG directive
// necessary rather than decorative.
func holdsSecondaryTag(v Value) bool {
	switch n := v.(type) {
	case Tagged:
		if strings.HasPrefix(n.Tag, "!!") {
			return true
		}

		return holdsSecondaryTag(n.V)
	case Anchored:
		return holdsSecondaryTag(n.V)
	case Seq:
		return slices.ContainsFunc(n.Items, holdsSecondaryTag)
	case Map:
		for _, p := range n.Pairs {
			if holdsSecondaryTag(p.Key) || holdsSecondaryTag(p.Val) {
				return true
			}
		}
	}

	return false
}

// propText writes the properties, and records the ones written tag first.
//
// A tag written before an anchor is dropped, so what a [Ledger] entry needs is
// which anchors were written that way -- and, separately, the two tags that
// make the parse fail outright rather than merely losing the tag.
func (e *emitter) propText(p props) string {
	if p.anchor != "" {
		e.feat.add(FeatureAnchor)
	}

	if p.tag != "" {
		e.feat.add(tagFeature(spellTag(p.tag, e.st)))
	}

	if p.anchor != "" && p.tag != "" && e.st.PropertyOrder == TagFirst {
		e.feat.add(FeatureTagBeforeAnchor)
		e.taggedAnchorNames = append(e.taggedAnchorNames, p.anchor)

		if p.tag == TagSeq || p.tag == TagMap {
			e.collectionTagAnchors++
		}
	}

	return p.text(e.st)
}

// opensOnAnEmptyKey reports whether v is a block mapping whose first entry
// writes no key at all, which happens when the key is null and the style spells
// null as nothing.
func (e *emitter) opensOnAnEmptyKey(v Value) bool {
	m, ok := v.(Map)
	if !ok || len(m.Pairs) == 0 || e.st.NullSpelling != "" {
		return false
	}

	_, empty := m.Pairs[0].Key.(Null)

	return empty
}

// countEmptyTagAnchor records a tag written ahead of an anchor on a node with
// nothing after it, which is the shape that swallows the rest of the document.
func (e *emitter) countEmptyTagAnchor(p props) {
	if p.anchor != "" && p.tag != "" && e.st.PropertyOrder == TagFirst {
		e.emptyTagAnchors++
	}
}

// text writes the properties in the order the style asks for.
func (p props) text(st Style) string {
	anchor := ""
	if p.anchor != "" {
		anchor = "&" + p.anchor
	}

	parts := []string{anchor, spellTag(p.tag, st)}
	if st.PropertyOrder == TagFirst {
		parts[0], parts[1] = parts[1], parts[0]
	}

	out := parts[0]
	if out != "" && parts[1] != "" {
		out += " "
	}

	return out + parts[1]
}

// inline returns v written on one line, when it can be.
//
// Collections qualify in flow style, and when they are empty: an empty block
// collection has no spelling, so `[]` and `{}` are the only way to write one.
func (e *emitter) inline(v Value, flow bool) (string, bool) {
	return e.inlineWith(v, flow, "")
}

// inlineWith is inline, told which tag the node carries.
//
// Only one tag changes how the node is written. `!!str` says the scalar is a
// string whatever it looks like, so `!!str null` is a plain scalar meaning the
// text "null" -- and without the tag the same three letters are the empty
// value, which is why canPlain refuses them untagged.
func (e *emitter) inlineWith(v Value, flow bool, tag string) (string, bool) {
	switch n := v.(type) {
	case Alias:
		// An alias is always one token, wherever it stands.
		e.feat.add(FeatureAlias)

		return "*" + n.Name, true
	case Anchored, Tagged:
		p, node := strip(v)

		inner, ok := e.inlineWith(node, flow, p.tag)
		if !ok {
			return "", false
		}
		if inner == "" {
			// A property on an empty node is the property and nothing else; a
			// space after it would be trailing whitespace with no content
			// behind it.
			if p.tag != "" {
				e.taggedLineEnds++
			}
			e.countEmptyTagAnchor(p)

			return e.propText(p), true
		}

		return e.propText(p) + " " + inner, true
	case Seq:
		if flow || len(n.Items) == 0 {
			return e.flowSeq(n), true
		}

		return "", false
	case Map:
		if flow || len(n.Pairs) == 0 {
			return e.flowMap(n), true
		}

		return "", false
	case Str:
		// A literal block scalar is the one scalar that is not one line.
		if !flow && e.blockScalar(n.V) {
			return "", false
		}

		out := e.scalarString(n.V, flow, tag == TagStr)

		// A tag settles the type, so only an untagged scalar resolves by its
		// spelling. scalarString hands back the text unchanged exactly when it
		// wrote it plain; every quoting adds delimiters.
		if tag == "" {
			e.reads.sawScalar(n.V, out == n.V)
		}

		return out, true
	default:
		return e.simpleScalar(v, flow), true
	}
}

// block writes v starting at the given indentation, on its own lines.
//
// depth counts collections from the root, so that Style.FlowFrom can say where
// the document changes over. It tracks indent whenever Style.Indent is 1 and
// parts company from it otherwise, which is why both are carried.
func (e *emitter) block(v Value, indent, depth int) {
	switch n := v.(type) {
	case Anchored, Tagged:
		p, node := strip(v)

		// A block scalar takes its properties in front of the header, where the
		// header still ends the line. A block collection cannot: its first line
		// belongs to its first entry, so the properties take a line of their
		// own.
		e.pad(indent)

		if p.anchor != "" && p.tag != "" && e.st.PropertyOrder == TagFirst && e.opensOnAnEmptyKey(node) {
			e.emptyKeyTagAnchors++
		}

		e.buf.WriteString(e.propText(p))

		if s, ok := node.(Str); ok && e.blockScalar(s.V) {
			e.buf.WriteString(" ")
			e.literal(s.V, indent+e.st.Indent, e.st.Indent+1)

			return
		}

		e.buf.WriteString("\n")
		e.block(node, indent, depth)
	case Seq:
		for _, item := range n.Items {
			e.headComment(indent)
			e.pad(indent)
			e.buf.WriteString("-")
			e.child(item, indent, depth)
		}
	case Map:
		for _, p := range n.Pairs {
			e.headComment(indent)
			e.pad(indent)
			e.buf.WriteString(e.keyIn(p.Key, false))
			e.buf.WriteString(":")
			e.child(p.Val, indent, depth)
		}
	case Str:
		e.pad(indent)
		// The content of a block scalar is indented relative to the header, so
		// it cannot sit at the header's own column -- at the root that would be
		// column zero, which is not indentation at all.
		e.literal(n.V, indent+e.st.Indent, e.st.Indent+1)
	default:
		e.pad(indent)
		e.buf.WriteString(e.simpleScalar(v, false))
		e.buf.WriteString("\n")
	}
}

// child writes the value of a mapping pair or a sequence entry, having already
// written the `-` or the `key:` it belongs to.
//
// depth is the depth of the collection this entry belongs to, so the value
// itself sits one deeper.
func (e *emitter) child(v Value, indent, depth int) {
	flow := e.st.flowAt(depth + 1)

	// The properties stay on the line that introduced the entry, whatever the
	// value turns out to need: `k: &a !!seq` then the collection below it, or
	// `k: &a !!str |` then the scalar's content.
	//
	// Style.PropertyLine is the other placement, and it only works in block
	// context: a property on its own line above a value that is written on the
	// entry's own line would be a property with nothing after it.
	p, node := strip(v)
	v = node

	head := ""
	if !p.none() {
		head = " " + e.propText(p)
	}

	if s, ok := v.(Str); ok && !flow && e.blockScalar(s.V) {
		e.buf.WriteString(head)
		e.buf.WriteString(" ")
		e.literal(s.V, indent+e.st.Indent, e.st.Indent)

		return
	}

	// Properties above the value, on a line of their own, when the value is
	// going to occupy lines of its own anyway.
	if e.st.PropertyLine && !p.none() && !flow {
		if _, ok := e.inlineWith(v, flow, p.tag); !ok {
			e.feat.add(FeaturePropertyLine)
			e.propertyLines++
			e.lineComment()
			e.buf.WriteString("\n")
			e.pad(indent + e.st.Indent)
			e.buf.WriteString(e.propText(p))
			e.buf.WriteString("\n")
			e.block(v, indent+e.st.Indent, depth+1)

			return
		}
	}

	e.buf.WriteString(head)

	if inline, ok := e.inlineWith(v, flow, p.tag); ok {
		// An empty node is written as nothing at all, so the separating space
		// would be the only thing on the line after the `-` or the `key:` --
		// trailing whitespace, and invisible in any failure it caused.
		if inline != "" {
			e.buf.WriteString(" ")
		} else {
			if p.tag != "" {
				e.taggedLineEnds++
			}
			e.countEmptyTagAnchor(p)
		}
		e.buf.WriteString(inline)
		e.lineComment()
		e.buf.WriteString("\n")

		return
	}

	// A comment may sit on the line that introduces a nested block, where the
	// value itself has not been written yet.
	if p.tag != "" {
		e.taggedLineEnds++
	}
	e.lineComment()
	e.buf.WriteString("\n")
	e.block(v, indent+e.st.Indent, depth+1)
}

// literal writes a string as a block scalar, choosing the chomping indicator
// that reproduces its trailing newlines exactly.
// stated is what the indentation indicator should say when the style asks for
// one: the content's column counted from the enclosing node's indentation. A
// block scalar at the root of a document is measured from -1, not from 0, so
// the root passes one more than the column it writes at.
func (e *emitter) literal(s string, indent, stated int) {
	e.reads.sawScalar(s, false)

	if e.folds(s) {
		e.foldedScalar(s, indent, stated)

		return
	}

	e.feat.add(FeatureBlockLiteral)

	body := strings.TrimRight(s, "\n")
	trailing := len(s) - len(body)

	e.buf.WriteString("|")

	if e.st.BlockIndicator {
		e.feat.add(FeatureBlockIndicator)
		e.buf.WriteString(itoa(stated))
	}

	switch trailing {
	case 0:
		e.buf.WriteString("-")
	case 1:
	default:
		e.buf.WriteString("+")
	}

	e.buf.WriteString("\n")

	for _, line := range strings.Split(body, "\n") {
		if line != "" {
			e.pad(indent)
			e.buf.WriteString(line)
		}
		e.buf.WriteString("\n")
	}

	// Clip already wrote the one trailing newline; keep needs the rest.
	for range max(trailing-1, 0) {
		e.buf.WriteString("\n")
	}
}

func (e *emitter) flowSeq(n Seq) string {
	e.feat.add(FeatureFlowCollection)

	items := make([]string, 0, len(n.Items))
	for _, item := range n.Items {
		if pair, ok := e.flowPair(item); ok {
			items = append(items, pair)

			continue
		}
		s, _ := e.inline(item, true)
		items = append(items, s)
	}

	return "[" + strings.Join(items, ", ") + "]"
}

// flowPair writes a one-entry mapping inside a flow sequence without its
// braces, as the `b: c` in [a, b: c].
//
// The braces are optional there and nowhere else, so this shape appears in no
// other position in the grammar. It carries the same meaning either way, which
// keeps it an invariance case rather than a second value.
func (e *emitter) flowPair(v Value) (string, bool) {
	if !e.st.FlowPairs {
		return "", false
	}

	n, ok := v.(Map)
	if !ok || len(n.Pairs) != 1 {
		return "", false
	}

	val, ok := e.inline(n.Pairs[0].Val, true)
	if !ok {
		return "", false
	}

	e.feat.add(FeatureFlowPair)

	return e.keyIn(n.Pairs[0].Key, true) + ": " + val, true
}

func (e *emitter) flowMap(n Map) string {
	e.feat.add(FeatureFlowCollection)

	pairs := make([]string, 0, len(n.Pairs))
	for _, p := range n.Pairs {
		key := e.keyIn(p.Key, true)

		if _, empty := p.Val.(Null); empty {
			switch e.st.FlowEmpty {
			case FlowNullEmpty:
				// The space after the colon is not optional: without it, `p:,`
				// puts the colon inside the plain scalar rather than between
				// the key and its value.
				e.feat.add(FeatureFlowEmptyValue)
				pairs = append(pairs, key+": ")

				continue
			case FlowNullKeyAlone:
				e.feat.add(FeatureFlowKeyAlone)
				pairs = append(pairs, key)

				continue
			case FlowNullSpelled:
			}
		}

		v, _ := e.inline(p.Val, true)
		pairs = append(pairs, key+": "+v)
	}

	return "{" + strings.Join(pairs, ", ") + "}"
}

// keyIn writes a mapping key.
//
// A key is a node, so it is written the way the same node would be written as a
// value -- which is what keeps "1:" and "\"1\":" apart, and what makes
// canPlain's refusal of every numeric spelling load-bearing rather than merely
// conservative: Str{"1"} has to reach the document quoted or it resolves to the
// integer and becomes a different key.
func (e *emitter) keyIn(k Value, flow bool) string {
	s, ok := k.(Str)
	if !ok {
		// Null, Bool, Int and Float, whose spelling has no choices beyond the
		// ones Style names. A Null key under the empty spelling writes nothing
		// at all, which is the ": a" shape.
		return e.simpleScalar(k, flow)
	}

	// Deliberately not inline(): a key is written on the line that introduces
	// the entry, so it can never be a block scalar however Style.Literal is
	// set.
	out := e.scalarString(s.V, flow, false)
	e.reads.sawScalar(s.V, out == s.V)

	return out
}

// simpleScalar writes the scalars whose spelling has no interesting choices
// beyond the ones Style names.
func (e *emitter) simpleScalar(v Value, flow bool) string {
	switch n := v.(type) {
	case Null:
		// The empty spelling of null is a block-context spelling: it relies on
		// there being nothing after the `:` or the `-`. Inside a flow
		// collection the same emptiness runs into the next comma.
		if flow && e.st.NullSpelling == "" {
			e.feat.add(FeaturePlain)

			return "null"
		}

		if e.st.NullSpelling != "" {
			e.feat.add(FeaturePlain)
		}

		return e.st.NullSpelling
	case Bool:
		e.feat.add(FeaturePlain)

		return e.st.BoolSpelling(n.V)
	case Int:
		e.feat.add(FeaturePlain)

		return strconv.Itoa(n.V)
	case BigInt:
		e.feat.add(FeaturePlain)
		e.feat.add(FeatureValueBigInt)

		return n.V.String()
	case BigFloat:
		e.feat.add(FeaturePlain)
		e.feat.add(FeatureValueBigFloat)

		// The shortest text that reads back as the same value at the precision
		// a big.Float carries, which is what the library parses it into.
		return n.V.Text('g', -1)
	case Float:
		e.feat.add(FeaturePlain)

		// The infinities and NaN are spelled the way YAML spells them. Go
		// prints "+Inf" and "NaN", which this library reads as strings.
		//
		// "+.inf" is deliberately not written for positive infinity: §10.3.2
		// admits the sign and this library reads "+.inf" as a string, which is
		// a recorded departure rather than something to generate around.
		switch {
		case math.IsInf(n.V, 1):
			e.feat.add(FeatureValueFloatSpecial)

			return ".inf"
		case math.IsInf(n.V, -1):
			e.feat.add(FeatureValueFloatSpecial)

			return "-.inf"
		case math.IsNaN(n.V):
			e.feat.add(FeatureValueFloatSpecial)

			return ".nan"
		}

		// Never exponent form. The library reads "1e3" as the float 1000, so an
		// exponent would round-trip -- but it would also cross the spelling
		// axis this emitter does not have yet, and a float written two ways is
		// that axis's question rather than this one's.
		s := strconv.FormatFloat(n.V, 'f', -1, 64)
		if !strings.Contains(s, ".") {
			// An integral float formats without a point, and a number without
			// a point reads back as an integer.
			s += ".0"
		}

		return s
	case Str:
		return e.scalarString(n.V, flow, false)
	default:
		panic(fmt.Sprintf("yamlgen: unknown value %T", v))
	}
}

// scalarString writes a string in the quoting the style asks for, falling back
// to double quotes, which can express anything.
func (e *emitter) scalarString(s string, flow, strTagged bool) string {
	switch e.st.Quoting {
	case QuotePlain:
		if canPlain(s, strTagged) {
			e.feat.add(FeaturePlain)

			return s
		}

		e.feat.add(FeatureQuotedDouble)

		return doubleQuote(s)
	case QuoteSingle:
		if canSingle(s) {
			e.feat.add(FeatureQuotedSingle)

			return "'" + strings.ReplaceAll(s, "'", "''") + "'"
		}

		e.feat.add(FeatureQuotedDouble)

		return doubleQuote(s)
	default:
		e.feat.add(FeatureQuotedDouble)

		return doubleQuote(s)
	}
}

func (e *emitter) pad(n int) { e.buf.WriteString(strings.Repeat(" ", n)) }

// plainSafe is deliberately narrower than YAML allows.
//
// It requires a leading letter or underscore, which rules out every numeric
// spelling, every indicator character and the document markers in one stroke.
// Being conservative here can only cost coverage of plain scalars; being wrong
// here would mean generating documents whose expected value we got wrong, and
// then blaming the library for it.
var plainSafe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_ .-]*$`)

// resolving are the plain spellings this library reads as something other than
// a string. Numbers are excluded by plainSafe's leading letter; these are not.
var resolving = map[string]struct{}{
	"null": {}, "Null": {}, "NULL": {},
	"true": {}, "True": {}, "TRUE": {},
	"false": {}, "False": {}, "FALSE": {},
}

// canPlain reports whether s can stand unquoted.
//
// strTagged says the node carries `!!str`, which is what lets the resolving
// spellings through: `!!str null` is the three letters and `null` on its own is
// the empty value. Nothing else in the table is unlocked by it, because
// plainSafe already refuses every numeric spelling on its leading character.
func canPlain(s string, strTagged bool) bool {
	if !plainSafe.MatchString(s) {
		return false
	}
	if _, resolves := resolving[s]; resolves && !strTagged {
		return false
	}
	// A trailing space is not preserved, and " #" opens a comment.
	if strings.HasSuffix(s, " ") || strings.Contains(s, " #") {
		return false
	}

	return true
}

// bom is the byte order mark, which YAML 1.2 admits at the start of a stream
// and nowhere else: nb-char is c-printable less the line breaks and less this.
//
// It is easy to miss, because it is not a control character and so a check for
// those lets it through -- into a single-quoted or block scalar that no reader
// following the spec will accept, whatever this library does with it. Double
// quoting escapes it, which is why that style needs no check.
const bom = '\uFEFF'

// writableRaw reports whether s can stand as itself, unescaped, in a scalar
// whose only structure is its line breaks.
func writableRaw(s string) bool {
	for _, r := range s {
		if r != '\n' && (r == '\r' || r == bom || unicode.IsControl(r)) {
			return false
		}
	}

	return true
}

// canSingle reports whether a single-quoted scalar written on one line
// reproduces s exactly.
//
// Only two things stop it. A line break folds, so a string holding one comes
// back as something else. A character the spec forbids cannot be written raw at
// all, and single quotes escape nothing but the quote itself.
//
// Everything else the quotes take care of, which is the point of them: the
// delimiters are what make leading and trailing whitespace survive, so refusing
// those was refusing the case this style exists to handle. It used to refuse
// them, and tabs, and the empty string -- a third of all drawn strings fell
// back to double quotes for no reason, taking with them exactly the shapes this
// library has had defects in.
func canSingle(s string) bool {
	if strings.ContainsAny(s, "\n\r") {
		return false
	}

	return writableRaw(s)
}

// canLiteral reports whether s can be written as a literal block scalar.
//
// Without an explicit indentation indicator the parser works the indentation
// out from the first content line, so content whose first line is empty, or
// whose lines begin with a space, has nothing to work it out from. The
// indicator states it instead, and those strings become writable -- which is
// the whole reason to generate one.
func canLiteral(s string, indicator bool) bool {
	// A block scalar needs at least one line with something on it. Without one
	// there is no content for the trailing breaks to trail after, and the
	// header cannot say how many of them the value has.
	if strings.TrimRight(s, "\n") == "" {
		return false
	}

	if !indicator {
		if strings.HasPrefix(s, "\n") {
			return false
		}

		for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
			if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
				return false
			}
			if line != "" && strings.TrimSpace(line) == "" {
				return false
			}
		}
	}

	return writableRaw(s)
}

// doubleQuote writes s as a double-quoted scalar, which can express any string.
func doubleQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')

	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r == bom || unicode.IsControl(r) {
				fmt.Fprintf(&b, `\u%04X`, r)

				continue
			}
			b.WriteRune(r)
		}
	}

	b.WriteByte('"')

	return b.String()
}

// folds reports whether s should be written as a folded block scalar.
//
// Folded is preferred over literal where both can express the value, so that
// the axis is actually exercised: literal is the default everywhere else.
func (e *emitter) folds(s string) bool {
	return e.st.Folded && canFolded(s)
}

// blockScalar reports whether s can be written as a block scalar at all, in
// whichever of the two styles this presentation allows.
func (e *emitter) blockScalar(s string) bool {
	return e.folds(s) || (e.st.Literal && canLiteral(s, e.st.BlockIndicator))
}

// foldedScalar writes a string as a folded block scalar.
//
// Folding joins two lines with a space and turns n+1 breaks into n, so a break
// in the value is written as a blank line and the lines of the value end up
// separated by one. That is the whole trick, and it is why canFolded refuses
// any value whose own lines are empty: those would need a run of breaks one
// longer again, and the arithmetic stops being obvious enough to trust.
func (e *emitter) foldedScalar(s string, indent, stated int) {
	body := strings.TrimRight(s, "\n")

	trailing := len(s) - len(body)

	e.feat.add(FeatureBlockFolded)
	e.buf.WriteString(">")

	if e.st.BlockIndicator {
		e.feat.add(FeatureBlockIndicator)
		e.buf.WriteString(itoa(stated))
	}

	switch trailing {
	case 0:
		e.buf.WriteString("-")
	case 1:
	default:
		e.buf.WriteString("+")
	}

	e.buf.WriteString("\n")

	for i, line := range strings.Split(body, "\n") {
		if i > 0 {
			// The blank line that folds away into the break it stands for.
			e.buf.WriteString("\n")
		}
		e.pad(indent)
		e.buf.WriteString(line)
		e.buf.WriteString("\n")
	}

	// Clip already wrote the one trailing newline; keep needs the rest.
	for range max(trailing-1, 0) {
		e.buf.WriteString("\n")
	}
}

// canFolded reports whether folding can reproduce s exactly.
//
// Deliberately narrow. Folding is the one presentation where a wrong emitter
// writes a document that means something else while looking perfectly
// reasonable, so everything whose inverse is not obvious is refused rather
// than guessed at: more-indented lines are not folded at all, trailing spaces
// before a fold are their own question, and a value containing its own blank
// line needs a run of breaks this does not write.
func canFolded(s string) bool {
	body := strings.TrimRight(s, "\n")
	if body == "" || strings.Contains(body, "\n\n") {
		return false
	}

	for _, line := range strings.Split(body, "\n") {
		if line == "" || strings.TrimSpace(line) != line {
			return false
		}
	}

	return writableRaw(s)
}
