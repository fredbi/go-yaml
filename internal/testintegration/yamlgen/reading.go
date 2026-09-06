// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import "strings"

// What a document denotes under a reading other than YAML 1.2's core schema.
//
// # Only plain scalars raise the question
//
// A quoted scalar is a string under every reading, a block scalar is a string
// under every reading, and a tag settles the type outright. So the question is
// asked exactly where a plain scalar is written, and the answer depends on how
// the emitter chose to write it rather than on the Value alone: Style.Literal
// turns Str{"yes"} into a literal block scalar, which is the string "yes"
// everywhere and raises nothing.
//
// That is why the emitter records this as it writes and Write hands it back,
// the same way it hands back the features.
//
// # The table is small because plainSafe is strict
//
// plainSafe requires a leading letter or underscore, which double-quotes every
// numeric spelling the schemas disagree about -- "0777", "1_000", "0b1010",
// "1:30", "0x1A", "1e3", ".inf" and "2001-12-14" all come out in quotes and
// mean the empty question. What is left is YAML 1.1's boolean words, which 1.2
// reads as strings. yamlcorpus's Resolutions carries the numeric ones as
// enumerated shapes, where the answers are written by hand.
//
// Widening plainSafe is what would grow this table, and the two go together:
// the comment there says being wrong would mean "generating documents whose
// expected value we got wrong", and a spelling whose readings are written down
// here is a spelling that can be let through.

// The readings a corpus can state a meaning under.
const (
	// ReadingCore is YAML 1.2's core schema, spec §10.3, and the default.
	ReadingCore = "yaml-1.2-core"
	// Reading11 is YAML 1.1's resolution, still widely implemented.
	Reading11 = "yaml-1.1"
	// Reading11Version is what a document writes in its "%YAML" directive to
	// ask for that reading, which is the version and not the reading's name.
	Reading11Version = "1.1"
	// ReadingJSON is YAML 1.2's JSON schema, spec §10.2, which resolves what
	// JSON resolves and reads everything else as a string.
	//
	// Nothing here produces it: the spellings it parts company with core over
	// are all numeric, and plainSafe writes those in quotes. yamlcorpus's
	// enumerated scalars carry it.
	ReadingJSON = "yaml-1.2-json"
)

// legacyBooleans are the plain spellings YAML 1.1 reads as booleans and YAML
// 1.2 reads as strings.
//
// The whole of 1.1's bool production less the six spellings 1.2 also reads as
// booleans -- true, True, TRUE, false, False, FALSE -- which agree and so raise
// nothing. canPlain refuses none of these: plainSafe takes them all and
// `resolving` holds only the null and boolean words 1.2 itself resolves.
var legacyBooleans = map[string]bool{
	"y": true, "Y": true, "yes": true, "Yes": true, "YES": true,
	"on": true, "On": true, "ON": true,
	"n": false, "N": false, "no": false, "No": false, "NO": false,
	"off": false, "Off": false, "OFF": false,
}

// readings is what an emitter learns about resolution while it writes.
//
// plain holds the divergent spellings written as plain scalars and split holds
// the ones written both ways in one document. A document that writes "yes"
// plain in one place and as a literal block scalar in another has two answers
// under YAML 1.1 for one text, and this package will not guess which node the
// caller meant -- so it drops the alternate reading for that document rather
// than substituting both.
type readings struct {
	plain map[string]bool
	split map[string]bool
	// numbers holds the numbers written in a form YAML 1.1 does not read,
	// under the text they were written as. Where core reads a number and 1.1
	// reads the text back as a string, that text is the 1.1 answer.
	numbers map[string]bool
	// st is the presentation, which decides how a number was written and so
	// what 1.1 makes of it. The booleans above need no style; a number's
	// spelling is the whole question here.
	st Style
}

// numberUnder11 reports whether YAML 1.1 reads this text as the number the core
// schema reads.
//
// Three of the forms part company with 1.1. It has no "0o" prefix -- its octal
// is a bare leading zero -- so "0o37" is a string there. Its float production
// requires a '.' and, where an exponent is written, a sign on it, so "1e-320"
// and "1.5e0" are strings while "1.5e+00" is the float. The decimal, "+" and
// "0x" forms it reads exactly as core does.
//
// Written out rather than derived, and checked against the library's own 1.1
// reader by TestTheElevenAnswersMatchTheLibrary: being wrong here would mean
// stating a meaning no implementation holds.
func numberUnder11(text string) bool {
	body := strings.TrimLeft(text, "+-")

	if strings.HasPrefix(body, "0o") {
		return false
	}

	if strings.HasPrefix(body, "0x") {
		// 1.1 reads hex, and a hex digit may be an 'e' -- "0x3e8" is the
		// integer 1000 and not an exponent. The test below caught this.
		return true
	}

	exp := strings.IndexAny(body, "eE")
	if exp < 0 {
		return true
	}

	// A 1.1 float needs a '.' before the exponent and a sign on it.
	if !strings.Contains(body[:exp], ".") {
		return false
	}

	rest := body[exp+1:]

	return strings.HasPrefix(rest, "+") || strings.HasPrefix(rest, "-")
}

// sawNumber records a number whose text YAML 1.1 does not read as a number.
//
// The form is not consulted, and consulting it was wrong: NumberPlain looked
// like the form the two schemas always agree on, and a BigFloat breaks that.
// Its text comes from big.Float.Text('g', -1), which writes "1e+330" -- an
// exponent with no '.' before it, which 1.1 reads as a string. The text is the
// only thing that decides.
func (r *readings) sawNumber(text string) {
	if r == nil || numberUnder11(text) {
		return
	}

	r.numbers[text] = true
}

// sawScalar records how one scalar was written.
func (r *readings) sawScalar(text string, plain bool) {
	if r == nil {
		return
	}

	if _, diverges := legacyBooleans[text]; !diverges {
		return
	}

	if was, seen := r.plain[text]; seen && was != plain {
		r.split[text] = true
	}

	if !plain {
		if _, seen := r.plain[text]; !seen {
			r.plain[text] = false
		}

		return
	}

	r.plain[text] = true
}

// splitALegacySpelling reports whether a text the readings disagree about was
// written plain in one place and some other way in another.
//
// Which occurrence resolved is then a question about nodes, and this tracks
// spellings.
func (r *readings) splitALegacySpelling() bool {
	if r == nil {
		return false
	}

	for text := range r.split {
		if r.plain[text] {
			return true
		}
	}

	return false
}

// under returns what v denotes under a reading, and whether that differs from
// the core answer.
func (r *readings) under(v Value) (any, bool) {
	if r == nil || (len(r.plain) == 0 && len(r.numbers) == 0) {
		return nil, false
	}

	if len(r.numbers) > 0 {
		return r.legacy(v), true
	}

	var swapped bool

	for text, plain := range r.plain {
		if plain && !r.split[text] {
			swapped = true

			break
		}
	}

	if !swapped {
		return nil, false
	}

	return r.legacy(v), true
}

// numberKey is [KeyText] under YAML 1.1 for a number written in a form 1.1
// does not read: the key is named by the text rather than by the number.
func (r *readings) numberKey(v Value) (string, bool) {
	return r.numberText(v)
}

// numberText returns the text a number was written as and whether 1.1 reads it
// back as that text rather than as the number.
func (r *readings) numberText(v Value) (string, bool) {
	var text string

	switch n := v.(type) {
	case Int:
		text, _ = intText(n.V, r.st)
	case BigInt:
		text, _ = bigIntText(n.V, r.st)
	case Float:
		text, _ = floatText(n.V, r.st)
	case BigFloat:
		text = bigFloatText(n.V)
	default:
		return "", false
	}

	return text, r.numbers[text]
}

// legacyKey is [KeyText] under YAML 1.1.
//
// A key resolves before it is stringified, so a plain "yes:" is the key "true"
// under 1.1 and the key "yes" under 1.2 -- the same divergence as a value, in
// the one place where getting it wrong would put a wrong key in a stated
// meaning rather than a wrong value.
func (r *readings) legacyKey(v Value) string {
	if n, ok := v.(Str); ok {
		if b, diverges := legacyBooleans[n.V]; diverges && r.plain[n.V] && !r.split[n.V] {
			if b {
				return "true"
			}

			return "false"
		}
	}

	if text, diverges := r.numberKey(v); diverges {
		return text
	}

	return KeyText(v)
}

// legacy rebuilds the decoded value with the divergent plain scalars read as
// YAML 1.1 reads them.
//
// An Alias decodes to what its anchor stands for, so the substitution has to
// reach through it as well; Decoded already does that and this mirrors it.
func (r *readings) legacy(v Value) any {
	switch n := v.(type) {
	case Str:
		if b, diverges := legacyBooleans[n.V]; diverges && r.plain[n.V] && !r.split[n.V] {
			return b
		}

		return n.V
	case Int, BigInt, Float, BigFloat:
		if text, diverges := r.numberText(n); diverges {
			return text
		}

		return n.Decoded()
	case Seq:
		out := make([]any, 0, len(n.Items))
		for _, item := range n.Items {
			out = append(out, r.legacy(item))
		}

		return out
	case Map:
		out := make(map[string]any, len(n.Pairs))
		for _, p := range n.Pairs {
			out[r.legacyKey(p.Key)] = r.legacy(p.Val)
		}

		return out
	case Anchored:
		return r.legacy(n.V)
	case Alias:
		return r.legacy(n.V)
	case Tagged:
		// A tag settles the type of the node it stands on, and of that node
		// only. On a scalar it means the spelling no longer decides, so the
		// core answer stands. On a collection it says the node is a sequence
		// or a mapping, which the scalars inside are none the wiser for --
		// "!!map" over "k: 0o0" leaves the 0o0 resolving by spelling, and
		// stopping here read it as the core schema does under a directive
		// asking for 1.1.
		switch n.V.(type) {
		case Seq, Map:
			return r.legacy(n.V)
		default:
			return n.Decoded()
		}
	default:
		return v.Decoded()
	}
}
