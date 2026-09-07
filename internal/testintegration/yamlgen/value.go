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
	"time"

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
	// MergeKey is the "<<" of a merge entry.
	//
	// A construct rather than Str{"<<"} written plain, and plainSafe is why: it
	// requires a leading letter, deliberately, so a string spelling "<<" is
	// always quoted -- and a quoted "<<" is an ordinary key, not a merge. So
	// writing the merge key at all needs something that says "this is the merge
	// key" rather than a string that happens to spell it.
	//
	// It has no meaning of its own and Decoded panics: a MergeKey is read by
	// the Map that holds it, which is where the merge happens. Standing
	// anywhere else it is a generator fault rather than a document.
	MergeKey struct{}
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
	// Timestamp is a point in time, which this library reads from a scalar
	// carrying `!!timestamp`.
	//
	// The tag is not optional. YAML 1.2's core schema resolves null, bool, int,
	// float and str and no timestamp, so "2001-12-14" written plain is the
	// string "2001-12-14" under every version this library reads --
	// codec.TestATimestampIsATextualScalar holds that down. A Timestamp is
	// therefore always drawn inside a [Tagged], and drawTextual is the only
	// place one is made.
	//
	// The time is always UTC and carries no monotonic reading, so the value the
	// document decodes to compares equal to this one under reflect's equality.
	Timestamp struct{ V time.Time }
	// Binary is a byte string, which this library reads from a scalar carrying
	// `!!binary` by decoding its base64 text.
	//
	// Tagged for the same reason as [Timestamp], and drawn the same way.
	Binary struct{ V []byte }
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
	case MergeKey:
		// Reached only where a merge is suppressed, since Map.Decoded takes the
		// entry out otherwise. Nothing suppresses one today; the case is here so
		// a future Style that quotes the key names it as the ordinary key it
		// then is.
		return "<<"
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

func (t Timestamp) Decoded() any { return t.V }
func (b Binary) Decoded() any    { return b.V }

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

// Decoded panics: a merge key means nothing on its own.
//
// [Map.Decoded] reads it where it stands, so reaching this is a MergeKey put
// somewhere a merge entry's key is not.
func (MergeKey) Decoded() any {
	panic("yamlgen: a MergeKey stands outside a mapping entry's key")
}

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

// Decoded returns what Unmarshal into an `any` produces for this mapping, with
// a "<<" entry merged into it.
//
// # The merge rule, and why it is written down rather than deferred
//
// The 1.1 merge type says an entry's own keys win over the ones a "<<" brings,
// and that a sequence of mappings merges with the earlier winning. That is a
// specified rule and not a guess -- go.yaml.in/yaml/v3 v3.0.5 agrees with this
// library on every resolvable shape -- so modeling it here is the same kind of
// work as modeling the 1.1 resolution table, and it is what lets a generated
// merge document check a *value* rather than only that it parses. The defect it
// exists to catch was a value defect: a mapping's own key lost outright when
// the "<<" came second.
//
// # And why the corpus still states no meaning for one
//
// YAML 1.2 dropped the merge type, so a conforming 1.2 reader gives "<<" back
// as an ordinary key and is not wrong. What this returns is what *this library*
// does, which is what the properties compare against; [Written.MeansUnclear] is
// set for any document holding a merge key, so the artifact ships the stance
// tag and no answer. See yamlcorpus/merge.go.
func (m Map) Decoded() any {
	out := make(map[string]any, len(m.Pairs))

	var merged []map[string]any

	for _, p := range m.Pairs {
		if _, isMerge := p.Key.(MergeKey); isMerge {
			merged = append(merged, mergedMappings(p.Val)...)

			continue
		}

		out[KeyText(p.Key)] = p.Val.Decoded()
	}

	// Own entries first, then each merged mapping in turn, and neither
	// overwrites a key already standing. That is one statement of both halves
	// of the rule: own keys win, and among the merged the earlier wins.
	for _, from := range merged {
		for k, v := range from {
			if _, held := out[k]; !held {
				out[k] = v
			}
		}
	}

	return out
}

// mergedMappings returns the mappings a "<<" entry's value brings in, in the
// order they are to be merged.
//
// The value is a mapping, or a sequence of them, and an alias or an anchor may
// stand in front of either -- 1.1 says what the value is and nothing about how
// it got there.
func mergedMappings(v Value) []map[string]any {
	switch n := v.(type) {
	case Anchored:
		return mergedMappings(n.V)
	case Alias:
		return mergedMappings(n.V)
	case Map:
		one, _ := n.Decoded().(map[string]any)

		return []map[string]any{one}
	case Seq:
		out := make([]map[string]any, 0, len(n.Items))
		for _, item := range n.Items {
			out = append(out, mergedMappings(item)...)
		}

		return out
	default:
		// "<<: 1" asks to merge a scalar, which is no operation at all. The
		// generator does not draw one; yamlcorpus enumerates it by hand, where
		// it carries TagMergeNonMapping and no meaning.
		return nil
	}
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

	// TagSet, TagOMap and TagPairs name collection types the 2005 type
	// repository defines. Each is an annotation and nothing more here: a `!!set`
	// mapping decodes to the same map[string]any as an untagged one, and an
	// `!!omap` sequence to the same []any.
	//
	// Offered on the kind the type is written over -- `!!set` on a mapping,
	// `!!omap` and `!!pairs` on a sequence -- and not on the shapes *inside*
	// it, which the type definitions constrain and no implementation enforces.
	// Measured on 2026-09-13: `a: !!set` over `x: 1` and `a: !!omap` over `- 1`
	// are read by this library, by libfyaml 1.0.0b1 and by
	// go.yaml.in/yaml/v3 v3.0.5, and grammar.NewRecognizer accepts both. So
	// requiring null values under a set, or single-pair mappings under an omap,
	// would narrow the draw for a rule nobody applies.
	TagSet   = "!!set"
	TagOMap  = "!!omap"
	TagPairs = "!!pairs"

	// TagTimestamp and TagBinary name types the 2005 type repository defines
	// rather than types a schema resolves, so they carry a value no untagged
	// scalar can spell. [TagFor] offers each on its own kind and on nothing
	// else, and drawTextual writes the tag as it draws the value.
	TagTimestamp = "!!timestamp"
	TagBinary    = "!!binary"
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
	case Timestamp:
		return []string{TagTimestamp}
	case Binary:
		return []string{TagBinary}
	case Str:
		return []string{TagStr, TagLocal, TagNone}
	case Seq:
		return []string{TagSeq, TagOMap, TagPairs, TagLocal, TagNone}
	case Map:
		return []string{TagMap, TagSet, TagLocal, TagNone}
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
		// Snapshotted before the children are walked, and that is the whole of
		// what makes the alias resolve. An anchor created inside this mapping
		// is in the pool by the end of the loop but stands *after* the "<<"
		// this prepends, and an alias naming an anchor its document has not
		// declared yet is refused -- "{<<: *a1, b: &a1 {c: 1}}" is
		// `could not find alias "a1"`.
		from := a.mappings()

		pairs := make([]Pair, 0, len(n.Pairs)+1)
		for _, p := range n.Pairs {
			pairs = append(pairs, Pair{Key: p.Key, Val: a.walk(p.Val)})
		}

		return Map{Pairs: a.merge(pairs, from)}
	case Tagged:
		return Tagged{Tag: n.Tag, V: a.children(n.V)}
	default:
		return v
	}
}

// mergeOdds is one in N, over the mappings drawn while the pool holds a
// mapping to merge from.
//
// Low, because a merge is a rare thing to write and because every one of them
// costs the corpus a stated meaning -- yamlgen will not say what a merge
// document denotes, so a document carrying one is scored on its shape and not
// on its value. Frequent merges would trade away the corpus's reach.
const mergeOdds = 11

// merge gives a mapping a "<<" entry, one time in mergeOdds.
//
// # Why here
//
// A merge needs a mapping to merge *from*, anchored earlier in the same
// document, and the aliaser is already holding exactly that: a pool of finished
// anchors in document order. Drawing the merge anywhere else would mean
// building the anchor and the alias separately and hoping they line up.
//
// # What it draws
//
// One "<<" and no more: two of them in a mapping repeat a key, which 3.2.1.1
// makes an error whatever merge means, and yamlcorpus enumerates that by hand
// with TagDuplicateKey on it. The value is an alias to a mapping, or a sequence
// of two aliases -- 1.1 admits both, and the sequence is where the earlier-wins
// half of the rule lives.
//
// A key the merged mapping and the own mapping share is not avoided. It is the
// interesting case rather than a collision to dodge: an own key winning over a
// merged one is the precedence rule, and reading it the other way round was a
// real defect.
//
// from is the pool as it stood before this mapping's children were walked. See
// the Map case in children for why it cannot be taken afterwards.
func (a *aliaser) merge(pairs []Pair, from []Anchored) []Pair {
	if len(pairs) == 0 || rapid.IntRange(0, mergeOdds).Draw(a.t, "merge") != 0 {
		return pairs
	}

	// An anchored mapping earlier in the document if there is one, and a
	// mapping written where it stands otherwise. The inline form is not a
	// fallback for its own sake: 1.1 says the value is a mapping and nothing
	// about how it got there, so "<<: {a: 1}" asks for the same merge that
	// "<<: *b" does, and an implementation that resolves the alias may still
	// have no path for a mapping that was never anchored. It is also what makes
	// a merge reachable at all on a document whose anchors all came later.
	val := a.mergeFrom(from)

	if rapid.Bool().Draw(a.t, "mergeseq") {
		// A sequence of two, which is where the earlier-wins half of the rule
		// lives. Drawn independently, so an alias and an inline mapping can
		// stand side by side -- the shape codec.ToJSON writes invalid JSON for.
		val = Seq{Items: []Value{a.mergeFrom(from), a.mergeFrom(from)}}
	}

	// In front of the mapping's own entries, which is where a "<<" is usually
	// written and the order that makes the precedence rule visible: the own
	// keys come after and still win.
	return append([]Pair{{Key: MergeKey{}, Val: val}}, pairs...)
}

// mergeFrom draws one mapping for a "<<" to merge: an alias to an anchored one
// where the pool has any, and a small mapping written in place otherwise.
func (a *aliaser) mergeFrom(from []Anchored) Value {
	if len(from) > 0 && rapid.Bool().Draw(a.t, "mergealias") {
		return Alias(rapid.SampledFrom(from).Draw(a.t, "mergefrom"))
	}

	return Map{Pairs: []Pair{{
		Key: Str{V: "merged" + strconv.Itoa(rapid.IntRange(0, 3).Draw(a.t, "mergedkey"))},
		Val: Int{V: rapid.IntRange(-9, 9).Draw(a.t, "mergedvalue")},
	}}}
}

// mappings returns the pooled anchors standing on a mapping, which are the ones
// a "<<" may merge from.
func (a *aliaser) mappings() []Anchored {
	var out []Anchored

	for _, anchored := range a.pool {
		// Non-empty, and that is not fussiness. An empty mapping writes nothing
		// at all in block context, so "<<: *a" pointing at one comes out as
		// "<<:" with no value -- a merge of null, which is not a merge. It is
		// also a shape the two decode paths disagree about, held by
		// TestDefectMergingNullIsReadByTheWalkAndRefusedByTheTree, so drawing
		// it here would put a document in the corpus whose meaning this package
		// cannot state.
		if m, isMap := anchored.V.(Map); isMap && len(m.Pairs) > 0 {
			out = append(out, anchored)
		}
	}

	return out
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
			rapid.Custom(drawTextual),
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

// drawTextual draws a string, and one draw in eight a [Timestamp] or a
// [Binary] instead.
//
// They share the string's slot for the reason [drawInt] gives for the wide
// numbers: given a slot each they would be a third of every scalar drawn, and
// awkwardStrings is what reaches the corners of the presentation axes. One in
// eight of one slot in six puts a timestamp or a byte string on 1 scalar in 48.
//
// Each comes back already inside a [Tagged], because neither resolves: an
// untagged "2001-12-14" is the string "2001-12-14" and an untagged "aGVsbG8="
// is that text. The tagger leaves a node that carries a tag alone, so the tag
// drawn here is the one the document is written with, and [Style.TagSpelling]
// still chooses among "!!timestamp", "!<tag:yaml.org,2002:timestamp>" and a
// handle the document declares.
func drawTextual(t *rapid.T) Value {
	switch rapid.IntRange(0, 15).Draw(t, "textual") {
	case 0:
		return Tagged{Tag: TagTimestamp, V: Timestamp{V: drawTime(t)}}
	case 1:
		return Tagged{Tag: TagBinary, V: Binary{V: rapid.SliceOfN(rapid.Byte(), 1, 12).Draw(t, "bytes")}}
	default:
		return Str{V: Strings().Draw(t, "string")}
	}
}

// drawTime draws a UTC instant, half of them at midnight so that the date-only
// spelling [TimeDate] has values to write.
//
// The fraction is drawn in hundredths of a second rather than in nanoseconds.
// Go's reference layouts read as many fractional digits as are written, but the
// timestamp type's own examples stop at two, and a nine-digit fraction would
// test the layout table rather than the presentation axis this draw exists for.
func drawTime(t *rapid.T) time.Time {
	var (
		year  = rapid.IntRange(1900, 2100).Draw(t, "year")
		month = rapid.IntRange(1, 12).Draw(t, "month")
		day   = rapid.IntRange(1, 28).Draw(t, "day")
	)

	if rapid.Bool().Draw(t, "midnight") {
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	}

	var (
		hour     = rapid.IntRange(0, 23).Draw(t, "hour")
		min      = rapid.IntRange(0, 59).Draw(t, "minute")
		sec      = rapid.IntRange(0, 59).Draw(t, "second")
		hundreds = rapid.IntRange(0, 99).Draw(t, "hundredths")
	)

	return time.Date(year, time.Month(month), day, hour, min, sec, hundreds*int(10*time.Millisecond), time.UTC)
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

	// YAML 1.1's boolean words belong with the boolean they spell there.
	// Style.Version may write "%YAML 1.1" over any document, and under it
	// "no:" is the key false -- so "no" beside Bool{false} is one key and the
	// library refuses the document as a duplicate. The style is drawn after
	// the value and the two are independent on purpose, so the dedupe has to
	// hold for every style rather than for the one that was drawn.
	if b, legacy := legacyBooleans[s.V]; legacy {
		return strconv.FormatBool(b)
	}

	// A string's own text, which is also the name this library gives it: a
	// quoted key is a string and a string is named by what it spells. So
	// Str{"1.0"} lands with Float{1}, whose canonical name is "1.0", and
	// Str{"0"} lands with Int{0}, whose canonical name is "0".
	//
	// This used to run the text through ParseFloat and name the float it
	// found, which was wrong in both directions. It missed Str{"0"} beside
	// Int{0} -- two keys the library merges, drawn together and refused as a
	// duplicate, which TestEmitParses caught -- and it needlessly collided
	// Str{"1e3"} with Float{1000}, whose names are "1e3" and "1000.0" and are
	// two keys to everybody.
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

		// ⚠️ A collection key is NOT drawn yet, and drawKeyCollection below is
		// what would draw one. The emitter writes one correctly -- explicitKey
		// takes the long form for it whatever Style.ExplicitKeys says and puts
		// it below the "?", which is the census gap this was for -- and the
		// defects it reaches are filed and pinned:
		//
		//	an explicit key inside an explicit key is refused
		//	a key written below its indicator loses its indentation on render
		//	a collection key written alone in flow is refused
		//	two collection keys in one mapping collide, both named "{"
		//
		// What is left is this package's own model rather than the library's.
		// KeyText names a collection key from Decoded(), which is the core
		// answer, so under "%YAML 1.1" a key holding "08" is named wrongly --
		// readings.legacyKey answers for a scalar key and has no case for a
		// collection. Rendering a collection key also drops a comment and
		// changes a value in ways not yet reduced.
		//
		// Turning the draw on is a round of work rather than a line. The
		// yamlcorpus census keeps the gap open and says why.

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

// drawKeyCollection draws the small collections a key may be.
//
// Small on purpose and never nested: a key is written twice in the document the
// property tests render -- once as itself and once in every message naming it --
// and KeyText renders a collection with Go's %v, which grows fast. Two entries
// is enough to be a collection.
//
//nolint:unused // Parked, not dead: Keys() names it in the note above and turning the draw on is one line.
func drawKeyCollection(t *rapid.T) Value {
	items := []Value{
		Str{V: Strings().Draw(t, "keyitem")},
		Int{V: rapid.IntRange(-99, 99).Draw(t, "keyint")},
	}

	if rapid.Bool().Draw(t, "keyseq") {
		return Seq{Items: items[:1+rapid.IntRange(0, 1).Draw(t, "keyitems")]}
	}

	return Map{Pairs: []Pair{{Key: items[0], Val: items[1]}}}
}

// Streams generates a sequence of documents, one to three of them.
//
// Each is drawn independently, so no alias reaches past the document that
// declares it and no %TAG handle is used outside the document that declares it.
// Both of those are documents the library refuses, and refuses correctly --
// measured on 2026-09-13 against libfyaml 1.0.0b1, go.yaml.in/yaml/v3 v3.0.5
// and the reference parser, which all refuse the handle -- so they are
// enumerated in yamlcorpus rather than drawn here.
//
// Weighted towards two. One document is what Values already draws and says
// nothing new; three is enough to reach a separator between separators, and a
// fourth repeats it.
func Streams() *rapid.Generator[[]Value] {
	return rapid.Custom(func(t *rapid.T) []Value {
		n := rapid.SampledFrom([]int{2, 2, 2, 3}).Draw(t, "documents")

		docs := make([]Value, 0, n)
		for range n {
			docs = append(docs, Values().Draw(t, "document"))
		}

		return docs
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
