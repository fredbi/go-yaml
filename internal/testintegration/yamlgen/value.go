// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"pgregory.net/rapid"
)

// Value is a logical YAML value: what a document means, with nothing said about
// how it is written down.
//
// Keeping meaning and presentation apart is the whole design. A generated Value
// can be written many ways, and every one of them has to read back as the same
// Value -- which is a property we can check without a grammar, an oracle, or a
// reference implementation.
type Value interface {
	// Decoded returns what Unmarshal into an `any` should produce for this
	// value.
	Decoded() any
}

type (
	// Null is the empty value.
	Null struct{}
	// Bool is true or false.
	Bool struct{ V bool }
	// Int is an integer.
	Int struct{ V int }
	// Float is a floating point number, the infinities and NaN included.
	//
	// They were excluded for a long time, on the grounds that their spellings
	// were their own conformance question. They are worth having and the
	// question was worth asking: each is a float the library resolves, JSON has
	// no spelling for any of them, and the encoder round-trips them. Three
	// properties rather than one thing to defer.
	//
	// A NaN costs the property tests one helper: it is not equal to itself, so
	// they compare with sameValue rather than with reflect's equality.
	Float struct{ V float64 }
	// BigInt is an integer past what a machine word holds, which this library
	// reads as a *big.Int rather than losing.
	BigInt struct{ V *big.Int }
	// BigFloat is a float past what a float64 holds, read as a *big.Float.
	//
	// The value carries prec 64, because that is what the library's own
	// big.Float carries and the properties compare the two with reflect's
	// equality -- a wider one would be the same number and a different struct.
	BigFloat struct{ V *big.Float }
	// Str is a string, and the interesting one: most of the ways to write a
	// YAML document differently are ways to write a string differently.
	Str struct{ V string }
	// Seq is an ordered list.
	Seq struct{ Items []Value }
	// Map is an ordered set of pairs with distinct keys. Order is kept so that
	// emitting is deterministic; it carries no meaning.
	Map struct{ Pairs []Pair }

	// Anchored is a value carrying a name that an Alias can refer to.
	//
	// The anchor changes nothing about what the value means, which is the point:
	// it is presentation that lives in the tree rather than in the Style,
	// because which node carries it is structural.
	Anchored struct {
		Name string
		V    Value
	}

	// Alias stands in for a value anchored earlier in the same document.
	//
	// It holds the anchored value rather than just the name, so Decoded needs no
	// symbol table and cannot disagree with the document about what the name
	// refers to.
	Alias struct {
		Name string
		V    Value
	}

	// Tagged is a value carrying an explicit tag.
	//
	// Like [Anchored], it lives in the tree rather than in the [Style], because
	// which node carries it is structural. Unlike an anchor it can change what
	// the document means, so only the tags that agree with the value's own kind
	// are generated -- see [TagFor]. `!!str` on a sequence is a document this
	// library refuses and it should, which is a different question from the one
	// this package asks.
	Tagged struct {
		Tag string
		V   Value
	}
)

// Pair is one mapping entry. Keys are strings because that is what decoding
// into an `any` produces, whatever the document said.
// Pair is one mapping entry.
type Pair struct {
	// Key is the node written before the colon.
	//
	// A Value rather than a string, because YAML lets any node be a key and
	// this library reads several of them differently. "1:", "01:" and "0x1:"
	// are one key, "null:", "~:" and a key left empty are another, and a
	// quoted "\"1\":" is a third that collides with the first once resolved.
	// None of that is reachable while a key is Go text.
	Key Value
	// Val is the node written after it.
	Val Value
}

// KeyText is what this library makes of a key when it decodes a mapping into an
// `any`: the canonical spelling of the key's type.
//
// Measured, and the measurement is the whole reason this is a function rather
// than a field. Decoding into an `any` always produces a map[string]any, and
// the key is stringified **from the value the scalar resolves to** rather than
// from the text that was written: ":", "~:", "Null:", "NULL:" and
// "!!null null:" all arrive as "null", and "1:", "01:", "1.0:" and "0x1:" all
// arrive as "1".
//
// That is what makes a key presentation-invariant, and it is why the generator
// can draw a key of any scalar kind and still state what the document means.
//
// ⚠️ Naming per type puts every typed key into the strings' namespace, and a
// map[string]any cannot hold a key twice -- so Str{"1.0"} and Float{1.0} are
// two keys that this library collapses into one, silently. keyFamily keeps the
// generator out of that; yamlcorpus.Departures records it.
// It also means two keys can collide after resolution while looking nothing
// alike -- Int{1} and Str{"1"} are both "1" -- which the library refuses as a
// duplicate and [yamlcorpus.Departures] records as wrong. drawMap keeps out of
// that; breakRules produces it on purpose.
func KeyText(v Value) string {
	switch n := v.(type) {
	case Null:
		return "null"
	case Bool:
		if n.V {
			return "true"
		}

		return "false"
	case Int:
		return strconv.Itoa(n.V)
	case Float:
		// The canonical spelling of a float: shortest round-trip, and a ".0"
		// when that leaves no decimal point or exponent. 1.0 is named "1.0",
		// 1e3 is "1000.0", 1e30 stays "1e+30", 0.5 is "0.5".
		//
		// The ".0" is what keeps a float out of the integers' namespace, which
		// is what lets 1 and 1.0 be two keys rather than a collision. Naming
		// per type and uniqueness per type are one mechanism, not two.
		return floatKeyText(n.V)
	case Str:
		return n.V
	case BigInt:
		return n.V.String()
	case BigFloat:
		return n.V.Text('g', -1)
	case Anchored:
		return KeyText(n.V)
	case Alias:
		return KeyText(n.V)
	case Tagged:
		return KeyText(n.V)
	default:
		// A collection used as a key. The library renders it with Go's %v and
		// codec.ToJSON writes something else again, which is a recorded
		// divergence rather than a meaning -- so the generator does not draw
		// one and nothing here has to name it.
		return fmt.Sprintf("%v", v.Decoded())
	}
}

func (Null) Decoded() any   { return nil }
func (b Bool) Decoded() any { return b.V }

// Decoded reflects that this library reads a non-negative integer as a uint64
// and a negative one as an int64. That asymmetry is the library's, not YAML's,
// and it is recorded here rather than worked around: a generator that quietly
// normalized it would stop noticing if it changed.
func (i Int) Decoded() any {
	if i.V >= 0 {
		return uint64(i.V)
	}

	return int64(i.V)
}

func (f Float) Decoded() any    { return f.V }
func (b BigInt) Decoded() any   { return b.V }
func (b BigFloat) Decoded() any { return b.V }
func (s Str) Decoded() any      { return s.V }

func (s Seq) Decoded() any {
	if len(s.Items) == 0 {
		return []any{}
	}

	out := make([]any, 0, len(s.Items))
	for _, item := range s.Items {
		out = append(out, item.Decoded())
	}

	return out
}

func (a Anchored) Decoded() any { return a.V.Decoded() }

// Decoded returns what the tagged value decodes to.
//
// One tag changes it. An untagged non-negative integer comes back as a uint64
// and a negative one as an int64, and `!!int` overrides both with a plain int
// -- so 5, !!int 5 and -5 are three spellings that produce three Go types. The
// asymmetry is the library's rather than YAML's, and it is written down here
// for the same reason [Int.Decoded] writes down the other half of it: a
// generator that normalized it would stop noticing if it changed.
func (t Tagged) Decoded() any {
	if t.Tag == TagInt {
		if n, ok := t.V.(Int); ok {
			return n.V
		}
	}

	return t.V.Decoded()
}

// Decoded returns what the anchored value decodes to, built afresh.
//
// Two occurrences of an alias decode to two structures that are equal and not
// identical, which is what comparing with ObjectsAreEqual sees. Whether the
// library shares the underlying object is a question about the library, not
// about what the document means.
func (a Alias) Decoded() any { return a.V.Decoded() }

func (m Map) Decoded() any {
	out := make(map[string]any, len(m.Pairs))
	for _, p := range m.Pairs {
		out[KeyText(p.Key)] = p.Val.Decoded()
	}

	return out
}

// awkwardStrings are the strings that have caused trouble before, or that sit
// on a boundary the emitter has to notice: they resolve to another type when
// written plain, or they cannot be written plain at all.
//
// A uniformly random string almost never lands on one of these, and they are
// where the defects are.
var awkwardStrings = []string{
	"", " ", "  ", "\t", "\n", "\n\n", " leading", "trailing ", " both ",
	"null", "Null", "NULL", "~", "true", "False", "yes", "no", "on", "off",
	"0", "1", "-1", "007", "1.5", ".5", "1e3", "0x1f", "0o17", ".inf", ".nan",
	"a: b", "a:b", "a #b", "a#b", "#a", "- a", "-a", "? a", ": a", ", a",
	"[a]", "{a}", "&a", "*a", "!a", "|a", ">a", "'a", `"a`, "%a", "@a", "`a",
	"a\nb", "a\n\nb", "a\nb\n", "line\n", "\nlead", "trail\n\n",
	"---", "...", "a---b", "a...b",
	"café", "日本語", "é", " nbsp", "emoji 🙂",
	"a\\b", "a\"b", "a'b", "a''b", `a\nb`,
	"2001-12-14", "12:34:56", "a: b: c",
}

// The tags a node can carry without changing what it means.
//
// Every one of them is measured rather than assumed. The two that resolve a
// plain scalar by its own kind -- the non-specific `!` and a local tag -- turn
// any scalar into its text, so they are only put on a [Str], a [Seq] or a
// [Map], where they are the identity.
//
// These are the tags, not the ways of writing them. The same tag is spelled
// three ways and [Style.TagSpelling] chooses: "!!int",
// "!<tag:yaml.org,2002:int>" and "!e!int" are one tag on the node and three
// documents. Splitting the two was what let the long form reach every kind --
// it used to exist only as a tenth tag, offered on [Str] alone, and so was
// written on the one kind where it made no difference.
const (
	TagNull  = "!!null"
	TagBool  = "!!bool"
	TagInt   = "!!int"
	TagFloat = "!!float"
	TagStr   = "!!str"
	TagSeq   = "!!seq"
	TagMap   = "!!map"
	TagLocal = "!foo"
	TagNone  = "!"
)

// TagFor returns the tags that can be written on v without changing what it
// decodes to, apart from the integer case [Tagged.Decoded] records.
func TagFor(v Value) []string {
	switch v.(type) {
	case Null:
		return []string{TagNull}
	case Bool:
		return []string{TagBool}
	case Int:
		return []string{TagInt}
	case Float, BigFloat:
		return []string{TagFloat}
	case BigInt:
		return []string{TagInt}
	case Str:
		return []string{TagStr, TagLocal, TagNone}
	case Seq:
		return []string{TagSeq, TagLocal, TagNone}
	case Map:
		return []string{TagMap, TagLocal, TagNone}
	default:
		// Nothing else takes a tag. Tagging runs before anchors and aliases
		// exist, so the only way here is a node that already carries one, and
		// YAML gives a node one tag.
		return nil
	}
}

// Values generates a Value tree, some of whose nodes carry anchors, some of
// which are aliases to them, and some of which carry a tag.
func Values() *rapid.Generator[Value] {
	return rapid.Custom(func(t *rapid.T) Value {
		return withAliases(t, withTags(t, values(0).Draw(t, "tree")))
	})
}

// withTags rewrites a tree so that some nodes carry a tag.
//
// It runs before [withAliases], because an [Alias] holds the value it stands
// for rather than looking it up, and a tag put on the anchored node afterwards
// would leave every alias to it decoding to what it meant before the tag. That
// is not hypothetical: `!!int` turns a uint64 into an int, so the document and
// the expected value disagreed on one node the first time round.
func withTags(t *rapid.T, v Value) Value {
	return (&tagger{t: t}).walk(v)
}

type tagger struct {
	t *rapid.T
}

// tagOdds is one in N. Low enough that most nodes stay untagged, high enough
// that a document of any size usually carries one.
const tagOdds = 7

func (g *tagger) walk(v Value) Value {
	switch n := v.(type) {
	case Seq:
		items := make([]Value, 0, len(n.Items))
		for _, item := range n.Items {
			items = append(items, g.walk(item))
		}

		return g.maybeTag(Seq{Items: items})
	case Map:
		pairs := make([]Pair, 0, len(n.Pairs))
		for _, p := range n.Pairs {
			pairs = append(pairs, Pair{Key: p.Key, Val: g.walk(p.Val)})
		}

		return g.maybeTag(Map{Pairs: pairs})
	default:
		return g.maybeTag(v)
	}
}

// maybeTag puts a tag on v, sometimes.
func (g *tagger) maybeTag(v Value) Value {
	if rapid.IntRange(0, tagOdds).Draw(g.t, "tag") != 0 {
		return v
	}

	tags := TagFor(v)
	if len(tags) == 0 {
		return v
	}

	return Tagged{Tag: rapid.SampledFrom(tags).Draw(g.t, "tagname"), V: v}
}

// withAliases rewrites a tree so that some nodes are anchored and some later
// nodes are replaced by an alias to one of them.
//
// The tree is generated first and rewritten afterwards, which is what makes the
// two rules an alias has to obey true by construction rather than by checking:
//
//   - an anchor enters the pool only once its own subtree is finished, so a node
//     can only alias something that was completed before it began. Since the
//     walk is in document order, the anchor is always written above the alias;
//   - a finished subtree cannot contain the node currently being visited, so no
//     alias can point at one of its own ancestors and no cycle is possible.
//
// Generating the aliases during the tree walk instead would need both rules
// enforced by hand, and a rejected draw every time one was broken.
func withAliases(t *rapid.T, v Value) Value {
	a := &aliaser{t: t}

	return a.walk(v)
}

type aliaser struct {
	t *rapid.T
	// pool holds the anchors whose subtree is complete, in document order.
	pool []Anchored
	// n numbers the anchors, so a name says where it was introduced.
	n int
}

// aliasOdds and anchorOdds are one in N. Anchors have to be more common than
// aliases, or the pool stays empty and the axis is never exercised; both stay
// low enough that most documents are still ordinary trees.
const (
	aliasOdds  = 5
	anchorOdds = 6
)

func (a *aliaser) walk(v Value) Value {
	if len(a.pool) > 0 && rapid.IntRange(0, aliasOdds).Draw(a.t, "alias") == 0 {
		target := rapid.SampledFrom(a.pool).Draw(a.t, "target")

		// An alias says exactly what its anchor said: the same name, standing
		// for the same value.
		return Alias(target)
	}

	out := a.children(v)

	if rapid.IntRange(0, anchorOdds).Draw(a.t, "anchor") != 0 {
		return out
	}

	a.n++
	anchored := Anchored{Name: fmt.Sprintf("a%d", a.n), V: out}
	a.pool = append(a.pool, anchored)

	return anchored
}

// children rebuilds v with the walk applied to everything inside it, and
// nothing applied to v itself.
//
// A tag is transparent here. The walk descends through it so that anchors land
// inside a tagged collection, but the tagged node is not offered an anchor of
// its own -- [aliaser.walk] has already done that for the whole of it. Without
// the split a node picks up an anchor on both sides of its tag, and `&a2 !!seq
// &a1 []` names one node twice.
func (a *aliaser) children(v Value) Value {
	switch n := v.(type) {
	case Seq:
		items := make([]Value, 0, len(n.Items))
		for _, item := range n.Items {
			items = append(items, a.walk(item))
		}

		return Seq{Items: items}
	case Map:
		pairs := make([]Pair, 0, len(n.Pairs))
		for _, p := range n.Pairs {
			pairs = append(pairs, Pair{Key: p.Key, Val: a.walk(p.Val)})
		}

		return Map{Pairs: pairs}
	case Tagged:
		return Tagged{Tag: n.Tag, V: a.children(n.V)}
	default:
		return v
	}
}

const maxDepth = 3

func values(depth int) *rapid.Generator[Value] {
	return rapid.Deferred(func() *rapid.Generator[Value] {
		scalars := []*rapid.Generator[Value]{
			rapid.Just(Value(Null{})),
			rapid.Custom(func(t *rapid.T) Value { return Bool{V: rapid.Bool().Draw(t, "bool")} }),
			rapid.Custom(drawInt),
			rapid.Custom(drawFloat),
			rapid.Custom(func(t *rapid.T) Value { return Str{V: Strings().Draw(t, "string")} }),
			// Strings twice over: they carry most of the presentation choices,
			// so they should carry most of the generated weight.
			rapid.Custom(func(t *rapid.T) Value { return Str{V: Strings().Draw(t, "string")} }),
		}

		if depth >= maxDepth {
			return rapid.OneOf(scalars...)
		}

		return rapid.OneOf(append(scalars,
			rapid.Custom(func(t *rapid.T) Value { return drawSeq(t, depth) }),
			rapid.Custom(func(t *rapid.T) Value { return drawMap(t, depth) }),
		)...)
	})
}

func drawSeq(t *rapid.T, depth int) Value {
	items := rapid.SliceOfN(values(depth+1), 0, 4).Draw(t, "items")

	// A flow pair -- the "b: c" in "[a, b: c]" -- can be written from one shape
	// and no other: a mapping of exactly one pair standing directly in a
	// sequence. A uniform draw reached it in 20 documents out of 40,000, so
	// Style.FlowPairs was an axis on paper.
	//
	// So two thirds of the mappings drawn into a sequence are cut to their first
	// pair, and one non-empty sequence in three gains a single-pair mapping it
	// did not draw. Nothing is lost by either: a mapping of three pairs inside
	// a sequence is written the same way as one anywhere else, and every other
	// axis already reaches it. Together they take the flow pair from 1 document
	// in 2,000 to 1 in 240.
	//
	// An empty sequence is left alone -- value/empty-collection is an axis too.
	for i, item := range items {
		m, keyed := item.(Map)
		if !keyed || len(m.Pairs) < 2 {
			continue
		}

		if rapid.IntRange(0, 2).Draw(t, "onepair") > 0 {
			items[i] = Map{Pairs: m.Pairs[:1]}
		}
	}

	if len(items) > 0 && rapid.IntRange(0, 2).Draw(t, "addpair") == 0 {
		at := rapid.IntRange(0, len(items)-1).Draw(t, "at")
		items[at] = Map{Pairs: []Pair{{
			Key: Keys().Draw(t, "pairkey"),
			Val: values(depth+1).Draw(t, "pairvalue"),
		}}}
	}

	return Seq{Items: items}
}

func drawMap(t *rapid.T, depth int) Value {
	// Weighted towards the small ones. A mapping of one pair is the only shape
	// a flow pair can be written from and the only one a flow mapping can hold
	// while still fitting on a line with something else; a fourth and fifth
	// pair repeat what the third already showed. Uniform over 0..4 spent most
	// of its mappings on the sizes that say least.
	n := rapid.SampledFrom([]int{0, 1, 1, 1, 2, 2, 3, 4}).Draw(t, "pairs")

	// Distinct once resolved, not distinct as written. Int{1} and Str{"1"} are
	// two different nodes and one key: this library refuses such a document as
	// a duplicate, and whether it is right to is a question breakRules asks on
	// purpose rather than one every drawn document should stumble into.
	seen := make(map[string]struct{}, n)
	pairs := make([]Pair, 0, n)

	for range n {
		key := Keys().Draw(t, "key")

		text := keyFamily(key)
		if _, dup := seen[text]; dup {
			continue
		}

		seen[text] = struct{}{}
		pairs = append(pairs, Pair{Key: key, Val: drawMapValue(t, depth)})
	}

	sort.Slice(pairs, func(i, j int) bool { return keyFamily(pairs[i].Key) < keyFamily(pairs[j].Key) })

	return Map{Pairs: pairs}
}

// drawMapValue draws what stands after a "k:", one value in five being null.
//
// Null is the value the language spells four ways -- "k:", "k: null", "k: ~"
// and "k: Null" -- and inside a flow mapping it is the only value
// Style.FlowEmpty can leave out or write as a key alone. Drawn from values() it
// arrives one time in eight, which put both of those axes past 1 in 300.
func drawMapValue(t *rapid.T, depth int) Value {
	if rapid.IntRange(0, 4).Draw(t, "nullvalue") == 0 {
		return Null{}
	}

	return values(depth+1).Draw(t, "value")
}

// keyFamily is [KeyText] widened to the spellings this library treats as one
// key, which is coarser than resolution and deliberately so.
//
// Str{"NULL"} resolves to the three letters and Null{} resolves to nothing, so
// they are two keys and KeyText says so. This library refuses the document
// anyway -- "mapping key \"NULL\" already defined" -- because it compares keys
// by the text that was written and NullSpelling may well have written the same
// letters. That is the departure yamlcorpus records as "two keys alike in text
// and different once resolved", and drawing a document that trips it would mean
// every such draw failing on a defect the corpus already states. breakRules
// produces the collision on purpose instead.
//
// Int{1} and Str{"1"} need no widening: KeyText already calls both "1".
func keyFamily(v Value) string {
	s, text := v.(Str)
	if !text {
		return KeyText(v)
	}

	if s.V == "~" {
		return "null"
	}

	if _, resolves := resolving[s.V]; resolves {
		return strings.ToLower(s.V)
	}

	// A string that spells a number belongs with the number: "1.0" and
	// Float{1} are one key to this library, whichever of them was written
	// first. ParseFloat is generous -- it takes "Inf" and "1e3", which YAML
	// spells differently -- and being generous here only makes the dedupe
	// coarser, which costs a draw and never a wrong document.
	if f, err := strconv.ParseFloat(s.V, 64); err == nil {
		return KeyText(Float{V: f})
	}

	return s.V
}

// Keys generates a mapping key, weighted heavily towards strings.
//
// Weighted, because a corpus of mappings keyed by floats would look nothing
// like the documents this library reads and would spend its draws away from
// where the defects are. One key in six is a non-string, which is enough to
// reach the class in most documents that have more than a pair or two.
//
// Scalars only. A sequence or a mapping used as a key is a document YAML
// admits, and what this library makes of one is a recorded divergence rather
// than a settled value -- see [KeyText] -- so drawing one would mean generating
// documents whose meaning the corpus cannot state.
func Keys() *rapid.Generator[Value] {
	return rapid.Custom(func(t *rapid.T) Value {
		// Five draws in six are strings. The weighting is measured, not
		// guessed: an even spread over the five kinds put two thirds of all
		// keys on a non-string, which cost the corpus a grammar bucket and six
		// matched ones -- awkwardStrings is what reaches the corners, and a
		// key drawn as a float reaches none of them.
		if rapid.IntRange(0, 5).Draw(t, "keykind") > 0 {
			return Str{V: Strings().Draw(t, "key")}
		}

		switch rapid.IntRange(0, 3).Draw(t, "scalar") {
		case 0:
			return Null{}
		case 1:
			return Bool{V: rapid.Bool().Draw(t, "bool")}
		case 2:
			return Int{V: rapid.IntRange(-1000, 1000).Draw(t, "int")}
		default:
			return Float{V: rapid.Float64Range(-1000, 1000).Draw(t, "float")}
		}
	})
}

// Strings generates a string, weighted toward the ones that are awkward to
// write down.
func Strings() *rapid.Generator[string] {
	return rapid.OneOf(
		rapid.SampledFrom(awkwardStrings),
		rapid.SampledFrom(awkwardStrings),
		rapid.StringMatching(`[a-zA-Z0-9 _.:#-]{0,12}`),
		rapid.StringOf(Runes()),
	)
}

// drawInt draws an integer, one in eight of them past a machine word.
//
// Wide numbers share the integer's slot rather than taking one of their own,
// because given a slot each they were a quarter of every scalar drawn and the
// corpus stopped reaching four of the parser's error messages.
//
// One in eight is not a tuned figure and the ratio barely matters. Sweeping it
// from one in six to one in sixteen moved the unmatched buckets in
// TestTheCorpusReachesMostOfTheGrammar between 63 and 65 with no trend, and the
// unreached templates in TestTheParserVocabularyGapIsMeasured between 27 and
// 28. Keeping the "wide" draw and never acting on it -- so the byte stream
// shifts and no wide number is produced -- costs 3 buckets and 2 templates on
// its own. Most of what a new kind costs is the reshuffle, not the dilution:
// the last few buckets are reached by a handful of documents each, and any
// change to the draw sequence hands them to different ones.
func drawInt(t *rapid.T) Value {
	if rapid.IntRange(0, 7).Draw(t, "wide") == 0 {
		return BigInt{V: bigInts().Draw(t, "bigint")}
	}

	return Int{V: rapid.IntRange(-1000, 1000).Draw(t, "int")}
}

// drawFloat draws a float, one in eight of them past what a float64 holds.
func drawFloat(t *rapid.T) Value {
	if rapid.IntRange(0, 7).Draw(t, "wide") == 0 {
		return BigFloat{V: bigFloats().Draw(t, "bigfloat")}
	}

	return Float{V: floats().Draw(t, "float")}
}

// Numbers wider than a machine word, and the bounds they are drawn inside.
//
// # The guard, and why it is not caution
//
// A big.Float keeps its exponent in an int32, and this library falls back to a
// float64 past that and hands back **zero** with nothing reported -- a recorded
// defect. Drawing a number past the bound would mean generating documents whose
// meaning the corpus states and the library cannot reach, which is a corpus
// accusing a library of a defect it has already recorded. So the exponent stays
// far inside: past float64's 308, nowhere near int32's two billion.
//
// The integer side has no such cliff, since a big.Int is bounded only by
// memory. The digit count is bounded anyway, to keep a document readable.
const (
	// bigFloatMinExp is past float64's range, so the value needs a big.Float.
	//
	// 330 and not 310, because float64 reaches 1e-320 through its subnormals:
	// this library reads 1e-320 as a float64 and 1e-324 as a big.Float, and a
	// generator that drew 1e-310 would state a big.Float meaning for a document
	// the library reads as a double. Measured rather than reasoned from the
	// exponent range.
	bigFloatMinExp = 330
	// bigFloatMaxExp is far inside big.Float's int32 exponent, and far inside
	// the bound where this library stops building one.
	bigFloatMaxExp = 4900
	// bigIntMinDigits keeps every drawn integer past a machine word.
	//
	// 21 and not 20, because uint64's maximum is itself twenty digits --
	// 18446744073709551615 -- so a twenty-digit draw may still fit one, and
	// this library reads what fits as a uint64. Twenty-one digits is at least
	// 1e20 and never does.
	bigIntMinDigits = 21
	bigIntMaxDigits = 45
)

// bigInts draws an integer past what a machine word holds.
func bigInts() *rapid.Generator[*big.Int] {
	return rapid.Custom(func(t *rapid.T) *big.Int {
		digits := rapid.IntRange(bigIntMinDigits, bigIntMaxDigits).Draw(t, "digits")

		text := make([]byte, 0, digits+1)
		if rapid.Bool().Draw(t, "negative") {
			text = append(text, '-')
		}

		text = append(text, byte('1'+rapid.IntRange(0, 8).Draw(t, "lead")))
		for range digits - 1 {
			text = append(text, byte('0'+rapid.IntRange(0, 9).Draw(t, "digit")))
		}

		out, ok := new(big.Int).SetString(string(text), 10)
		if !ok {
			panic("yamlgen: a decimal integer that big.Int will not read: " + string(text))
		}

		return out
	})
}

// bigFloats draws a float past what a float64 holds.
//
// Built from its text and parsed back, so the value carries exactly what the
// library's own parse of the same text carries: prec 64, and the same rounding.
func bigFloats() *rapid.Generator[*big.Float] {
	return rapid.Custom(func(t *rapid.T) *big.Float {
		mantissa := rapid.IntRange(1, 9999).Draw(t, "mantissa")
		exponent := rapid.IntRange(bigFloatMinExp, bigFloatMaxExp).Draw(t, "exponent")

		sign := ""
		if rapid.Bool().Draw(t, "negative") {
			sign = "-"
		}

		if rapid.Bool().Draw(t, "tiny") {
			exponent = -exponent
		}

		text := fmt.Sprintf("%s%de%d", sign, mantissa, exponent)

		out, ok := new(big.Float).SetString(text)
		if !ok {
			panic("yamlgen: a decimal float that big.Float will not read: " + text)
		}

		return out
	})
}

func floats() *rapid.Generator[float64] {
	return rapid.Custom(func(t *rapid.T) float64 {
		// One float in nine is a special. Weighted low on purpose: they are
		// three values against a continuum, and a corpus that drew them evenly
		// would spend most of its floats on three documents.
		switch rapid.IntRange(0, 8).Draw(t, "kind") {
		case 0:
			return math.Inf(1)
		case 1:
			return math.Inf(-1)
		case 2:
			return math.NaN()
		}

		f := rapid.Float64Range(-1e6, 1e6).Draw(t, "f")
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return 0
		}

		return f
	})
}

// floatKeyText is the canonical spelling of a float, shared by [KeyText] and
// the tests that check it against the other readings.
//
// The infinities and NaN are spelled the way YAML spells them rather than the
// way Go prints them -- ".inf" and not "+Inf" -- which is what this library
// does and what round-trips. A reader comparing against libfyaml will find
// "Infinity" there instead; the difference is deliberate.
func floatKeyText(f float64) string {
	switch {
	case math.IsNaN(f):
		return ".nan"
	case math.IsInf(f, 1):
		return ".inf"
	case math.IsInf(f, -1):
		return "-.inf"
	}

	out := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(out, ".eE") {
		out += ".0"
	}

	return out
}
