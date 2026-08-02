// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"math"
	"sort"

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
	// Float is a floating point number, excluding the infinities and NaN,
	// whose spellings are their own conformance question.
	Float struct{ V float64 }
	// Str is a string, and the interesting one: most of the ways to write a
	// YAML document differently are ways to write a string differently.
	Str struct{ V string }
	// Seq is an ordered list.
	Seq struct{ Items []Value }
	// Map is an ordered set of pairs with distinct keys. Order is kept so that
	// emitting is deterministic; it carries no meaning.
	Map struct{ Pairs []Pair }
)

// Pair is one mapping entry. Keys are strings because that is what decoding
// into an `any` produces, whatever the document said.
type Pair struct {
	Key string
	Val Value
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

func (f Float) Decoded() any { return f.V }
func (s Str) Decoded() any   { return s.V }

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

func (m Map) Decoded() any {
	out := make(map[string]any, len(m.Pairs))
	for _, p := range m.Pairs {
		out[p.Key] = p.Val.Decoded()
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

// Values generates a Value tree.
func Values() *rapid.Generator[Value] {
	return values(0)
}

const maxDepth = 3

func values(depth int) *rapid.Generator[Value] {
	return rapid.Deferred(func() *rapid.Generator[Value] {
		scalars := []*rapid.Generator[Value]{
			rapid.Just(Value(Null{})),
			rapid.Custom(func(t *rapid.T) Value { return Bool{V: rapid.Bool().Draw(t, "bool")} }),
			rapid.Custom(func(t *rapid.T) Value { return Int{V: rapid.IntRange(-1000, 1000).Draw(t, "int")} }),
			rapid.Custom(func(t *rapid.T) Value { return Float{V: floats().Draw(t, "float")} }),
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

	return Seq{Items: items}
}

func drawMap(t *rapid.T, depth int) Value {
	n := rapid.IntRange(0, 4).Draw(t, "pairs")

	// Distinct keys: a document with a duplicate key is a different question
	// than the one this generator asks.
	seen := make(map[string]struct{}, n)
	pairs := make([]Pair, 0, n)
	for range n {
		key := Strings().Draw(t, "key")
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		pairs = append(pairs, Pair{Key: key, Val: values(depth+1).Draw(t, "value")})
	}

	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Key < pairs[j].Key })

	return Map{Pairs: pairs}
}

// Strings generates a string, weighted toward the ones that are awkward to
// write down.
func Strings() *rapid.Generator[string] {
	return rapid.OneOf(
		rapid.SampledFrom(awkwardStrings),
		rapid.SampledFrom(awkwardStrings),
		rapid.StringMatching(`[a-zA-Z0-9 _.:#-]{0,12}`),
		rapid.String(),
	)
}

// floats avoids the infinities and NaN. They are worth testing and they are a
// separate question: their spelling is schema-dependent, so a disagreement
// there says nothing about presentation invariance.
func floats() *rapid.Generator[float64] {
	return rapid.Custom(func(t *rapid.T) float64 {
		f := rapid.Float64Range(-1e6, 1e6).Draw(t, "f")
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return 0
		}

		return f
	})
}
