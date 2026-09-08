// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus

import "github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"

// Injecting a duplicate key rather than waiting for one to be drawn.
//
// # Why the draw cannot supply these
//
// yamlgen.drawMap dedupes with keyFamily, deliberately: a document whose keys
// collide is one the library refuses, and a generator that produced them by
// accident would spend its draws on documents no property can score. So the
// only duplicates a corpus sees are the ones a rule puts there.
//
// [duplicateAKey] has done that since the register was resurrected, and it is
// narrow: it needs a mapping of two entries and it overwrites the second key
// with the first, so the document loses an entry as well as gaining a
// collision. That covers the commonest shape and misses the interesting ones --
// above all a *collection* standing as the key, where the two defects of
// 2026-09-07 and 2026-09-08 lived, because naming a collection key is where
// this parser has gone wrong three times.
//
// [repeatAKey] fills the gap. It appends an entry whose key is one the mapping
// already holds, so nothing is lost and every mapping admits it, and it aims
// at a collection key wherever the document has one.
//
// # What the document then is, and what it is not
//
// Valid YAML: 3.2.1.1 is a rule about the representation graph and no grammar
// sees it, which TestTheBrokenDocumentsAreInvalidWhereTheGrammarCannotTell
// asserts over every break here. Refused by a strict consumer: KeyRules settles
// TagDuplicateKey at Reject, "it is an error for two equal keys to appear in
// the same mapping node".
//
// Refused by a strict consumer, and not by every consumer. libfyaml 1.0.0b1
// reads every one of them and keeps the last; this library refuses them by
// default and reads them under codec.AllowDuplicateMapKey, which records
// nothing and lets the last entry win. A table saying so stands Either on the
// tag and stance.Table.Expect leaves the document unscored -- see KeyRules,
// which sets that out and declines to settle enforcement.

// repeatAKey appends an entry whose key repeats one the mapping already holds.
//
// The target is the first collection key in document order, and the first key
// of the first mapping where the document holds no collection key. Preferring
// the collection is the whole point of the rule: a collection key is rare in a
// draw -- Keys() gives one key in 24 -- and it is where the naming defects are.
//
// The appended value is Str{"dup"} rather than a copy of what stands under the
// original key, so a consumer reading the document under
// codec.AllowDuplicateMapKey can tell which entry won.
func repeatAKey(v yamlgen.Value) (yamlgen.Value, bool) {
	target, found := aRepeatableKey(v, true)
	if !found {
		target, found = aRepeatableKey(v, false)
	}

	if !found {
		return v, false
	}

	var done bool

	var walk func(yamlgen.Value) yamlgen.Value

	walk = func(n yamlgen.Value) yamlgen.Value {
		if done {
			return n
		}

		switch t := n.(type) {
		case yamlgen.Anchored:
			return yamlgen.Anchored{Name: t.Name, V: walk(t.V)}
		case yamlgen.Tagged:
			return yamlgen.Tagged{Tag: t.Tag, V: walk(t.V)}
		case yamlgen.Seq:
			items := make([]yamlgen.Value, 0, len(t.Items))
			for _, item := range t.Items {
				items = append(items, walk(item))
			}

			return yamlgen.Seq{Items: items}
		case yamlgen.Map:
			if repeatable(t) && holdsTheKey(t, target) {
				done = true

				return appendEntries(t, yamlgen.Pair{Key: target, Val: yamlgen.Str{V: "dup"}})
			}

			pairs := make([]yamlgen.Pair, 0, len(t.Pairs))
			for _, p := range t.Pairs {
				pairs = append(pairs, yamlgen.Pair{Key: walk(p.Key), Val: walk(p.Val)})
			}

			return yamlgen.Map{Pairs: pairs}
		default:
			return n
		}
	}

	out := walk(v)

	return out, done
}

// repeatACollectionKey appends two entries keyed by the same collection.
//
// Where repeatAKey recycles a key the document already holds, this one builds
// the key: a sequence holding the mapping's first key, so "a: 1" gains
// "[a]: dup" twice. Recycling cannot reach this often enough to matter --
// Keys() draws a collection for one key in 24 and only one drawn value in seven
// holds a mapping at all, which put a collection key at 24 of 2000 documents.
// Building one puts it in every document that has a mapping.
//
// ⚠️ In block the document is refused for a second reason as well: two block
// collection keys collide whatever they hold, which is
// yamlgen_test.TestDefectTwoBlockCollectionKeysCollide. So a rejection here is
// evidence about the duplicate check only in flow, until that defect closes.
func repeatACollectionKey(v yamlgen.Value) (yamlgen.Value, bool) {
	seed, found := aRepeatableKey(v, false)
	if !found {
		return v, false
	}

	key := yamlgen.Seq{Items: []yamlgen.Value{seed}}

	var done bool

	var walk func(yamlgen.Value) yamlgen.Value

	walk = func(n yamlgen.Value) yamlgen.Value {
		if done {
			return n
		}

		switch t := n.(type) {
		case yamlgen.Anchored:
			return yamlgen.Anchored{Name: t.Name, V: walk(t.V)}
		case yamlgen.Tagged:
			return yamlgen.Tagged{Tag: t.Tag, V: walk(t.V)}
		case yamlgen.Seq:
			items := make([]yamlgen.Value, 0, len(t.Items))
			for _, item := range t.Items {
				items = append(items, walk(item))
			}

			return yamlgen.Seq{Items: items}
		case yamlgen.Map:
			if repeatable(t) && !holdsTheKey(t, key) {
				done = true

				return appendEntries(t,
					yamlgen.Pair{Key: key, Val: yamlgen.Str{V: "dup"}},
					yamlgen.Pair{Key: key, Val: yamlgen.Str{V: "dup2"}},
				)
			}

			pairs := make([]yamlgen.Pair, 0, len(t.Pairs))
			for _, p := range t.Pairs {
				pairs = append(pairs, yamlgen.Pair{Key: walk(p.Key), Val: walk(p.Val)})
			}

			return yamlgen.Map{Pairs: pairs}
		default:
			return n
		}
	}

	out := walk(v)

	return out, done
}

// aRepeatableKey returns a key to repeat, taking a collection where collections
// asks for one.
//
// Two passes rather than one so that a collection key anywhere in the document
// beats a scalar key at its root: the caller asks for a collection first and
// settles for whatever it finds second.
func aRepeatableKey(v yamlgen.Value, collections bool) (yamlgen.Value, bool) {
	switch t := v.(type) {
	case yamlgen.Anchored:
		return aRepeatableKey(t.V, collections)
	case yamlgen.Tagged:
		return aRepeatableKey(t.V, collections)
	case yamlgen.Seq:
		for _, item := range t.Items {
			if k, found := aRepeatableKey(item, collections); found {
				return k, true
			}
		}
	case yamlgen.Map:
		if repeatable(t) {
			for _, p := range t.Pairs {
				if !collections || isACollectionKey(p.Key) {
					return p.Key, true
				}
			}
		}

		for _, p := range t.Pairs {
			if k, found := aRepeatableKey(p.Key, collections); found {
				return k, true
			}

			if k, found := aRepeatableKey(p.Val, collections); found {
				return k, true
			}
		}
	}

	return nil, false
}

// repeatable reports whether a mapping's first key can be repeated below it.
//
// A merge key cannot. Repeating a "<<" writes a document whose break is a
// different rule than the tag claims -- see holdsAMergeKey, which
// [duplicateAKey] skips for the same reason.
func repeatable(m yamlgen.Map) bool {
	return len(m.Pairs) > 0 && !holdsAMergeKey(m)
}

// isACollectionKey reports whether a key is a sequence or a mapping, looking
// through the properties that may stand in front of one.
func isACollectionKey(v yamlgen.Value) bool {
	switch t := v.(type) {
	case yamlgen.Seq, yamlgen.Map:
		return true
	case yamlgen.Anchored:
		return isACollectionKey(t.V)
	case yamlgen.Alias:
		return isACollectionKey(t.V)
	case yamlgen.Tagged:
		return isACollectionKey(t.V)
	}

	return false
}

// holdsTheKey reports whether a mapping holds the key the walk was sent to find.
//
// Compared by the name the library gives it rather than by identity: the walk
// rebuilds the tree as it goes, so the node aRepeatableKey returned is not the
// node the walk holds. yamlgen.KeyText is the same naming the duplicate check
// itself uses, so a match here is a collision there.
func holdsTheKey(m yamlgen.Map, target yamlgen.Value) bool {
	name := yamlgen.KeyText(target)

	for _, p := range m.Pairs {
		if yamlgen.KeyText(p.Key) == name {
			return true
		}
	}

	return false
}

// appendEntries returns m with more entries after the ones it holds.
func appendEntries(m yamlgen.Map, extra ...yamlgen.Pair) yamlgen.Map {
	pairs := make([]yamlgen.Pair, len(m.Pairs), len(m.Pairs)+len(extra))
	copy(pairs, m.Pairs)

	return yamlgen.Map{Pairs: append(pairs, extra...)}
}
