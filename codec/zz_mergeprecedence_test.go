// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
)

// TestAMergeResolvesTheSameWayOnEveryPath holds the four readers of a document
// to one answer about "<<".
//
// The merge type says a mapping's own keys win over the ones it merges in, and
// that an earlier mapping of a "<<" sequence wins over a later one. The walking
// path and ToJSON applied both; the map and ordered-map paths wrote entries in
// document order and let the last writer win, so:
//
//   - "x: 9" over "<<: *a" read the merged x, losing the key the mapping wrote
//     itself -- the "<<" came second and overwrote it;
//   - "<<: [*a, *b]" read b's x where the spec gives it to a;
//   - UseOrderedMap appended every merged entry, so the MapSlice held x twice
//     and json.Marshal wrote a repeated member name.
//
// go.yaml.in/yaml/v3 v3.0.5 agrees with the answers below on all four. libfyaml
// 1.0.0b1 is no oracle here: it does not resolve a merge at all and writes the
// "<<" out as a member name of its own.
func TestAMergeResolvesTheSameWayOnEveryPath(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      map[string]any
		order     []string
	}{
		{
			name:  "the mapping's own key wins over a later <<",
			src:   "m:\n  x: 9\n  <<: &a {x: 1}\n",
			want:  map[string]any{"x": uint64(9)},
			order: []string{"x"},
		},
		{
			name:  "and over an earlier one",
			src:   "m:\n  <<: &a {x: 1}\n  x: 9\n",
			want:  map[string]any{"x": uint64(9)},
			order: []string{"x"},
		},
		{
			name:  "the earlier mapping of a sequence wins",
			src:   "a: &a {x: 1}\nb: &b {x: 2}\nm:\n  <<: [*a, *b]\n",
			want:  map[string]any{"x": uint64(1)},
			order: []string{"x"},
		},
		{
			name:  "and the mapping's own key wins over both",
			src:   "a: &a {x: 1}\nb: &b {x: 2}\nm:\n  x: 9\n  <<: [*a, *b]\n",
			want:  map[string]any{"x": uint64(9)},
			order: []string{"x"},
		},
		{
			name:  "a sequence of disjoint mappings brings all of them",
			src:   "a: &a {x: 1}\nb: &b {y: 2}\nm:\n  <<: [*a, *b]\n",
			want:  map[string]any{"x": uint64(1), "y": uint64(2)},
			order: []string{"x", "y"},
		},
		{
			name:  "a merge of a mapping that merges",
			src:   "a: &a {x: 1}\nb: &b\n  <<: *a\n  y: 2\nm:\n  <<: *b\n",
			want:  map[string]any{"x": uint64(1), "y": uint64(2)},
			order: []string{"y", "x"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var walked any
			require.NoErrorf(t, codec.Unmarshal([]byte(tc.src), &walked), "%q", tc.src)
			assert.Equal(t, tc.want, walked.(map[string]any)["m"], "into an any")

			var typed map[string]any
			require.NoErrorf(t, codec.Unmarshal([]byte(tc.src), &typed), "%q", tc.src)
			assert.Equal(t, tc.want, typed["m"], "into a map[string]any")

			var ordered any
			require.NoErrorf(t,
				codec.UnmarshalWithOptions([]byte(tc.src), &ordered, codec.UseOrderedMap()), "%q", tc.src)
			assert.Equal(t, tc.order, orderedKeysOf(t, ordered, "m"),
				"UseOrderedMap: the mapping's own keys in document order, then the merged ones")
		})
	}
}

// TestAMergedKeyIsWrittenOnceIntoAMapSlice is the half a map cannot show.
//
// A MapSlice keeps what the document wrote in order, and nothing deduplicated
// it: every merged entry was appended, so a mapping that overrode a merged key
// came back holding that key twice. A caller ranging over it saw both, and
// json.Marshal wrote a JSON object with the member name repeated.
func TestAMergedKeyIsWrittenOnceIntoAMapSlice(t *testing.T) {
	for _, src := range []string{
		"m:\n  <<: &a {x: 1}\n  x: 9\n",
		"m:\n  x: 9\n  <<: &a {x: 1}\n",
		"a: &a {x: 1}\nb: &b {x: 2}\nm:\n  x: 9\n  <<: [*a, *b]\n",
	} {
		var got any
		require.NoErrorf(t, codec.UnmarshalWithOptions([]byte(src), &got, codec.UseOrderedMap()), "%q", src)

		m := mapSliceAt(t, got, "m")
		require.Lenf(t, m, 1, "%q: the MapSlice holds %v", src, m)
		assert.Equal(t, "x", m[0].Key, "%q", src)
		assert.Equal(t, uint64(9), m[0].Value, "%q: the mapping's own value", src)
	}
}

// mapSliceAt is the MapSlice standing at key of the document's root mapping.
func mapSliceAt(t *testing.T, doc any, key string) codec.MapSlice {
	t.Helper()

	root, ok := doc.(codec.MapSlice)
	require.Truef(t, ok, "the document read as %T and not a MapSlice", doc)

	for _, item := range root {
		if item.Key != key {
			continue
		}
		m, ok := item.Value.(codec.MapSlice)
		require.Truef(t, ok, "%q holds %T and not a MapSlice", key, item.Value)

		return m
	}
	require.Failf(t, "no entry", "the document holds no %q", key)

	return nil
}

// orderedKeysOf is the keys of the MapSlice at key, in the order it holds them.
func orderedKeysOf(t *testing.T, doc any, key string) []string {
	t.Helper()

	var keys []string
	for _, item := range mapSliceAt(t, doc, key) {
		k, ok := item.Key.(string)
		require.Truef(t, ok, "a key read as %T", item.Key)
		keys = append(keys, k)
	}

	return keys
}
