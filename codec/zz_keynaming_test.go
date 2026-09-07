// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
)

// Two keys that resolve to different nodes and name themselves alike are read
// five ways, and only one of them is right.
//
// §3.2.1.1 makes two keys equal when they resolve to the same node, so `1: x`
// over `"1": y` is an integer key and a string key -- two nodes, two keys, two
// entries.
//
// The rule is general over the resolution table rather than a handful of cases,
// which `0x1f: x` over `"31": y` is the clearest way to see: those two keys
// share not one character, and the walk merges them, because a key is named by
// the canonical spelling of what it resolved to and 0x1f resolves to 31. The
// empty key reaches it the same way -- it resolves to null, whose spelling is
// "null" -- so `: x` over `"null": y` merges too.
//
// A fix wants to be general over that table rather than to add cases, and the
// one pair that looks like an exception is the reason to say so.
//
// `.inf: x` over `"+.inf": y` keeps both today, and NOT because the naming is
// careful there. It is that `+.inf` does not resolve at all: token.go's
// reservedInfKeywords lists `.inf`, `.Inf`, `.INF`, `-.inf`, `-.Inf`, `-.INF`
// and omits the three `+` spellings, where the 1.2 core schema float production
// is `[-+]? ( \.inf | \.Inf | \.INF )`. So the quoted key and the plain key are
// both strings and there is nothing to merge. `.nan` is the control and is
// right: 1.2 admits no sign there, and we, libfyaml and go.yaml.in/yaml/v3 all
// read `+.nan` as a string.
//
// ⚠️ **That row will start merging when the resolver is fixed**, and it should
// not be read as a regression here when it does. It is a separate defect
// standing in front of this one -- both loaders read `+.inf` and `+.INF` as
// infinity, and codec.ToJSON writes `{"a":"+.inf"}` where it refuses `.inf`
// outright, so the same value is a number or a string depending on which of two
// legal spellings the document used.
//
// What each destination does with that:
//
//	map[any]any     both, typed: uint64(1) => "x" and "1" => "y"   ✅
//	map[string]any  refused, `duplicate key "1"`
//	an `any`        one entry, {"1": "y"} -- "x" is gone, silently
//	UseOrderedMap   both entries, both named "1"
//	ToJSON          {"1":"x","1":"y"}, a repeated member name
//
// The map[any]any row is what makes this the naming rather than the read: the
// library holds the two apart wherever the destination can, so nothing is lost
// until a key is named by the canonical spelling of its type and both land in
// the strings' namespace. yamlcorpus.Departures records the reading; this pins
// which destination does what, since the five disagree and a caller has no way
// to know which one they are on.
//
// RFC 8259 section 4 says the names in a JSON object SHOULD be unique, so the
// JSON was not invalid -- but every reader collapses it, and "x" is lost one
// step later.
//
// ✅ ToJSON refuses these documents since 2026-09-13, as ErrNotJSON rather than
// ErrDuplicateKey: they are two keys, and it is JSON that cannot hold both. The
// parser records the pair the way it records a repeated key, under
// WithJSONCompatible. The decoder's four rows are unchanged and are the defect
// this file is still for.

// TestDefectATypedKeyIsNamedIntoTheStringsNamespace pins all five.
func TestDefectATypedKeyIsNamedIntoTheStringsNamespace(t *testing.T) {
	for name, tc := range map[string]struct {
		src    string
		merged string
	}{
		"an integer and a string": {src: "1: x\n\"1\": y\n", merged: "1"},
		"a boolean and a string":  {src: "true: x\n\"true\": y\n", merged: "true"},
		"a null and a string":     {src: "~: x\n\"null\": y\n", merged: "null"},
		"a float and a string":    {src: "1.0: x\n\"1.0\": y\n", merged: "1.0"},
		// The one that shows what the rule is. "0x1f" and "31" share not one
		// character, and the walk merges them: a key is named by the canonical
		// spelling of what it resolved to, and 0x1f resolves to the integer 31.
		"a hexadecimal integer and the decimal string": {src: "0x1f: x\n\"31\": y\n", merged: "31"},
		// The empty key reaches it too, since it resolves to null and null's
		// canonical spelling is "null".
		"an empty key and the string \"null\"": {src: ": x\n\"null\": y\n", merged: "null"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Run("a map[any]any keeps both, and is the only one that does", func(t *testing.T) {
				var got map[any]any
				require.NoError(t, codec.Unmarshal([]byte(tc.src), &got))
				assert.Len(t, got, 2)
				assert.Contains(t, got, tc.merged, "the quoted key stays a string")
			})

			t.Run("a typed string map refuses it as a duplicate", func(t *testing.T) {
				var got map[string]any
				err := codec.Unmarshal([]byte(tc.src), &got)
				require.Error(t, err)
				assert.Contains(t, err.Error(), `duplicate key "`+tc.merged+`"`)
			})

			t.Run("today an `any` keeps one entry and reports nothing", func(t *testing.T) {
				var got any
				require.NoError(t, codec.Unmarshal([]byte(tc.src), &got))
				assert.Equal(t, map[string]any{tc.merged: "y"}, got)
			})

			t.Run("an ordered map keeps both entries under one name", func(t *testing.T) {
				var got any
				require.NoError(t, codec.UnmarshalWithOptions([]byte(tc.src), &got, codec.UseOrderedMap()))
				assert.Equal(t, codec.MapSlice{
					{Key: tc.merged, Value: "x"},
					{Key: tc.merged, Value: "y"},
				}, got)
			})

			t.Run("ToJSON refuses it rather than writing the name twice", func(t *testing.T) {
				_, err := codec.ToJSON([]byte(tc.src))
				require.Error(t, err)
				assert.ErrorIs(t, err, yamlerrors.ErrNotJSON)
				assert.Contains(t, err.Error(), `two keys write the JSON member "`+tc.merged+`"`)
			})
		})
	}
}
