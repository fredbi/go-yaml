// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"testing"

	"github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"
)

// Counting what the draws reach, where no byte can see it.
//
// yamlcorpus.Constructs counts the shapes a document *contains*, which answers
// the same question for anything visible in the text. A rule about how two
// nodes relate -- a merged key colliding with an own key -- is not visible
// there, so it is counted here, over the values themselves.
//
// The count exists because a range can narrow the rule above it without
// moving any label: every declared feature was reachable on 2026-09-08 while a
// whole-valued float was not, and the merge below claimed a rule its own draws
// never reached.

// TestAMergeSharesAKeyWithWhatItMerges is the precedence rule's reachability.
//
// The 1.1 merge type says an entry's own keys win over the ones a "<<" brings,
// [Map.Decoded] and readings.legacyMap both implement it, and reading it the
// other way round was a real defect. None of that is exercised by a document
// where the two mappings share no key.
//
// Measured on 2026-09-08, before aliaser.mergeFrom took a name from the
// mapping's own entries: 110 mappings carried a merge over 4,000 drawn values
// and **none** of them shared a key, because mergeFrom named its keys "merged0"
// to "merged3" and the own keys came from Strings(). The comment in merge said
// the collision was "not avoided", which was true and describes a shape nothing
// reached.
func TestAMergeSharesAKeyWithWhatItMerges(t *testing.T) {
	values := yamlgen.Values()

	var merges, sharing int

	for i := range 2000 {
		walkMappings(values.Example(i), func(m yamlgen.Map) {
			own := map[string]bool{}

			var merged []yamlgen.Value

			for _, p := range m.Pairs {
				if _, isMerge := p.Key.(yamlgen.MergeKey); isMerge {
					merged = append(merged, p.Val)

					continue
				}

				own[yamlgen.KeyText(p.Key)] = true
			}

			if len(merged) == 0 {
				return
			}

			merges++

			for _, from := range merged {
				for _, name := range mergedKeyNames(from) {
					if own[name] {
						sharing++

						return
					}
				}
			}
		})
	}

	t.Logf("%d mappings carry a merge over 2000 drawn values, %d of them share a key with what they merge",
		merges, sharing)

	if merges == 0 {
		t.Fatal("no drawn value carries a merge, so nothing above was measured")
	}

	if sharing == 0 {
		t.Error("no merge shares a key with the mapping that holds it, so the precedence rule " +
			"is claimed by Map.Decoded and reached by no document")
	}
}

// TestAMergeSequenceSharesAKeyBetweenItsMappings is the other half of the merge
// rule's reachability.
//
// 1.1 merges a sequence of mappings with the earlier winning, and a sequence
// whose mappings share no key never asks which wins. mergeFrom draws each
// mapping independently, so the two collide when they take the same name --
// measured at 12 of 52 sequence merges over 4,000 drawn values on 2026-09-08,
// against the precedence half's zero, which is why only the other one needed
// fixing.
func TestAMergeSequenceSharesAKeyBetweenItsMappings(t *testing.T) {
	values := yamlgen.Values()

	var sequences, sharing int

	for i := range 2000 {
		walkMappings(values.Example(i), func(m yamlgen.Map) {
			for _, p := range m.Pairs {
				if _, isMerge := p.Key.(yamlgen.MergeKey); !isMerge {
					continue
				}

				seq, isSeq := peelForCounting(p.Val).(yamlgen.Seq)
				if !isSeq {
					continue
				}

				sequences++

				seen := map[string]bool{}

				for _, item := range seq.Items {
					for _, name := range mergedKeyNames(item) {
						if seen[name] {
							sharing++

							return
						}

						seen[name] = true
					}
				}
			}
		})
	}

	t.Logf("%d merges of a sequence over 2000 drawn values, %d of them share a key between its mappings",
		sequences, sharing)

	if sequences == 0 {
		t.Fatal("no merge takes a sequence, so nothing above was measured")
	}

	if sharing == 0 {
		t.Error("no merge sequence holds two mappings sharing a key, so the earlier-wins half of " +
			"the rule is claimed by Map.Decoded and reached by no document")
	}
}

// mergedKeyNames returns the names of the keys a "<<" value brings in.
func mergedKeyNames(v yamlgen.Value) []string {
	switch n := peelForCounting(v).(type) {
	case yamlgen.Map:
		out := make([]string, 0, len(n.Pairs))
		for _, p := range n.Pairs {
			out = append(out, yamlgen.KeyText(p.Key))
		}

		return out
	case yamlgen.Seq:
		var out []string
		for _, item := range n.Items {
			out = append(out, mergedKeyNames(item)...)
		}

		return out
	}

	return nil
}

// walkMappings calls fn for every mapping in a value, keys included.
func walkMappings(v yamlgen.Value, fn func(yamlgen.Map)) {
	switch n := peelForCounting(v).(type) {
	case yamlgen.Map:
		fn(n)

		for _, p := range n.Pairs {
			walkMappings(p.Key, fn)
			walkMappings(p.Val, fn)
		}
	case yamlgen.Seq:
		for _, item := range n.Items {
			walkMappings(item, fn)
		}
	}
}

// peelForCounting returns the node an anchor, an alias and a tag stand in front
// of.
func peelForCounting(v yamlgen.Value) yamlgen.Value {
	for {
		switch n := v.(type) {
		case yamlgen.Anchored:
			v = n.V
		case yamlgen.Alias:
			v = n.V
		case yamlgen.Tagged:
			v = n.V
		default:
			return v
		}
	}
}
