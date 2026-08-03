// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
	"pgregory.net/rapid"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"
)

// TestPresentationInvariance is the property this package exists for: the same
// value, written down several different ways, has to read back the same every
// time.
//
// It needs no grammar, no oracle and no reference implementation. The invariant
// is internal, so every failure is unambiguous -- either two presentations of
// one value disagree, or one of them does not read back at all.
func TestPresentationInvariance(t *testing.T) {
	tally := newTally()

	rapid.Check(t, func(rt *rapid.T) {
		value := yamlgen.Values().Draw(rt, "value")
		styles := yamlgen.DistinctStyles(rt, 4)
		expected := value.Decoded()

		for _, style := range styles {
			src := yamlgen.Emit(value, style)
			known := yamlgen.Known(yamlgen.Decode, value, style)

			var got any
			err := yaml.Unmarshal([]byte(src), &got)
			diverged := err != nil || !assert.ObjectsAreEqual(expected, got)

			if known != nil {
				tally.record(known.Name, diverged)

				continue
			}

			if err != nil {
				rt.Fatalf("style %s did not read back:\n%s\n---\nerror: %v", style, src, err)
			}
			if diverged {
				rt.Fatalf("style %s read back differently:\n%s\n---\nexpected: %#v\ngot:      %#v",
					style, src, expected, got)
			}
		}
	})

	tally.report(t, yamlgen.Decode)
}

// TestEmitParses is the weaker half of the same idea, kept separate because it
// fails for a different reason: whatever the value, every style has to produce
// something the parser will read.
//
// A failure here is a document the emitter believes is valid YAML and the
// library does not, which is either an emitter defect or a parser defect. The
// grammar oracle is what will tell the two apart; until then it is worth
// knowing that the disagreement exists.
func TestEmitParses(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		value := yamlgen.Values().Draw(rt, "value")
		style := yamlgen.Styles().Draw(rt, "style")

		src := yamlgen.Emit(value, style)

		var got any
		if err := yaml.Unmarshal([]byte(src), &got); err != nil {
			rt.Fatalf("style %s produced a document that does not parse:\n%s\n---\nerror: %v",
				style, src, err)
		}
	})
}

// TestEmitterAgreesOnKnownDocuments guards the emitter itself.
//
// The property tests above are only worth what the emitter is worth: if it
// wrote a value down wrongly, the failure would be blamed on the library. These
// are the cases where the expected text is obvious enough to write out by hand.
func TestEmitterAgreesOnKnownDocuments(t *testing.T) {
	block := yamlgen.Style{Indent: 2, Quoting: yamlgen.QuotePlain, NullSpelling: "null"}
	flow := yamlgen.Style{Flow: true, Indent: 2, Quoting: yamlgen.QuotePlain, NullSpelling: "null"}
	literal := yamlgen.Style{Indent: 2, Quoting: yamlgen.QuotePlain, Literal: true, NullSpelling: "null"}

	tests := []struct {
		name  string
		value yamlgen.Value
		style yamlgen.Style
		want  string
	}{
		{
			name:  "block mapping",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: "a", Val: yamlgen.Int{V: 1}}}},
			style: block,
			want:  "a: 1\n",
		},
		{
			name:  "nested block mapping",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: "a", Val: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: "b", Val: yamlgen.Int{V: 2}}}}}}},
			style: block,
			want:  "a:\n  b: 2\n",
		},
		{
			name:  "block sequence",
			value: yamlgen.Seq{Items: []yamlgen.Value{yamlgen.Int{V: 1}, yamlgen.Int{V: 2}}},
			style: block,
			want:  "- 1\n- 2\n",
		},
		{
			name:  "empty collections have no block spelling",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: "a", Val: yamlgen.Seq{}}}},
			style: block,
			want:  "a: []\n",
		},
		{
			name:  "flow mapping",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: "a", Val: yamlgen.Seq{Items: []yamlgen.Value{yamlgen.Int{V: 1}}}}}},
			style: flow,
			want:  "{a: [1]}\n",
		},
		{
			name:  "literal block scalar clips one trailing newline",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: "a", Val: yamlgen.Str{V: "one\ntwo\n"}}}},
			style: literal,
			want:  "a: |\n  one\n  two\n",
		},
		{
			name:  "literal block scalar strips when there is none",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: "a", Val: yamlgen.Str{V: "one\ntwo"}}}},
			style: literal,
			want:  "a: |-\n  one\n  two\n",
		},
		{
			name:  "literal block scalar keeps the extra ones",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: "a", Val: yamlgen.Str{V: "one\n\n"}}}},
			style: literal,
			want:  "a: |+\n  one\n\n",
		},
	}

	tests = append(tests, commentCases()...)
	tests = append(tests, anchorCases()...)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, yamlgen.Emit(tc.value, tc.style))

			var got any
			require.NoError(t, yaml.Unmarshal([]byte(tc.want), &got))
			assert.Equal(t, tc.value.Decoded(), got)
		})
	}
}

// commentCases pins where comments are written. Comments carry no meaning, so
// nothing else in the suite would notice if they landed somewhere legal but
// unintended -- or somewhere illegal, which would then be blamed on the parser.
func commentCases() []struct {
	name  string
	value yamlgen.Value
	style yamlgen.Style
	want  string
} {
	base := yamlgen.Style{Indent: 2, Quoting: yamlgen.QuotePlain, NullSpelling: "null"}
	head := base
	head.Comments = yamlgen.HeadComments
	line := base
	line.Comments = yamlgen.LineComments
	both := base
	both.Comments = yamlgen.AllComments

	pair := func(k string, v yamlgen.Value) yamlgen.Value {
		return yamlgen.Map{Pairs: []yamlgen.Pair{{Key: k, Val: v}}}
	}

	return []struct {
		name  string
		value yamlgen.Value
		style yamlgen.Style
		want  string
	}{
		{
			name:  "a head comment sits above its entry",
			value: pair("a", yamlgen.Int{V: 1}),
			style: head,
			want:  "# c1\na: 1\n",
		},
		{
			name:  "a line comment sits after the value",
			value: pair("a", yamlgen.Int{V: 1}),
			style: line,
			want:  "a: 1 # c1\n",
		},
		{
			name:  "both, numbered in the order they are written",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: "a", Val: yamlgen.Int{V: 1}}, {Key: "b", Val: yamlgen.Int{V: 2}}}},
			style: both,
			want:  "# c1\na: 1 # c2\n# c3\nb: 2 # c4\n",
		},
		{
			name:  "a comment introduces a nested block",
			value: pair("a", pair("b", yamlgen.Int{V: 2})),
			style: line,
			want:  "a: # c1\n  b: 2 # c2\n",
		},
		{
			name:  "head comments indent with their entry",
			value: pair("a", pair("b", yamlgen.Int{V: 2})),
			style: head,
			want:  "# c1\na:\n  # c2\n  b: 2\n",
		},
		{
			name:  "sequence entries take comments too",
			value: yamlgen.Seq{Items: []yamlgen.Value{yamlgen.Int{V: 1}}},
			style: both,
			want:  "# c1\n- 1 # c2\n",
		},
	}
}

// anchorCases pins where an anchor is written and what an alias looks like.
//
// This is the first axis that changes the value rather than its presentation,
// so a mistake here would not merely write a document oddly -- it would write a
// different document and blame the library for reading it as one. Each case
// also asserts that the text decodes to the value, which is what catches an
// anchor written somewhere the parser attaches to the wrong node.
func anchorCases() []struct {
	name  string
	value yamlgen.Value
	style yamlgen.Style
	want  string
} {
	block := yamlgen.Style{Indent: 2, Quoting: yamlgen.QuotePlain, NullSpelling: "null"}
	flow := yamlgen.Style{Flow: true, Indent: 2, Quoting: yamlgen.QuotePlain, NullSpelling: "null"}
	literal := yamlgen.Style{Indent: 2, Quoting: yamlgen.QuotePlain, Literal: true, NullSpelling: "null"}

	one := yamlgen.Anchored{Name: "a1", V: yamlgen.Int{V: 1}}
	seq := yamlgen.Anchored{Name: "a1", V: yamlgen.Seq{Items: []yamlgen.Value{yamlgen.Int{V: 1}}}}

	return []struct {
		name  string
		value yamlgen.Value
		style yamlgen.Style
		want  string
	}{
		{
			name:  "an anchored scalar keeps the anchor on its line",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: "k", Val: one}}},
			style: block,
			want:  "k: &a1 1\n",
		},
		{
			name: "an alias refers back to it",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{
				{Key: "a", Val: one},
				{Key: "b", Val: yamlgen.Alias{Name: "a1", V: yamlgen.Int{V: 1}}},
			}},
			style: block,
			want:  "a: &a1 1\nb: *a1\n",
		},
		{
			name:  "an anchored block collection takes the line above it",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: "k", Val: seq}}},
			style: block,
			want:  "k: &a1\n  - 1\n",
		},
		{
			name:  "an anchor in flow style sits inside the brackets",
			value: yamlgen.Seq{Items: []yamlgen.Value{one}},
			style: flow,
			want:  "[&a1 1]\n",
		},
		{
			name:  "an anchored block scalar keeps its header on the line",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: "k", Val: yamlgen.Anchored{Name: "a1", V: yamlgen.Str{V: "x\n"}}}}},
			style: literal,
			want:  "k: &a1 |\n  x\n",
		},
		{
			name:  "an anchored empty node is the anchor alone",
			value: yamlgen.Seq{Items: []yamlgen.Value{yamlgen.Anchored{Name: "a1", V: yamlgen.Null{}}}},
			style: yamlgen.Style{Indent: 2, Quoting: yamlgen.QuotePlain, NullSpelling: ""},
			want:  "- &a1\n",
		},
		{
			name:  "an anchor at the root of a block collection",
			value: seq,
			style: block,
			want:  "&a1\n- 1\n",
		},
		{
			name: "a sequence entry can be an alias",
			value: yamlgen.Seq{Items: []yamlgen.Value{
				seq,
				yamlgen.Alias{Name: "a1", V: yamlgen.Seq{Items: []yamlgen.Value{yamlgen.Int{V: 1}}}},
			}},
			style: block,
			want:  "- &a1\n  - 1\n- *a1\n",
		},
	}
}
