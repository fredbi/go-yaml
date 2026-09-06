// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"reflect"
	"slices"
	"testing"

	"pgregory.net/rapid"

	"github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"
)

// plain is the style that writes a scalar unquoted wherever it can, which is
// the only style under which a spelling raises a resolution question.
func plainStyle() yamlgen.Style {
	return yamlgen.Style{Quoting: yamlgen.QuotePlain, Indent: 1, NullSpelling: "null"}
}

// TestAPlainLegacyBooleanGetsASecondReading is the case the field exists for.
func TestAPlainLegacyBooleanGetsASecondReading(t *testing.T) {
	v := yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "k"}, Val: yamlgen.Str{V: "yes"}}}}

	w := yamlgen.Write(v, plainStyle())

	if w.Text != "k: yes\n" {
		t.Fatalf("wrote %q, want %q", w.Text, "k: yes\n")
	}

	want := map[string]any{"k": true}
	if got := w.Readings[yamlgen.Reading11]; !reflect.DeepEqual(got, want) {
		t.Errorf("YAML 1.1 reads %#v, want %#v", got, want)
	}

	if got := v.Decoded(); !reflect.DeepEqual(got, map[string]any{"k": "yes"}) {
		t.Errorf("the core reading moved: %#v", got)
	}
}

// TestQuotingSettlesTheQuestion holds the reason the emitter records and the
// Style is not read off.
//
// The same Value under three styles: quoted twice and written as a block
// scalar once, and none of the three resolves to anything but a string.
func TestQuotingSettlesTheQuestion(t *testing.T) {
	v := yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "k"}, Val: yamlgen.Str{V: "yes"}}}}

	for _, st := range []yamlgen.Style{
		{Quoting: yamlgen.QuoteDouble, Indent: 1, NullSpelling: "null"},
		{Quoting: yamlgen.QuoteSingle, Indent: 1, NullSpelling: "null"},
		{Quoting: yamlgen.QuotePlain, Indent: 1, NullSpelling: "null", Literal: true},
	} {
		w := yamlgen.Write(v, st)
		if len(w.Readings) != 0 {
			t.Errorf("%q raises a resolution question and should not: %v", w.Text, w.Readings)
		}
	}
}

// TestATagSettlesTheQuestionToo checks the other way a spelling stops
// resolving.
func TestATagSettlesTheQuestionToo(t *testing.T) {
	v := yamlgen.Map{Pairs: []yamlgen.Pair{
		{Key: yamlgen.Str{V: "k"}, Val: yamlgen.Tagged{Tag: yamlgen.TagStr, V: yamlgen.Str{V: "yes"}}},
	}}

	if w := yamlgen.Write(v, plainStyle()); len(w.Readings) != 0 {
		t.Errorf("%q is tagged and should raise nothing: %v", w.Text, w.Readings)
	}
}

// TestOneTextWrittenTwoWaysDropsTheReading is the conservative case.
//
// "yes" plain in one entry and as a literal block scalar in another has two
// answers under YAML 1.1 for one spelling, and this package will not guess
// which node the caller meant.
func TestOneTextWrittenTwoWaysDropsTheReading(t *testing.T) {
	// FlowFrom 2 leaves the outer mapping's own values in block style, where a
	// literal block scalar is reachable, and puts the nested mapping's values
	// in flow, where it is not.
	st := plainStyle()
	st.Literal = true
	st.Flow = true
	st.FlowFrom = 2

	v := yamlgen.Map{Pairs: []yamlgen.Pair{
		{Key: yamlgen.Str{V: "block"}, Val: yamlgen.Str{V: "yes"}},
		{Key: yamlgen.Str{V: "flow"}, Val: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "k"}, Val: yamlgen.Str{V: "yes"}}}}},
	}}

	w := yamlgen.Write(v, st)
	if len(w.Readings) != 0 {
		t.Errorf("%q writes \"yes\" both ways and should state no second reading: %v", w.Text, w.Readings)
	}
}

// TestASecondReadingOnlyArrivesWithALegacySpelling is the property.
//
// A reading is stated only where the document holds one of the sixteen
// spellings YAML 1.1 reads as a boolean and 1.2 does not, written plain.
func TestASecondReadingOnlyArrivesWithALegacySpelling(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		v := yamlgen.Values().Draw(rt, "value")
		st := yamlgen.Styles().Draw(rt, "style")

		w := yamlgen.Write(v, st)
		if len(w.Readings) == 0 {
			return
		}

		if !holdsLegacySpelling(v) {
			rt.Fatalf("a second reading with no legacy spelling in the value\n%q\n%v", w.Text, w.Readings)
		}

		if st.Quoting != yamlgen.QuotePlain {
			rt.Fatalf("a second reading under %s, which quotes every string\n%q", st, w.Text)
		}
	})
}

func holdsLegacySpelling(v yamlgen.Value) bool {
	words := map[string]bool{
		"y": true, "Y": true, "yes": true, "Yes": true, "YES": true,
		"on": true, "On": true, "ON": true,
		"n": true, "N": true, "no": true, "No": true, "NO": true,
		"off": true, "Off": true, "OFF": true,
	}

	switch n := v.(type) {
	case yamlgen.Str:
		return words[n.V]
	case yamlgen.Seq:
		return slices.ContainsFunc(n.Items, holdsLegacySpelling)
	case yamlgen.Map:
		for _, p := range n.Pairs {
			// Keys as well as values: a key is a node now, so a plain "yes:"
			// is the key "true" under YAML 1.1.
			if holdsLegacySpelling(p.Key) || holdsLegacySpelling(p.Val) {
				return true
			}
		}
	case yamlgen.Anchored:
		return holdsLegacySpelling(n.V)
	case yamlgen.Tagged:
		return holdsLegacySpelling(n.V)
	case yamlgen.Alias:
		return holdsLegacySpelling(n.V)
	}

	return false
}
