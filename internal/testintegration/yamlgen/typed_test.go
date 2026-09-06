// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"reflect"
	"testing"

	"pgregory.net/rapid"

	yaml "github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"
)

// TestDecodingIntoAGoTypeGivesTheSameValue reads each document twice, once into
// an `any` and once into a Go type built from the value it was written from.
//
// The destination is the only variable. Reading into an `any` walks the token
// stream; reading into a Go type gathers a tree and fills fields by reflection,
// and since the two stopped sharing code they can disagree with nothing to say
// so. Every other property here decodes into an `any`, and so does every
// corpus-driven decode in yamlcorpus and in conformance/ -- three defects were
// found living on the reflection path in one afternoon of reading it, none of
// them by any harness.
//
// The comparison is against the `any` answer rather than against
// Value.Decoded, on purpose: a defect that hits both paths cancels out and does
// not have to be excused twice. What is left is the destination.
func TestDecodingIntoAGoTypeGivesTheSameValue(t *testing.T) {
	tally := newTally()

	var structs, reached int

	rapid.Check(t, func(rt *rapid.T) {
		value := yamlgen.Values().Draw(rt, "value")
		style := yamlgen.Styles().Draw(rt, "style")
		src := yamlgen.Emit(value, style)

		// The `any` path is the yardstick, so a document it will not read has
		// nothing to say here. TestPresentationInvariance holds that half.
		var loose any
		if yaml.Unmarshal([]byte(src), &loose) != nil {
			return
		}

		target := yamlgen.TargetFor(value)
		if target.Structs == 0 {
			// No mapping in the document could be named by struct tags, so
			// nothing reached decodeStruct and the run says nothing.
			return
		}
		structs++

		into := reflect.New(target.Type)
		err := yaml.Unmarshal([]byte(src), into.Interface())

		want := yamlgen.Normalize(reflect.ValueOf(loose))
		diverged := err != nil || !sameThroughAType(want, yamlgen.Normalize(into.Elem()))

		if known := yamlgen.Known(yamlgen.DecodeTyped, value, style); known != nil {
			tally.record(known.Name, diverged)

			return
		}

		reached++

		if err != nil {
			rt.Fatalf("%s read into an `any` and not into %v:\n%s\n---\nerror: %v",
				style, target.Type, src, err)
		}
		if diverged {
			rt.Fatalf("%s read differently into %v:\n%s\n---\nas an any: %#v\nas a type: %#v",
				style, target.Type, src, want, yamlgen.Normalize(into.Elem()))
		}
	})

	t.Logf("%d documents built a struct, %d of them compared", structs, reached)
	tally.report(t, yamlgen.DecodeTyped)
}

// sameThroughAType is [sameValue] with numbers compared across the Go types a
// destination chooses.
//
// An int64 field holds 1 where an `any` holds uint64(1), and a float64 field
// holds 1 where an `any` holds the same uint64. That is the destination doing
// its job rather than the two paths disagreeing, so the comparison is by value.
// Everything else -- a NaN, a big.Float, a string -- goes to sameValue.
func sameThroughAType(want, got any) bool {
	if w, isNumber := asFloat(want); isNumber {
		g, ok := asFloat(got)

		return ok && (w == g || (w != w && g != g))
	}

	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok || len(w) != len(g) {
			return false
		}

		for k, v := range w {
			other, found := g[k]
			if !found || !sameThroughAType(v, other) {
				return false
			}
		}

		return true
	case []any:
		g, ok := got.([]any)
		if !ok || len(w) != len(g) {
			return false
		}

		for i := range w {
			if !sameThroughAType(w[i], g[i]) {
				return false
			}
		}

		return true
	default:
		return sameValue(want, got)
	}
}

// asFloat reports the Go number kinds a destination can turn an integer into.
//
// The big types are left out: they are not a destination this package builds,
// and comparing one against a float64 would lose the precision that makes them
// worth having.
func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}
