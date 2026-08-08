// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus

import "math/rand/v2"

// Breaking a document.
//
// # Deliberately generic
//
// None of these knows anything about YAML. That is the point: a mutation that
// understood the grammar would produce the errors we already thought of, and
// the corpus exists for the ones we did not. The recognizer sorts the results,
// so a mutation that leaves a perfectly good document costs a verdict and
// nothing else.
//
// YAML makes that cheap and also makes it necessary. Nearly every disturbance
// of a YAML document leaves another valid document -- the language is close to
// total over plain text -- so a generator of *invalid* documents cannot be
// aimed. It has to be a generator of *different* documents, filtered.
//
// # The indentation mutations are the exception
//
// Two of these do know one thing: that a line has leading spaces and that
// changing them changes what a document means. They are here because a uniform
// byte mutation almost never lands on the indentation of a line -- there are
// far more content bytes than leading spaces -- and indentation is where this
// library and this recognizer have both had their bugs. Left to chance the
// corpus would be thin exactly where it should be thick.

// mutate breaks a document one way and says which way.
func mutate(rng *rand.Rand, src []byte) ([]byte, string) {
	if len(src) == 0 {
		return src, "nothing to break"
	}

	kinds := []struct {
		name string
		fn   func(*rand.Rand, []byte) []byte
	}{
		{"delete a byte", deleteByte},
		{"delete a run", deleteRun},
		{"duplicate a byte", duplicateByte},
		{"swap neighbors", swapNeighbours},
		{"truncate", truncate},
		{"substitute an indicator", substituteIndicator},
		{"insert an indicator", insertIndicator},
		{"substitute a byte", substituteByte},
		{"indent a line further", indentLine},
		{"outdent a line", outdentLine},
	}

	pick := kinds[rng.IntN(len(kinds))]

	return pick.fn(rng, src), pick.name
}

func deleteByte(rng *rand.Rand, src []byte) []byte {
	at := rng.IntN(len(src))

	return concat(src[:at], src[at+1:])
}

func deleteRun(rng *rand.Rand, src []byte) []byte {
	at := rng.IntN(len(src))
	end := min(at+1+rng.IntN(8), len(src))

	return concat(src[:at], src[end:])
}

func duplicateByte(rng *rand.Rand, src []byte) []byte {
	at := rng.IntN(len(src))

	return concat(src[:at+1], src[at:at+1], src[at+1:])
}

func swapNeighbours(rng *rand.Rand, src []byte) []byte {
	if len(src) < 2 {
		return src
	}

	at := rng.IntN(len(src) - 1)

	return concat(src[:at], src[at+1:at+2], src[at:at+1], src[at+2:])
}

func truncate(rng *rand.Rand, src []byte) []byte {
	return concat(src[:rng.IntN(len(src))])
}

// indicators are the bytes YAML gives a meaning to, taken from the spec's own
// list rather than invented.
//
// Substituting one of these is far likelier to change what a document means
// than substituting an arbitrary byte, and changing the meaning is the whole
// job. The rest of the byte space is covered by substituteByte.
var indicators = []byte{
	'-', '?', ':', ',', '[', ']', '{', '}', '#', '&', '*', '!', '|', '>',
	'\'', '"', '%', '@', '`', ' ', '\t', '\n', '\\',
}

func substituteIndicator(rng *rand.Rand, src []byte) []byte {
	at := rng.IntN(len(src))
	out := concat(src)
	out[at] = indicators[rng.IntN(len(indicators))]

	return out
}

func insertIndicator(rng *rand.Rand, src []byte) []byte {
	at := rng.IntN(len(src) + 1)

	return concat(src[:at], []byte{indicators[rng.IntN(len(indicators))]}, src[at:])
}

func substituteByte(rng *rand.Rand, src []byte) []byte {
	at := rng.IntN(len(src))
	out := concat(src)
	out[at] = byte(rng.IntN(256))

	return out
}

// indentLine adds spaces to the front of one line, and outdentLine takes them
// away.
//
// The only two mutations here that know what they are looking at. A uniform
// byte mutation lands on a line's leading spaces about as often as leading
// spaces occur, which is rarely, and every indentation defect either library
// has had would be reached only by such a landing.
func indentLine(rng *rand.Rand, src []byte) []byte {
	at, ok := lineStart(rng, src)
	if !ok {
		return src
	}

	pad := make([]byte, 1+rng.IntN(4))
	for i := range pad {
		pad[i] = ' '
	}

	return concat(src[:at], pad, src[at:])
}

func outdentLine(rng *rand.Rand, src []byte) []byte {
	at, ok := lineStart(rng, src)
	if !ok {
		return src
	}

	end := at
	for end < len(src) && src[end] == ' ' {
		end++
	}

	if end == at {
		return src
	}

	drop := 1 + rng.IntN(end-at)

	return concat(src[:at], src[at+drop:])
}

// lineStart picks the beginning of one line, reporting false where the document
// has none to pick.
func lineStart(rng *rand.Rand, src []byte) (int, bool) {
	starts := []int{0}

	for i, b := range src {
		if b == '\n' && i+1 < len(src) {
			starts = append(starts, i+1)
		}
	}

	if len(starts) == 0 {
		return 0, false
	}

	return starts[rng.IntN(len(starts))], true
}
