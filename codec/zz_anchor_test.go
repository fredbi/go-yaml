// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	yaml "github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/codec"
)

// ToJSON loses an anchor declared on a flow entry written as a key alone, when
// the entry's tag stands before its anchor.
//
// Three things have to be true at once, and changing any one of them makes it
// work: the entry is in a flow *mapping*, it is written as a key with no value,
// and its tag comes before its anchor. So "{!!null &a1 null, k: *a1}" loses the
// anchor where "{&a1 !!null null, k: *a1}", "{!!str &a1 x: 1, k: *a1}" and
// "[!!null &a1 null, *a1]" all keep it.
//
// The decoder reads every one of them, libfyaml 1.0.0b1 reads them, and the
// reference parser passes them. Found on 2026-09-11 by the generator, once the
// flow axes were weighted to reach a key written alone more than once in 350
// documents.

// TestDefectToJSONLosesAnAnchorOnATaggedFlowKeyAlone pins today's behavior on
// both sides of that line.
func TestDefectToJSONLosesAnAnchorOnATaggedFlowKeyAlone(t *testing.T) {
	t.Run("today the anchor is lost", func(t *testing.T) {
		for _, src := range []string{
			"{!!null &a1 null, k: *a1}\n",
			"{!!str &a1 x, k: *a1}\n",
		} {
			_, err := codec.ToJSON([]byte(src))
			require.Error(t, err, "today: %q loses the anchor", src)
			assert.Contains(t, err.Error(), `could not find alias "a1"`)
		}
	})

	t.Run("changing any one of the three makes it work", func(t *testing.T) {
		for _, tc := range []struct{ src, writes string }{
			// The anchor before the tag.
			{src: "{&a1 !!null null, k: *a1}\n", writes: `{"null":null,"k":null}`},
			// The entry carries a value.
			{src: "{!!str &a1 x: 1, k: *a1}\n", writes: `{"x":1,"k":"x"}`},
			// A sequence rather than a mapping.
			{src: "[!!null &a1 null, *a1]\n", writes: `[null,null]`},
			// A block mapping.
			{src: "a: !!null &a1 null\nk: *a1\n", writes: `{"a":null,"k":null}`},
		} {
			out, err := codec.ToJSON([]byte(tc.src))
			require.NoErrorf(t, err, "%q", tc.src)
			assert.Equal(t, tc.writes, string(out), "%q", tc.src)
		}
	})

	t.Run("the decoder reads all of them", func(t *testing.T) {
		for _, src := range []string{
			"{!!null &a1 null, k: *a1}\n",
			"{!!str &a1 x, k: *a1}\n",
		} {
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(src), &got), "%q", src)
			assert.NotNil(t, got, "%q", src)
		}
	})
}
