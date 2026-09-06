// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus_test

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	yaml "github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/testintegration/stance"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlcorpus"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"
)

// TestTheEnumeratedShapesReadIntoAGoType runs every enumerated document through
// the reflection path as well as through the `any` path.
//
// These are the documents the generator cannot write, and they are the ones the
// reflection path most needs. Two of the three defects found on it on
// 2026-09-06 were merge keys: `use: {<<: *base, b: 3}` refused its own key as a
// duplicate of the one it overrides, and `<<: [*one, *two]` was refused
// outright. Both were correct into an `any` and correct in
// go.yaml.in/yaml/v3, and wrong only into a struct -- so no corpus of verdicts
// and no comparison against another library would ever have found them.
//
// The `any` read is the yardstick: a document it refuses says nothing here, and
// yamlcorpus.GoYAML is what holds that half. The destination is built from what
// it read, so the type fits by construction and a disagreement is the
// destination's.
func TestTheEnumeratedShapesReadIntoAGoType(t *testing.T) {
	var structs, compared int

	for _, group := range [][]stance.Shape{
		yamlcorpus.KeyShapes(), yamlcorpus.TagShapes(), yamlcorpus.SchemaShapes(),
		yamlcorpus.MergeShapes(), yamlcorpus.DirectiveShapes(),
	} {
		for _, s := range group {
			var loose any
			if yaml.Unmarshal(s.Src, &loose) != nil {
				continue
			}

			target := yamlgen.TargetForDecoded(loose)
			if target.Structs == 0 {
				continue
			}
			structs++

			into := reflect.New(target.Type)
			err := yaml.Unmarshal(s.Src, into.Interface())

			want := yamlgen.Normalize(reflect.ValueOf(loose))
			got := yamlgen.Normalize(into.Elem())
			failed := err != nil || !sameNumerically(want, got)

			why, known := typedPathDefects[s.Name]

			switch {
			case failed && known:
				t.Logf("still fails -- %s: %s", s.Name, why)
			case failed:
				t.Errorf("%s: %q reads into an `any` and differently into %v\n"+
					"  as an any: %#v\n  as a type: %#v\n  error:     %v",
					s.Name, s.Src, target.Type, want, got, err)
			case known:
				t.Errorf("no longer fails, so its entry is stale: %s -- %s", s.Name, why)
			default:
				compared++
			}
		}
	}

	t.Logf("%d enumerated documents built a struct, %d of them read the same both ways", structs, compared)

	// A floor rather than a count, since a shape added to any family may or may
	// not be a mapping. It is here so that a change which stops the enumerated
	// documents reaching the reflection path at all shows up as a failure
	// rather than as a quieter run.
	require.GreaterOrEqual(t, structs, 20,
		"the enumerated shapes have stopped reaching the reflection path")
}

// The enumerated documents the reflection path gets wrong, and why.
//
// Every one is a defect already recorded elsewhere; this test found nothing the
// registers did not hold, which is the answer it was built to give. An entry
// leaves by being fixed -- the test says so rather than passing quietly.
//
// Eleven left that way on 2026-09-07, when the decoder branch landed. Eight
// were the struct zeroing, closed in two steps: 7dc4075 inverted decodeStruct,
// so a key no field can be named after is a key no field claims rather than one
// that abandons the whole mapping, and entryName then named a key by the
// type's own canonical spelling, so "true: x" reaches a field tagged "true" and
// "1.0: x" one tagged "1.0" -- which is how the same document reads into a
// map[string]any. Three were the merge path, closed by 6c10f40: a mapping's own
// key was refused as a duplicate of the one it overrides, and a merge given a
// sequence was refused with "sequence was used where mapping is expected".
//
// This test also found one the registers did not hold, which is what it was
// built for: the walking decoder read a merge written in place -- "<<: {a: 1}"
// rather than "<<: *b" -- as a key no field claims and dropped it. A merge
// written as an alias gave up on the alias and fell back to the tree; one
// written in place had nothing else to give up on. Fixed in the same branch.
var typedPathDefects = map[string]string{
	// A tag on a key is not unwrapped before the key is named, so the key
	// resolves to nothing a field can be named after. The same root as
	// yamlgen.Ledger's parser entries for a tag over a key, and parked with
	// them: see stream 2, defects 2, 13 and 14.
	"a key tagged !!float": "a tag on a key, parked",
}

// sameNumerically compares two decodes of one document, with numbers compared
// across the Go types the destination chooses: an int64 field holds 1 where an
// `any` holds uint64(1).
func sameNumerically(want, got any) bool {
	if w, isNumber := asFloat(want); isNumber {
		g, ok := asFloat(got)

		return ok && (w == g || (w != w && g != g))
	}

	switch w := want.(type) {
	case *big.Int:
		g, ok := got.(*big.Int)

		return ok && w.Cmp(g) == 0
	case *big.Float:
		g, ok := got.(*big.Float)

		return ok && w.Cmp(g) == 0
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok || len(w) != len(g) {
			return false
		}

		for k, v := range w {
			other, found := g[k]
			if !found || !sameNumerically(v, other) {
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
			if !sameNumerically(w[i], g[i]) {
				return false
			}
		}

		return true
	default:
		return assert.ObjectsAreEqual(want, got)
	}
}

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
