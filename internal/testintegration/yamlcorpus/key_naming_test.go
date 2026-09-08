// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus_test

import (
	"bytes"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
)

// How a mapping key is named, and when two of them are one.
//
// The rule, implemented 2026-09-10: a key is named by the canonical spelling of
// its *type*, and two keys conflict when their type and their name agree.
//
//	an integer   expanded decimal      007 -> "7", 0x10 -> "16", +1 -> "1"
//	a float      shortest, with .0     1e3 -> "1000.0", 1.00 -> "1.0"
//	the specials YAML's own spelling   .Inf -> ".inf", .NaN -> ".nan"
//	null         "null"                ~ and an empty key alike
//	a boolean    "true" / "false"      True -> "true"
//
// Naming per type is what makes uniqueness per type expressible: a float always
// carries its ".0", so it never lands in the integers' namespace and 1 beside
// 1.0 is two keys rather than a collision.

func read(t *testing.T, src string) any {
	t.Helper()

	var v any
	require.NoError(t, codec.NewDecoder(bytes.NewReader([]byte(src))).Decode(&v), "%q", src)

	return v
}

func refuses(t *testing.T, src string) string {
	t.Helper()

	var v any
	err := codec.NewDecoder(bytes.NewReader([]byte(src))).Decode(&v)
	require.Error(t, err, "%q should be refused", src)

	return err.Error()
}

// TestFixedAKeyIsNamedByItsType is the naming half.
//
// Every one of these was named by the source text, or by Go's %v on the
// resolved value, before 2026-09-10. 1e3 keyed "1000" and 1.0 keyed "1", so a
// float lost the fact that it was one.
func TestFixedAKeyIsNamedByItsType(t *testing.T) {
	for _, tc := range []struct{ src, key string }{
		{src: "1.0: a\n", key: "1.0"},
		{src: "1e3: a\n", key: "1000.0"},
		{src: "1.5e3: a\n", key: "1500.0"},
		{src: "-0.0: a\n", key: "-0.0"},
		{src: "0.5: a\n", key: "0.5"},

		{src: "1: a\n", key: "1"},
		{src: "007: a\n", key: "7"},
		{src: "0x10: a\n", key: "16"},
		{src: "+1: a\n", key: "1"},

		{src: "~: a\n", key: "null"},
		{src: ": a\n", key: "null"},
		{src: "NULL: a\n", key: "null"},
		{src: "True: a\n", key: "true"},
		{src: "FALSE: a\n", key: "false"},

		{src: ".Inf: a\n", key: ".inf"},
		{src: ".INF: a\n", key: ".inf"},
		{src: "-.inf: a\n", key: "-.inf"},
		{src: ".NaN: a\n", key: ".nan"},
		{src: ".NAN: a\n", key: ".nan"},
	} {
		assert.Contains(t, read(t, tc.src), tc.key, "%q should be keyed %q", tc.src, tc.key)
	}
}

// TestFixedTwoKeysOfOneTypeAndName Conflict is the uniqueness half.
//
// Every one of these was read without complaint before 2026-09-10, silently
// keeping the last value: two spellings of one key, and a value gone.
func TestFixedTwoKeysOfOneTypeAndNameConflict(t *testing.T) {
	for _, src := range []string{
		"007: a\n7: b\n",
		"0x10: a\n16: b\n",
		"+1: a\n1: b\n",
		"1.0: a\n1.00: b\n",
		"~: a\nnull: b\n",
		"true: a\nTrue: b\n",
		".inf: a\n.Inf: b\n",
		".nan: a\n.NaN: b\n",
	} {
		assert.Contains(t, refuses(t, src), "already defined", "%q", src)
	}
}

// TestFixedTwoTypesAreTwoKeys is what the naming rule buys.
//
// A float and an integer of the same value are two nodes and stay two keys,
// which is what go.yaml.in/yaml/v3 does and what 3.2.1.1 requires. libfyaml
// merges them, and is lax here.
func TestFixedTwoTypesAreTwoKeys(t *testing.T) {
	got := read(t, "1.0: a\n1: b\n")

	assert.Equal(t, map[string]any{"1.0": "a", "1": "b"}, got,
		"a float and an integer of equal value are two keys")

	t.Run("and the infinities keep their sign", func(t *testing.T) {
		assert.Len(t, read(t, ".inf: a\n-.inf: b\n"), 2)
	})
}

// TestFixedAnExplicitFloatTagOnAKeyKeepsItsFloatness: writing the tag changes
// nothing about the name, which is what writing it should do.
//
// Found on 2026-09-10 by the two converters in this library disagreeing with
// each other: ToJSON wrote "226.0" for both spellings and the decoder "226" for
// the tagged one. They read one node→name walk now, [ast.KeyName], which
// resolves a tag rather than stepping over it -- so "!!float 226" is the float
// 226.0 and is named "226.0", where stripping the tag would have read the
// token "226" and named it after an integer.
func TestFixedAnExplicitFloatTagOnAKeyKeepsItsFloatness(t *testing.T) {
	assert.Contains(t, read(t, "226.0: x\n"), "226.0", "untagged, the float keeps its name")
	assert.Contains(t, read(t, "!!float 226.0: x\n"), "226.0",
		"the tag costs the key nothing")
	assert.Contains(t, read(t, "!!float 226: x\n"), "226.0",
		"and a whole number under !!float is named as the float it is")

	t.Run("the other tags name a key correctly", func(t *testing.T) {
		for _, tc := range []struct{ src, key string }{
			{src: "!!int 226: x\n", key: "226"},
			{src: "!!str 226.0: x\n", key: "226.0"},
			{src: "!!bool True: x\n", key: "true"},
			{src: "!!null ~: x\n", key: "null"},
		} {
			assert.Contains(t, read(t, tc.src), tc.key, "%q", tc.src)
		}
	})
}
