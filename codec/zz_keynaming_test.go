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
// entries. `true`/`"true"` and `~`/`"null"` are the same shape.
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
