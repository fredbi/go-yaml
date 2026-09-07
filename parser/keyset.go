// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"github.com/go-openapi/go-yaml/token"
)

// spillAt is the number of keys past which a mapping gets a hash index instead
// of a scan.
//
// It is a guard against a document built to be slow, not a crossover to tune.
// Over the workload corpus the median mapping has two keys and the 99th
// percentile nine; of the 35,144 mappings in it, five have more than 64. A
// document that nests ten thousand keys under one mapping is the case this
// exists for, and anything from 32 to 256 reads the same on a real document.
const spillAt = 64

// mapKeyRef addresses one key of one mapping, and is the key of the spilled
// index: base is where that mapping's keys start in keySet.entries, and text is
// the key as mapKeyText reads it.
//
// §3.2.1.1 makes two keys equal when they resolve to the same node, so the type
// is half the identity: "7" and "007" are one integer written twice and "1" and
// "1.0" are an integer and a float. Comparing the characters alone read the
// first pair as two keys and the second as one, and read "1" and "\"1\"" as one
// where they are a number and a string.
type mapKeyRef struct {
	base int32
	kind token.KeyKind
	text string
}

// keyEntry is one key on the stack: its spelling and where it was written.
type keyEntry struct {
	text string
	at   token.Position
}

// keySet finds a key a mapping has already used.
//
// The keys of every mapping open at this point in the descent sit in entries,
// innermost last, and a mapping's own keys are the tail from its base --
// mappings nest, so a mapping records keys only while it is the innermost one
// open. The parser probe holds that invariant ("mapkey.stackTailIsOneMapping",
// nil over 180,000 recordings), and it is what lets a lookup be a scan of that
// tail rather than a hash into a map shared by every open mapping.
//
// filter is the scan. One word per key, sixteen to a cache line, against the 32
// bytes an entry takes: a mapping of forty keys is scanned in three lines
// rather than twenty-five. entries is read only where a filter matches.
//
// index is for a mapping that passes spillAt, and stays nil on every document
// in the corpus.
type keySet struct {
	filter  []uint32
	entries []keyEntry
	index   map[mapKeyRef]token.Position
}

// keyFilter stands in for a key in the scan, and is read in constant time: the
// kind, the length, and the first and last byte, packed into one word.
//
// It is deliberately not a hash of the whole string. Hashing every key is what
// the map did -- three times over, at the lookup, the insert and the delete --
// and is the cost this replaces. A collision costs one string compare and
// nothing else, so the filter only has to be cheap and roughly discriminating.
//
// The kind occupies the low byte whole, so a filter match settles the kind and
// only the text is left to confirm.
func keyFilter(text string, kind token.KeyKind) uint32 {
	f := uint32(kind) | uint32(len(text))<<8
	if len(text) > 0 {
		f |= uint32(text[0])<<16 | uint32(text[len(text)-1])<<24
	}

	return f
}

// base returns where the mapping opening now starts its keys.
func (k *keySet) base() int { return len(k.entries) }

// record records that the mapping starting at base uses text as a key, written
// at pos.
//
// It returns where text was first written, and whether the mapping had already
// used it.
func (k *keySet) record(base int, text string, kind token.KeyKind, pos token.Position) (token.Position, bool) {
	f := keyFilter(text, kind)
	if len(k.entries)-base >= spillAt {
		return k.recordIndexed(base, text, kind, pos, f)
	}

	for i := base; i < len(k.entries); i++ {
		if k.filter[i] == f && k.entries[i].text == text {
			return k.entries[i].at, true
		}
	}
	k.push(text, pos, f)

	return token.Position{}, false
}

// push adds one key to the stack.
func (k *keySet) push(text string, pos token.Position, f uint32) {
	k.filter = append(k.filter, f)
	k.entries = append(k.entries, keyEntry{text: text, at: pos})
}

// recordIndexed records a key of a mapping big enough to have earned an index,
// and builds that index the first time one is.
func (k *keySet) recordIndexed(base int, text string, kind token.KeyKind, pos token.Position, f uint32) (token.Position, bool) {
	if len(k.entries)-base == spillAt {
		k.spill(base)
	}

	ref := mapKeyRef{base: int32(base), kind: kind, text: text}
	if prev, defined := k.index[ref]; defined {
		return prev, true
	}
	k.index[ref] = pos
	k.push(text, pos, f)

	return token.Position{}, false
}

// spill moves the keys a mapping gathered under the scan into the index, once,
// as it passes spillAt.
func (k *keySet) spill(base int) {
	if k.index == nil {
		k.index = make(map[mapKeyRef]token.Position, spillAt*2)
	}
	for i := base; i < len(k.entries); i++ {
		ref := mapKeyRef{base: int32(base), kind: token.KeyKind(k.filter[i]), text: k.entries[i].text}
		if _, defined := k.index[ref]; !defined {
			k.index[ref] = k.entries[i].at
		}
	}
}

// close drops the keys of the mapping that started at base.
//
// Mappings close in the order they open, so the keys of the one closing are
// always those above its base. A mapping that never spilled is dropped by
// truncating; only a spilled one has entries to delete.
func (k *keySet) close(base int) {
	if len(k.entries)-base >= spillAt {
		for i := base; i < len(k.entries); i++ {
			delete(k.index, mapKeyRef{
				base: int32(base),
				kind: token.KeyKind(k.filter[i]),
				text: k.entries[i].text,
			})
		}
	}
	k.filter = k.filter[:base]
	k.entries = k.entries[:base]
}
