// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package lab_test

import (
	"bytes"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	yaml "github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
	"github.com/go-openapi/go-yaml/internal/lab"
)

// TestToJSONWalkMatchesCodec checks a converter written on the walk gets the
// same document as the shipped one.
//
// It holds nothing and asks the arena to keep nothing: what collection it is
// in, how deep, which entry of that, and where the token stands all arrive with
// each step. Getting the same JSON out is what says the walk hands everything
// over, in order, once.
func TestToJSONWalkMatchesCodec(t *testing.T) {
	all, err := workloads.All()
	require.NoError(t, err)

	for _, w := range all {
		t.Run(w.Name, func(t *testing.T) {
			want, err := yaml.ToJSON(w.Data)
			require.NoError(t, err)

			got, err := lab.ToJSONWalk(w.Data)
			require.NoError(t, err)
			require.True(t, json.Valid(got), "the walk wrote invalid JSON")

			assert.Equal(t, readJSON(t, want), readJSON(t, got))
		})
	}
}

// TestToJSONWalkOnSmallDocuments covers the shapes the workloads do not.
func TestToJSONWalkOnSmallDocuments(t *testing.T) {
	for _, src := range []string{
		"a: 1", "a: 1\nb: 2\n", "- 1\n- 2\n", "a:\n  b: 1\n  c: [1, 2]\n",
		"a: {}\nb: []\n", "top:\n  - x: 1\n    y: 2\n  - z: 3\n",
		// Flow mappings. Their keys reach the visitor from parseMapKeyValue and
		// the flow map's own TokenGroupMapKey branch, not from parseMapEntry,
		// and none of the six workloads writes one.
		"a: {p: 1}\n", "a: {p: 1, q: 2}\n", "a: {p: {q: [1, {r: 2}]}}\n",
		"- {p: 1}\n- {q: 2}\n", "a: {p: , q: 2}\n",
		// A flow key with no value at all, and one that is anchored.
		"a: {p}\n", "a: {p, q}\n", "a: {&n x}\n",
		"a:\n", "a: 1.5\nb: true\nc: ~\n", "e: é wide 日本\n",
		"a: 'quote \" and \\ backslash'\n", "- [1, [2, [3]]]\n",
	} {
		t.Run(src, func(t *testing.T) {
			want, err := yaml.ToJSON([]byte(src))
			require.NoError(t, err)

			got, err := lab.ToJSONWalk([]byte(src))
			require.NoError(t, err)

			assert.Equal(t, readJSON(t, want), readJSON(t, got), "shipped %q, walk %q", want, got)
		})
	}
}

// TestToJSONWalkOnAnchors checks an anchor and the aliases naming it.
//
// The anchor goes over before the node it names and closes after it, so the
// converter records what that node wrote and writes it again for each alias.
// Nesting matters: "&o {p: &i 1}" has to leave both names readable.
func TestToJSONWalkOnAnchors(t *testing.T) {
	for _, src := range []string{
		"a: &x 1\nb: *x\n",
		"a: &x hello\nb: *x\nc: *x\n",
		"a: &x {p: 1, q: [2, 3]}\nb: *x\n",
		"a: &x [1, 2]\nb: *x\n",
		"- &x hello\n- *x\n",
		"a: &o {p: &i 1}\nb: *i\nc: *o\n",
		"a: &x null\nb: *x\n",
		"a: &x\nb: *x\n",
		"top:\n  - &e {k: v}\n  - *e\n",
	} {
		t.Run(src, func(t *testing.T) {
			want, err := yaml.ToJSON([]byte(src))
			require.NoError(t, err)

			got, err := lab.ToJSONWalk([]byte(src))
			require.NoError(t, err)

			assert.Equal(t, readJSON(t, want), readJSON(t, got), "shipped %q, walk %q", want, got)
		})
	}
}

// TestToJSONWalkOnAnchorStress runs the anchor documents of the stress corpus,
// which are what Save was built for: anchors_many holds 218 chunks in the stash
// at once, anchors_nested 90.
func TestToJSONWalkOnAnchorStress(t *testing.T) {
	stress, err := workloads.Stress()
	require.NoError(t, err)

	var ran int
	for _, w := range stress {
		if !strings.HasPrefix(w.Name, "anchors_") {
			continue
		}
		ran++
		t.Run(w.Name, func(t *testing.T) {
			want, err := yaml.ToJSON(w.Data)
			require.NoError(t, err)

			got, err := lab.ToJSONWalk(w.Data)
			require.NoError(t, err)

			var wantValue, gotValue any
			require.NoError(t, json.Unmarshal(want, &wantValue))
			require.NoError(t, json.Unmarshal(got, &gotValue), "the walk wrote invalid JSON")

			assert.Equal(t, wantValue, gotValue)
		})
	}
	require.Positive(t, ran, "the stress corpus holds no anchor document")
}

// TestToJSONWalkRefusesWhatItCannotWrite names the shapes left out, so that
// dropping one of them later is a deliberate change and not a surprise.
func TestToJSONWalkRefusesWhatItCannotWrite(t *testing.T) {
	for name, src := range map[string]string{
		"merge key":      "a: &x {p: 1}\nb:\n  <<: *x\n  q: 2\n",
		"alias as a key": "a: &x k\n*x: 1\n",
		"tag":            "a: !!str 1\n",
		"unknown alias":  "a: *nowhere\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := lab.ToJSONWalk([]byte(src))
			require.Error(t, err, "the walk wrote something for %s", name)
		})
	}
}

// readJSON reads JSON into values, with every number held to the quantity it
// names rather than to the digits it was written with.
//
// Two things make the plain reader wrong here. It reads a number into a
// float64, which refuses anything past 1e308, and a converter that keeps a
// document's numbers as numbers writes those -- twitter_status holds 3E4415.
// And the converters spell a wide number differently: the shipped one writes
// the digits the document wrote, and these write what big.Float renders, so
// "3E4415" and "3e+4415" are the same number and not the same text.
func readJSON(t *testing.T, text []byte) any {
	t.Helper()

	dec := json.NewDecoder(bytes.NewReader(text))
	dec.UseNumber()

	var v any
	require.NoError(t, dec.Decode(&v), "not JSON: %s", text)

	return byQuantity(v)
}

// byQuantity replaces every number with the quantity it names, exactly. A
// big.Rat reads integers, decimals and exponents without rounding any of them,
// which a big.Float at any fixed precision would.
func byQuantity(v any) any {
	switch t := v.(type) {
	case json.Number:
		if r, ok := new(big.Rat).SetString(t.String()); ok {
			return "number " + r.RatString()
		}

		return "number " + t.String()
	case map[string]any:
		for k, e := range t {
			t[k] = byQuantity(e)
		}

		return t
	case []any:
		for i, e := range t {
			t[i] = byQuantity(e)
		}

		return t
	default:
		return v
	}
}
