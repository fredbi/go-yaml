// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/go-openapi/go-yaml/internal/testintegration/stance"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"
)

// Holding the feature labels to what the emitter actually wrote.
//
// A Feature cites no specification and settles nothing, so nothing in the type
// keeps the vocabulary honest. This file is what does, and it checks three
// separate things because they fail in three separate ways:
//
//   - a label recorded for text nobody wrote -- caught by requiring a mark in
//     the bytes for every label;
//   - a label recorded under a style that cannot produce it -- caught by
//     requiring the style to permit it;
//   - text written and never labeled -- caught by reading the marks back and
//     demanding the label, which is the direction that matters most and the
//     one a naive scan gets wrong.
//
// The third is sound only where a mark cannot also be scalar content. A "[" in
// a double-quoted string is not a flow collection. So it is asked only of the
// documents whose Value holds no scalar carrying the mark, which is most of
// them, and the check is skipped for the rest rather than guessed at.

// marks pairs a feature with the character that has to appear in the document
// when the emitter claims it.
//
// Some are one character and exact; some are the best a scan can do without
// parsing. The reverse column says which.
type mark struct {
	feature stance.Feature
	// in reports whether the text shows the mark.
	in func(string) bool
	// rune is the character the reverse direction reads back, and the one a
	// scalar must not contain for that direction to be sound. Zero where the
	// reverse direction is not asked.
	rune rune
}

func marks() []mark {
	has := func(sub string) func(string) bool {
		return func(s string) bool { return strings.Contains(s, sub) }
	}

	return []mark{
		{feature: yamlgen.FeatureDocumentMarker, in: func(s string) bool { return strings.HasPrefix(s, "---") }},
		{feature: yamlgen.FeatureCommentAbove, in: has("#"), rune: '#'},
		{feature: yamlgen.FeatureCommentInline, in: has("#"), rune: '#'},
		{feature: yamlgen.FeatureFlowCollection, in: func(s string) bool {
			return strings.ContainsAny(s, "[{")
		}, rune: '['},
		{feature: yamlgen.FeatureFlowPair, in: has("[")},
		{feature: yamlgen.FeatureFlowEmptyValue, in: func(s string) bool {
			return strings.Contains(s, ": ,") || strings.Contains(s, ": }")
		}},
		{feature: yamlgen.FeatureFlowKeyAlone, in: has("{")},
		{feature: yamlgen.FeatureBlockLiteral, in: has("|"), rune: '|'},
		{feature: yamlgen.FeatureBlockFolded, in: has(">"), rune: '>'},
		{feature: yamlgen.FeatureBlockIndicator, in: func(s string) bool {
			return strings.ContainsAny(s, "|>")
		}},
		{feature: yamlgen.FeaturePropertyLine, in: func(s string) bool {
			return strings.ContainsAny(s, "&!")
		}},
		{feature: yamlgen.FeatureTagBeforeAnchor, in: func(s string) bool {
			return strings.Contains(s, "&") && strings.Contains(s, "!")
		}},
		{feature: yamlgen.FeatureQuotedSingle, in: has("'"), rune: '\''},
		{feature: yamlgen.FeatureQuotedDouble, in: has(`"`), rune: '"'},

		{feature: yamlgen.FeatureBreakCRLF, in: has("\r\n")},
		{feature: yamlgen.FeatureBreakCR, in: loneCR},

		{feature: yamlgen.FeatureAnchor, in: has("&"), rune: '&'},
		{feature: yamlgen.FeatureAlias, in: has("*"), rune: '*'},
	}
}

// loneCR reports a carriage return that no line feed follows, which is the one
// spelling of BreakCR.
func loneCR(s string) bool {
	for i := range len(s) {
		if s[i] == '\r' && (i+1 == len(s) || s[i+1] != '\n') {
			return true
		}
	}

	return false
}

// tagMarks are the four tag features, which share one mark: a tag is the only
// thing in a document that writes a "!".
func tagMarks() []stance.Feature {
	return []stance.Feature{
		yamlgen.FeatureTagShorthand,
		yamlgen.FeatureTagLocal,
		yamlgen.FeatureTagVerbatim,
		yamlgen.FeatureTagNonSpecific,
	}
}

// TestEveryLabelHasAMarkInTheBytes is the first direction: nothing is claimed
// that was not written.
func TestEveryLabelHasAMarkInTheBytes(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		v := yamlgen.Values().Draw(rt, "value")
		st := yamlgen.Styles().Draw(rt, "style")

		w := yamlgen.Write(v, st)

		for _, m := range marks() {
			if slices.Contains(w.Features, m.feature) && !m.in(w.Text) {
				rt.Fatalf("%s is claimed and nothing in the document shows it\n%q", m.feature, w.Text)
			}
		}

		for _, f := range tagMarks() {
			if slices.Contains(w.Features, f) && !strings.Contains(w.Text, "!") {
				rt.Fatalf("%s is claimed and the document writes no tag\n%q", f, w.Text)
			}
		}
	})
}

// TestNoLabelOutrunsItsStyle is the second direction: a feature the style
// cannot ask for was recorded in the wrong place.
//
// Only one way round. A style that asks for a construct often does not get it
// -- Style.Folded on a string holding a carriage return falls back to double
// quotes -- and that asymmetry is the whole reason the emitter records instead
// of the style being read off.
func TestNoLabelOutrunsItsStyle(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		v := yamlgen.Values().Draw(rt, "value")
		st := yamlgen.Styles().Draw(rt, "style")

		w := yamlgen.Write(v, st)

		permits := map[stance.Feature]bool{
			yamlgen.FeatureDocumentMarker:  st.Markers,
			yamlgen.FeatureCommentAbove:    st.Comments == yamlgen.HeadComments || st.Comments == yamlgen.AllComments,
			yamlgen.FeatureCommentInline:   st.Comments == yamlgen.LineComments || st.Comments == yamlgen.AllComments,
			yamlgen.FeatureFlowPair:        st.FlowPairs,
			yamlgen.FeatureFlowEmptyValue:  st.FlowEmpty == yamlgen.FlowNullEmpty,
			yamlgen.FeatureFlowKeyAlone:    st.FlowEmpty == yamlgen.FlowNullKeyAlone,
			yamlgen.FeatureBlockLiteral:    st.Literal,
			yamlgen.FeatureBlockFolded:     st.Folded,
			yamlgen.FeatureBlockIndicator:  st.BlockIndicator,
			yamlgen.FeaturePropertyLine:    st.PropertyLine,
			yamlgen.FeatureTagBeforeAnchor: st.PropertyOrder == yamlgen.TagFirst,
			yamlgen.FeatureQuotedSingle:    st.Quoting == yamlgen.QuoteSingle,
			yamlgen.FeatureBreakCRLF:       st.Break == yamlgen.BreakCRLF,
			yamlgen.FeatureBreakCR:         st.Break == yamlgen.BreakCR,
		}

		for _, f := range w.Features {
			if permitted, known := permits[f]; known && !permitted {
				rt.Fatalf("%s is claimed and %s does not write one\n%q", f, st, w.Text)
			}
		}
	})
}

// TestEveryMarkInTheBytesIsLabeled is the direction that matters: text written
// and never labeled.
//
// Asked only of the documents where the mark cannot be scalar content, since a
// "[" inside a double-quoted string is not a flow collection. The two break
// features are exempt from that guard and asked of every document: no scalar
// can carry a raw carriage return through this emitter -- doubleQuote escapes
// it, canSingle refuses it and writableRaw refuses it for both block styles --
// so a "\r" in the output came from the break and from nothing else.
func TestEveryMarkInTheBytesIsLabeled(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		v := yamlgen.Values().Draw(rt, "value")
		st := yamlgen.Styles().Draw(rt, "style")

		w := yamlgen.Write(v, st)

		if strings.Contains(w.Text, "\r\n") != slices.Contains(w.Features, yamlgen.FeatureBreakCRLF) {
			rt.Fatalf("a CRLF and its label disagree\n%q\n%v", w.Text, w.Features)
		}

		if loneCR(w.Text) != slices.Contains(w.Features, yamlgen.FeatureBreakCR) {
			rt.Fatalf("a lone carriage return and its label disagree\n%q\n%v", w.Text, w.Features)
		}

		if strings.HasPrefix(w.Text, "---") != slices.Contains(w.Features, yamlgen.FeatureDocumentMarker) {
			rt.Fatalf("a document marker and its label disagree\n%q\n%v", w.Text, w.Features)
		}

		// A flow collection writes both brackets, and one label covers them, so
		// the guard has to clear both characters before the read-back is sound.
		flow := yamlgen.FeatureFlowCollection

		if structural(v, "[{") && strings.ContainsAny(w.Text, "[{") && !slices.Contains(w.Features, flow) {
			rt.Fatalf("a flow collection is written and not labeled\n%q\n%v", w.Text, w.Features)
		}

		for _, m := range marks() {
			if m.rune == 0 || m.feature == flow {
				continue
			}

			if !structural(v, string(m.rune)) || !strings.ContainsRune(w.Text, m.rune) {
				continue
			}

			// The two comment features share the "#", so either one answers for
			// it.
			if m.rune == '#' {
				if !slices.Contains(w.Features, yamlgen.FeatureCommentAbove) &&
					!slices.Contains(w.Features, yamlgen.FeatureCommentInline) {
					rt.Fatalf("a comment is written and not labeled\n%q\n%v", w.Text, w.Features)
				}

				continue
			}

			if !slices.Contains(w.Features, m.feature) {
				rt.Fatalf("%s is written and not labeled\n%q\n%v", m.feature, w.Text, w.Features)
			}
		}

		if scalarFree(v, "!") && strings.Contains(w.Text, "!") {
			var tagged bool

			for _, f := range tagMarks() {
				tagged = tagged || slices.Contains(w.Features, f)
			}

			if !tagged {
				rt.Fatalf("a tag is written and not labeled\n%q\n%v", w.Text, w.Features)
			}
		}
	})
}

// TestTheCorpusReachesEveryFeature keeps a name in the vocabulary from becoming
// a name nothing produces.
//
// A dead constant is the quiet failure here: it looks like coverage, a consumer
// filters on it and gets an empty run, and the empty run reports as a pass.
func TestTheCorpusReachesEveryFeature(t *testing.T) {
	values := yamlgen.Values()
	styles := yamlgen.Styles()

	seen := map[stance.Feature]bool{}

	for i := range 4000 {
		w := yamlgen.Write(values.Example(i), styles.Example(i))
		for _, f := range w.Features {
			seen[f] = true
		}
	}

	for _, f := range everyFeature() {
		if !seen[f] {
			t.Errorf("%s is in the vocabulary and 4000 documents produced none", f)
		}
	}
}

func everyFeature() []stance.Feature {
	out := []stance.Feature{
		yamlgen.FeaturePlain,
		yamlgen.FeatureValueNull, yamlgen.FeatureValueBool, yamlgen.FeatureValueInt,
		yamlgen.FeatureValueFloat, yamlgen.FeatureValueString,
		yamlgen.FeatureValueSequence, yamlgen.FeatureValueMapping,
		yamlgen.FeatureValueEmptyCollection,
	}
	out = append(out, tagMarks()...)

	for _, m := range marks() {
		out = append(out, m.feature)
	}

	return out
}

// structural reports whether a mark found in the text can only have come from
// the document's structure.
//
// Two things in a document write characters that are not structure. A scalar
// writes whatever it holds, so a "[" inside a double-quoted string is not a
// flow collection. A tag spelling writes its own URI, and the verbatim form
// "!<tag:yaml.org,2002:str>" ends in a ">" that is not a folded block scalar --
// which is what the read-back caught on its 64th draw.
//
// The "!" read-back is asked separately and skips this, because a tag is
// exactly the thing it is looking for.
func structural(v yamlgen.Value, chars string) bool {
	return scalarFree(v, chars) && tagFree(v, chars)
}

// tagFree reports whether no tag spelling in v carries any of chars.
func tagFree(v yamlgen.Value, chars string) bool {
	switch n := v.(type) {
	case yamlgen.Tagged:
		return !strings.ContainsAny(n.Tag, chars) && tagFree(n.V, chars)
	case yamlgen.Anchored:
		return tagFree(n.V, chars)
	case yamlgen.Seq:
		for _, item := range n.Items {
			if !tagFree(item, chars) {
				return false
			}
		}
	case yamlgen.Map:
		for _, p := range n.Pairs {
			if !tagFree(p.Val, chars) {
				return false
			}
		}
	}

	return true
}

// scalarFree reports whether no scalar in v carries any of chars.
//
// Keys as well as values, and the string a Tagged or Anchored node stands on.
// Anchor names are not consulted: an anchor is structure and its name is drawn
// from letters and digits.
func scalarFree(v yamlgen.Value, chars string) bool {
	switch n := v.(type) {
	case yamlgen.Str:
		return !strings.ContainsAny(n.V, chars)
	case yamlgen.Seq:
		for _, item := range n.Items {
			if !scalarFree(item, chars) {
				return false
			}
		}
	case yamlgen.Map:
		for _, p := range n.Pairs {
			if strings.ContainsAny(p.Key, chars) || !scalarFree(p.Val, chars) {
				return false
			}
		}
	case yamlgen.Anchored:
		return scalarFree(n.V, chars)
	case yamlgen.Tagged:
		return scalarFree(n.V, chars)
	}

	return true
}
