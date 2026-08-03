// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"regexp"
	"strings"

	"pgregory.net/rapid"
)

// Mutate breaks a document, and says which way it broke it.
//
// It is the generator for the question [Emit] cannot ask. Everything Emit
// writes is valid YAML 1.2 -- deliberately, and checked -- so the library
// accepting it is never news. Finding what the library accepts that it should
// not needs documents that are not YAML, and this makes them.
//
// It makes no claim that the result is invalid. It cannot: YAML is nearly
// total, and most ways of disturbing a document leave another perfectly good
// one. That is what the recognizer is for -- the caller asks it, and throws
// away everything that is still YAML. The mutations here only have to be
// plausible ways of getting it wrong, and cheap enough to run a hundred
// thousand times.
//
// The name comes back so that a run can report which mutations are producing
// anything. One that never yields an invalid document is either impossible or
// broken, and both are worth knowing.
//
// # What the oracle cannot see
//
// There was a mutation here that left an alias pointing at no anchor. Over
// twenty thousand draws it produced not one document the recognizer refused,
// which is correct and worth writing down: the grammar says what a document
// looks like, and nothing more. An alias resolving to an anchor, keys in a
// mapping being distinct, a tag having a meaning -- none of these are in it.
// A mutation aimed at one of those will be filtered out as valid, so those
// classes need an oracle this package does not have.
func Mutate(t *rapid.T, src string) (string, string) {
	i := rapid.IntRange(0, len(mutations)-1).Draw(t, "mutation")
	m := mutations[i]

	return m.apply(t, src), m.name
}

// MutationNames lists the mutations, so a test can report coverage against them.
func MutationNames() []string {
	out := make([]string, 0, len(mutations))
	for _, m := range mutations {
		out = append(out, m.name)
	}

	return out
}

type mutation struct {
	name  string
	apply func(*rapid.T, string) string
}

// The catalog is in two halves.
//
// The first half breaks a rule: indentation that does not line up, a quote that
// never closes, a header stating a width its content does not have. These are
// what a person writing YAML by hand gets wrong, and each is aimed at a
// production the library has to enforce.
//
// The second half breaks a byte, with no theory at all. It is there because the
// first half can only find the mistakes we thought of, and the recognizer makes
// an untargeted mutator affordable: it costs one call to learn that a mutant is
// still YAML and can be dropped.
var mutations = []mutation{
	{"a-tab-in-the-indentation", func(t *rapid.T, src string) string {
		i := spotIn(t, src, "indent", func(i int) bool {
			return src[i] == ' ' && onlySpacesBefore(src, i)
		})
		if i < 0 {
			return src
		}

		return src[:i] + "\t" + src[i+1:]
	}},

	{"a-line-moved-sideways", func(t *rapid.T, src string) string {
		// The first line is left alone: moving it sideways usually makes a
		// document that is invalid for the dullest possible reason.
		i := spotIn(t, src, "line", func(i int) bool { return i > 0 && src[i-1] == '\n' && i < len(src) })
		if i < 0 {
			return src
		}

		if src[i] == ' ' && rapid.Bool().Draw(t, "outdent") {
			return src[:i] + src[i+1:]
		}

		return src[:i] + " " + src[i:]
	}},

	{"an-unterminated-quote", func(t *rapid.T, src string) string {
		i := spotIn(t, src, "quote", func(i int) bool { return src[i] == '"' || src[i] == '\'' })
		if i < 0 {
			return src
		}

		return src[:i] + src[i+1:]
	}},

	{"an-unknown-escape", func(t *rapid.T, src string) string {
		i := spotIn(t, src, "escape", func(i int) bool { return src[i] == '\\' && i+1 < len(src) })
		if i < 0 {
			return src
		}

		return src[:i+1] + "q" + src[i+2:]
	}},

	{"a-document-marker-inside", func(t *rapid.T, src string) string {
		// Aimed at the content of a block scalar, which is the one place a
		// marker cannot go. At the top level it can go almost anywhere -- that
		// is what a marker is for -- so the untargeted version of this spent
		// almost all of its draws writing another perfectly good document.
		i := spotIn(t, src, "line", func(i int) bool {
			return i > 0 && src[i-1] == '\n' && endsOnABlockHeader(src[:i-1])
		})
		if i < 0 {
			return src
		}

		marker := rapid.SampledFrom([]string{"---\n", "...\n"}).Draw(t, "marker")

		return src[:i] + marker + src[i:]
	}},

	{"a-stray-indicator", func(t *rapid.T, src string) string {
		if src == "" {
			return src
		}

		i := rapid.IntRange(0, len(src)-1).Draw(t, "at")
		c := rapid.SampledFrom([]string{"@", "`", "[", "]", "{", "}", ",", "%", "\t", ": ", " #"}).
			Draw(t, "indicator")

		return src[:i] + c + src[i:]
	}},

	{"a-wider-indentation-indicator", func(t *rapid.T, src string) string {
		i := spotIn(t, src, "header", func(i int) bool { return src[i] == '|' || src[i] == '>' })
		if i < 0 {
			return src
		}

		digit := rapid.SampledFrom([]string{"6", "7", "8", "9"}).Draw(t, "digit")
		if i+1 < len(src) && src[i+1] >= '1' && src[i+1] <= '9' {
			return src[:i+1] + digit + src[i+2:]
		}

		return src[:i+1] + digit + src[i+1:]
	}},

	{"the-document-cut-short", func(t *rapid.T, src string) string {
		if src == "" {
			return src
		}

		return src[:rapid.IntRange(0, len(src)-1).Draw(t, "at")]
	}},

	{"a-byte-dropped", func(t *rapid.T, src string) string {
		if src == "" {
			return src
		}

		i := rapid.IntRange(0, len(src)-1).Draw(t, "at")

		return src[:i] + src[i+1:]
	}},

	{"a-byte-doubled", func(t *rapid.T, src string) string {
		if src == "" {
			return src
		}

		i := rapid.IntRange(0, len(src)-1).Draw(t, "at")

		return src[:i] + string(src[i]) + src[i:]
	}},

	{"an-awkward-byte-inserted", func(t *rapid.T, src string) string {
		if src == "" {
			return src
		}

		i := rapid.IntRange(0, len(src)-1).Draw(t, "at")
		c := rapid.SampledFrom(awkwardBytes).Draw(t, "byte")

		return src[:i] + c + src[i:]
	}},
}

// awkwardBytes are the ones YAML gives a meaning to, plus the ones it forbids
// outright. A uniformly random byte is almost always an ordinary letter, which
// leaves the document valid and the call wasted.
var awkwardBytes = []string{
	"\t", "\n", "\r", " ", "\x00", "\x1b", "\x7f", "\ufeff",
	"-", "?", ":", ",", "[", "]", "{", "}", "#", "&", "*", "!", "|", ">",
	"'", "\"", "%", "@", "`", "\\",
}

// spotIn draws one of the positions where pred holds, or reports -1.
//
// Drawing from the candidates rather than drawing a position and testing it is
// what keeps a mutation from mostly declining to fire: a document has few tabs
// worth making and many bytes.
func spotIn(t *rapid.T, src, label string, pred func(int) bool) int {
	var spots []int
	for i := range len(src) {
		if pred(i) {
			spots = append(spots, i)
		}
	}

	if len(spots) == 0 {
		return -1
	}

	return rapid.SampledFrom(spots).Draw(t, label)
}

func onlySpacesBefore(src string, i int) bool {
	start := strings.LastIndexByte(src[:i], '\n') + 1

	return strings.TrimLeft(src[start:i], " ") == ""
}

// blockHeader matches a line whose last token opens a block scalar, so that
// what follows it is content rather than another node.
var blockHeader = regexp.MustCompile(`[|>][0-9]?[-+]?$`)

func endsOnABlockHeader(upTo string) bool {
	start := strings.LastIndexByte(upTo, '\n') + 1

	return blockHeader.MatchString(upTo[start:])
}
