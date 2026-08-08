// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus

import (
	"math/rand/v2"

	"github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"
)

// Generating documents for the corpus.
//
// # Why this is not yamlgen's generator
//
// yamlgen already draws values and styles, and this reuses neither of its
// generators. They are rapid's, drawn from a *rapid.T inside a property check,
// and rapid owns the randomness -- which is right for a property test and fatal
// for an artifact. A corpus claims a seed reproduces it byte for byte, and that
// claim cannot be made about a stream somebody else controls. It is the same
// objection that kept the fuzzer's corpus out of artifact construction.
//
// What is reused is everything that matters: [yamlgen.Value], [yamlgen.Style]
// and [yamlgen.Emit]. The emitter is the part that is hard to get right and
// dangerous to get wrong -- a second one would write documents whose meaning we
// had computed incorrectly, and then blame the library for the difference. Only
// the drawing is written again, and drawing is cheap.

// Entry is one generated document, with where it came from.
type Entry struct {
	// Name identifies it, and is stable for a given seed.
	Name string
	// Src is the document.
	Src []byte
	// Value is what it means, known because the value came first. Nil for a
	// mutant, which means whatever the mutation left behind.
	Value yamlgen.Value
	// Mutation names how it was broken, empty for a document emitted whole.
	Mutation string
}

// Generate draws documents from a seed and breaks each of them.
//
// Meaning first, then presentation: a value is drawn, a style is drawn, and the
// document is what the emitter writes. That order is what makes the expectation
// free -- generating text first would leave nothing to compare a decode against.
func Generate(seed uint64, documents, mutantsEach int) []Entry {
	rng := rand.New(rand.NewPCG(seed, 0x59414d4c))

	out := make([]Entry, 0, documents*(1+mutantsEach))

	for i := range documents {
		value := randomValue(rng, 0)
		src := []byte(yamlgen.Emit(value, randomStyle(rng)))

		out = append(out, Entry{
			Name:  "generated/" + digits(i),
			Src:   src,
			Value: value,
		})

		for j := range mutantsEach {
			broken, how := mutate(rng, src)
			out = append(out, Entry{
				Name:     "generated/" + digits(i) + "/" + how + "/" + digits(j),
				Src:      broken,
				Mutation: how,
			})
		}
	}

	return out
}

// digits renders a number without pulling in a formatter, so that a name is
// built the same way every time regardless of locale or format verb.
func digits(n int) string {
	if n == 0 {
		return "0"
	}

	var out []byte

	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}

	return string(out)
}

// randomValue draws a value, narrowing as it descends so that a tree
// terminates.
//
// Depth is bounded rather than left to chance because the corpus is regenerated
// on every test run: an unlucky seed producing a document a megabyte deep would
// not be a finding, it would just be slow forever.
func randomValue(rng *rand.Rand, depth int) yamlgen.Value {
	kinds := 7
	if depth >= 3 {
		// Past here, scalars only, so the recursion has an end.
		kinds = 5
	}

	switch rng.IntN(kinds) {
	case 0:
		return yamlgen.Null{}
	case 1:
		return yamlgen.Bool{V: rng.IntN(2) == 0}
	case 2:
		return yamlgen.Int{V: rng.IntN(2001) - 1000}
	case 3:
		return yamlgen.Float{V: float64(rng.IntN(2001)-1000) / 8}
	case 4:
		return yamlgen.Str{V: randomString(rng)}
	case 5:
		items := make([]yamlgen.Value, 0, 3)
		for range rng.IntN(4) {
			items = append(items, randomValue(rng, depth+1))
		}

		return yamlgen.Seq{Items: items}
	default:
		pairs := make([]yamlgen.Pair, 0, 3)
		seen := map[string]bool{}

		for range rng.IntN(4) {
			key := randomKey(rng)
			if seen[key] {
				continue
			}

			seen[key] = true
			pairs = append(pairs, yamlgen.Pair{Key: key, Val: randomValue(rng, depth+1)})
		}

		return yamlgen.Map{Pairs: pairs}
	}
}

// alphabet is what a generated string is drawn from.
//
// Every entry is here because it is awkward somewhere. A break decides between
// a literal and a folded scalar; a leading space defeats indentation detection;
// "#" opens a comment unless something precedes it; the indicators are the
// characters the published grammar under-constrains. A generator drawing from
// letters would produce documents that are all the same document.
var alphabet = []rune{
	'a', 'b', 'z', 'A', 'Z', '0', '9',
	' ', '\t', '\n',
	'#', ':', '-', '?', '&', '*', '!', '|', '>', '%', '@', '`',
	'\'', '"', '\\', '[', ']', '{', '}', ',',
	'é',          // a letter that is two bytes, so a byte mutation can split it
	'中',          // three bytes
	'\U0001f600', // four, and outside the basic plane
	'',          // a next line, which YAML counts as a break and Go does not
	' ',          // a non-breaking space, which is not s-white and looks like it
}

func randomString(rng *rand.Rand) string {
	out := make([]rune, 0, 8)
	for range rng.IntN(9) {
		out = append(out, alphabet[rng.IntN(len(alphabet))])
	}

	return string(out)
}

// randomKey draws a mapping key, which is a string with no break in it.
//
// A key holding a line break is a different question -- it forces a quoted or
// an explicit key -- and one this generator does not raise, because the emitter
// declines to write those and a document it declines to write is not a
// document.
func randomKey(rng *rand.Rand) string {
	out := make([]rune, 0, 6)

	for range 1 + rng.IntN(5) {
		r := alphabet[rng.IntN(len(alphabet))]
		if r == '\n' || r == '' {
			r = 'k'
		}

		out = append(out, r)
	}

	return string(out)
}

func randomStyle(rng *rand.Rand) yamlgen.Style {
	nulls := []string{"null", "~", "", "Null", "NULL"}

	return yamlgen.Style{
		Flow:           rng.IntN(2) == 0,
		Indent:         1 + rng.IntN(6),
		Quoting:        yamlgen.Quoting(rng.IntN(3)),
		Literal:        rng.IntN(2) == 0,
		Folded:         rng.IntN(2) == 0,
		BlockIndicator: rng.IntN(2) == 0,
		Markers:        rng.IntN(2) == 0,
		NullSpelling:   nulls[rng.IntN(len(nulls))],
		BoolCase:       rng.IntN(3),
		Comments:       yamlgen.Commenting(rng.IntN(4)),
	}
}
