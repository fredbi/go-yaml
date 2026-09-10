// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"reflect"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"
)

// TestNormalizeKeepsBothKeysThatSpellAlike holds the comparator to counting
// what it was given.
//
// yamlgen.Normalize flattens a map to map[string]any so that a map[any]any and
// the `any` path's map[string]any compare, naming each key the way the `any`
// path names one. uint64(1) and "1" are two entries and one name -- which is
// what the document "1: x" over "\"1\": y" reads into a map[any]any -- and
// writing one over the other dropped an entry and let Go's map order pick
// which. Both are kept now, under one Collision.
func TestNormalizeKeepsBothKeysThatSpellAlike(t *testing.T) {
	m := map[any]any{uint64(1): "x", "1": "y"}

	got := yamlgen.Normalize(reflect.ValueOf(m))

	require.Len(t, got, 1, "one name, and both entries under it")
	assert.Equal(t, map[string]any{
		"1": yamlgen.Collision{Name: "1", Values: []any{"x", "y"}},
	}, got)
}

// TestNormalizeIsTheSameOnEveryWalk is the property the Collision exists for:
// the same map normalizes to the same value however Go walks it.
//
// Before, this failed within a handful of runs, and the test it feeds failed
// with it -- TestTheEnumeratedShapesReadIntoAGoType agreed on three runs in
// five over the one enumerated document that reaches this.
func TestNormalizeIsTheSameOnEveryWalk(t *testing.T) {
	m := map[any]any{uint64(1): "x", "1": "y", true: "t", "true": "u"}

	first := yamlgen.Normalize(reflect.ValueOf(m))
	for range 200 {
		require.Equal(t, first, yamlgen.Normalize(reflect.ValueOf(m)))
	}
}

// TestNormalizeLeavesAMapWithoutACollisionAlone: the ordinary map keeps its
// values where they were, so nothing outside the collision changes shape.
func TestNormalizeLeavesAMapWithoutACollisionAlone(t *testing.T) {
	m := map[any]any{uint64(1): "x", "two": "y", true: "z"}

	assert.Equal(t, map[string]any{"1": "x", "two": "y", "true": "z"},
		yamlgen.Normalize(reflect.ValueOf(m)))
}
