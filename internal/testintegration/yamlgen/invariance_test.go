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

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, yamlgen.Emit(tc.value, tc.style))

			var got any
			require.NoError(t, yaml.Unmarshal([]byte(tc.want), &got))
			assert.Equal(t, tc.value.Decoded(), got)
		})
	}
}
