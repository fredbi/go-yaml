// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
	"pgregory.net/rapid"

	yaml "github.com/go-openapi/go-yaml"
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
// A reading is stated for two reasons and no third. Either the document holds
// one of the sixteen spellings YAML 1.1 reads as a boolean and 1.2 does not,
// written plain; or it writes a number in a form 1.1 does not read, which
// Style.NumberForm decides.
func TestASecondReadingOnlyArrivesWithALegacySpelling(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		v := yamlgen.Values().Draw(rt, "value")
		st := yamlgen.Styles().Draw(rt, "style")

		w := yamlgen.Write(v, st)
		if len(w.Readings) == 0 {
			return
		}

		// A number's form is the style's, so the value alone cannot say
		// whether one was written -- only that there is a number to write.
		if divergentForm(st, v) && holdsNumber(v) {
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

// divergentForm reports the number forms YAML 1.1 may not read.
//
// "0o37" is a string there, and so is a float whose exponent carries no sign.
// The decimal and "+" forms it reads exactly as core does, and hex too.
//
// A BigFloat diverges under every form: its text comes from
// big.Float.Text('g', -1), which writes an exponent with no '.' before it.
func divergentForm(st yamlgen.Style, v yamlgen.Value) bool {
	if st.NumberForm == yamlgen.NumberOctal || st.NumberForm == yamlgen.NumberExponent {
		return true
	}

	return holdsBigFloat(v)
}

// holdsBigFloat reports whether v has a float past what a float64 holds in it.
func holdsBigFloat(v yamlgen.Value) bool {
	switch n := v.(type) {
	case yamlgen.BigFloat:
		return true
	case yamlgen.Seq:
		return slices.ContainsFunc(n.Items, holdsBigFloat)
	case yamlgen.Map:
		for _, p := range n.Pairs {
			if holdsBigFloat(p.Key) || holdsBigFloat(p.Val) {
				return true
			}
		}
	case yamlgen.Anchored:
		return holdsBigFloat(n.V)
	case yamlgen.Alias:
		return holdsBigFloat(n.V)
	case yamlgen.Tagged:
		return holdsBigFloat(n.V)
	}

	return false
}

// holdsNumber reports whether v has a number in it anywhere.
func holdsNumber(v yamlgen.Value) bool {
	switch n := v.(type) {
	case yamlgen.Int, yamlgen.Float, yamlgen.BigInt, yamlgen.BigFloat:
		return true
	case yamlgen.Seq:
		return slices.ContainsFunc(n.Items, holdsNumber)
	case yamlgen.Map:
		for _, p := range n.Pairs {
			if holdsNumber(p.Key) || holdsNumber(p.Val) {
				return true
			}
		}
	case yamlgen.Anchored:
		return holdsNumber(n.V)
	case yamlgen.Alias:
		return holdsNumber(n.V)
	case yamlgen.Tagged:
		return holdsNumber(n.V)
	}

	return false
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

// TestTheNumberFormsMeanUnder11WhatTheLibraryReads holds yamlgen's table of
// YAML 1.1 answers to the library's own 1.1 reader.
//
// The table in reading.go is written out rather than derived, so it can be
// wrong, and being wrong there would mean stating a meaning no implementation
// holds -- the one failure the whole readings mechanism exists to avoid. This
// asks the library the same question for every form the emitter can write.
//
// The library is not the specification, so a disagreement is a question rather
// than a verdict. It has been the right question twice: libfyaml overturned a
// triage done without it, and the reference parser settled the key departure.
func TestTheNumberFormsMeanUnder11WhatTheLibraryReads(t *testing.T) {
	values := []yamlgen.Value{
		yamlgen.Int{V: 0}, yamlgen.Int{V: 1}, yamlgen.Int{V: 31}, yamlgen.Int{V: 511},
		yamlgen.Int{V: 1000}, yamlgen.Int{V: -31},
		yamlgen.Float{V: 1.5}, yamlgen.Float{V: -1.5}, yamlgen.Float{V: 0.5},
		yamlgen.Float{V: 1000}, yamlgen.Float{V: 1e-320}, yamlgen.Float{V: 0},
	}

	forms := []yamlgen.NumberForm{
		yamlgen.NumberPlain, yamlgen.NumberSigned,
		yamlgen.NumberHex, yamlgen.NumberOctal, yamlgen.NumberExponent,
	}

	for _, form := range forms {
		st := yamlgen.Style{NullSpelling: "null", NumberForm: form}

		for _, v := range values {
			w := yamlgen.Write(yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "k"}, Val: v}}}, st)

			// The core answer first, so a form that changes the value at all
			// fails here rather than quietly in the 1.1 column.
			var core any
			require.NoError(t, yaml.Unmarshal([]byte(w.Text), &core), "%q", w.Text)
			assert.Equal(t, map[string]any{"k": v.Decoded()}, core,
				"%q does not read back as the value it was written from", w.Text)

			var legacy any
			require.NoError(t, yaml.Unmarshal([]byte("%YAML 1.1\n---\n"+w.Text), &legacy), "%q", w.Text)

			want := map[string]any{"k": v.Decoded()}
			if alt, differs := w.Readings[yamlgen.Reading11]; differs {
				want = alt.(map[string]any)
			}

			assert.Equal(t, want, legacy,
				"%q: yamlgen says %v under 1.1 and the library reads %v", w.Text, want, legacy)
		}
	}
}
