// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

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

// under returns what v denotes under a reading, and whether that differs from
// the core answer.
func (r *readings) under(v Value) (any, bool) {
	if r == nil || len(r.plain) == 0 {
		return nil, false
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
	case Seq:
		out := make([]any, 0, len(n.Items))
		for _, item := range n.Items {
			out = append(out, r.legacy(item))
		}

		return out
	case Map:
		out := make(map[string]any, len(n.Pairs))
		for _, p := range n.Pairs {
			out[p.Key] = r.legacy(p.Val)
		}

		return out
	case Anchored:
		return r.legacy(n.V)
	case Alias:
		return r.legacy(n.V)
	case Tagged:
		// A tag settles the type, so nothing under it resolves by spelling.
		return n.Decoded()
	default:
		return v.Decoded()
	}
}
