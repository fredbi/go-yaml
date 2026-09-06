// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"encoding/json"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	yaml "github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/codec"
)

// A merge key whose value is written where it stands makes ToJSON write JSON
// that will not parse.
//
// The 1.1 merge type says "<<" takes a mapping or a sequence of mappings and
// says nothing about how the value got there, so "<<: {a: 1}" asks for the same
// merge that "<<: *b" does. The decoder performs both correctly. ToJSON
// performs the merge and then writes the "<<" entry's own slot as well, so the
// output carries a member with no key, or a whole mapping where a key belongs.
//
// Found on 2026-09-11: every shape in yamlcorpus.MergeShapes wrote its merge as
// an alias, and adding the two that do not exposed it.

// TestDefectAMergeWrittenInPlaceMakesToJSONWriteBrokenJSON pins the output for
// the shapes that break and for the neighbors that do not.
func TestDefectAMergeWrittenInPlaceMakesToJSONWriteBrokenJSON(t *testing.T) {
	t.Run("today the JSON does not parse", func(t *testing.T) {
		for _, tc := range []struct{ src, writes string }{
			{src: "<<: {a: 1}\n", writes: `{:"a":1}`},
			{src: "z: 9\n<<: {a: 1}\n", writes: `{"z":,"a":1}`},
			{src: "<<: [{a: 1}, {b: 2}]\n", writes: `{,{"b":2}"a":1}`},
			{src: "z: 9\n<<: [{a: 1}, {b: 2}]\n", writes: `{"z":9,{"b":2},"a":1}`},
			{src: "b: &b {q: 1}\n<<: [*b, {a: 1}]\n", writes: `{"b":{"q":1},,"q":1,"a":1}`},
		} {
			out, err := codec.ToJSON([]byte(tc.src))
			require.NoError(t, err, "%q", tc.src)
			assert.Equal(t, tc.writes, string(out), "%q", tc.src)
			assert.False(t, json.Valid(out), "today: %q writes JSON that will not parse", tc.src)
		}
	})

	t.Run("three neighbors are written correctly", func(t *testing.T) {
		for _, tc := range []struct{ src, writes string }{
			// An alias, which is how every shape wrote it before.
			{src: "b: &b {q: 1}\n<<: *b\n", writes: `{"b":{"q":1},"q":1}`},
			{src: "b: &b {q: 1}\nc: &c {r: 1}\n<<: [*b, *c]\n", writes: `{"b":{"q":1},"c":{"r":1},"q":1,"r":1}`},
			// One entry in the sequence, which takes the path that works.
			{src: "z: 9\n<<: [{a: 1}]\n", writes: `{"z":9,"a":1}`},
		} {
			out, err := codec.ToJSON([]byte(tc.src))
			require.NoError(t, err, "%q", tc.src)
			assert.Equal(t, tc.writes, string(out), "%q", tc.src)
			assert.True(t, json.Valid(out), "%q", tc.src)
		}
	})

	t.Run("the decoder merges every one of them correctly", func(t *testing.T) {
		for _, tc := range []struct {
			src  string
			want map[string]any
		}{
			{src: "<<: {a: 1}\n", want: map[string]any{"a": uint64(1)}},
			{src: "z: 9\n<<: {a: 1}\n", want: map[string]any{"z": uint64(9), "a": uint64(1)}},
			{src: "<<: [{a: 1}, {b: 2}]\n", want: map[string]any{"a": uint64(1), "b": uint64(2)}},
		} {
			var got any
			require.NoError(t, yaml.Unmarshal([]byte(tc.src), &got), "%q", tc.src)
			assert.Equal(t, tc.want, got, "%q", tc.src)
		}
	})
}
