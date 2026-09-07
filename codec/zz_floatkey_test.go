// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
)

// ToJSON names a float key written the long way by its source text.
//
// "? 1e3" over ": x" converts to {"1e3":"x"}; the same mapping written
// "1e3: x" converts to {"1000.0":"x"}. So two spellings of one document give
// two JSON documents, and which one you get depends on where the "?" is.
//
// §3.2.1.1 makes a key equal to another when they resolve to the same node, so
// the name belongs to the value and not to the text. The decoder names both
// "1000.0" and libfyaml 1.0.0b1 writes {"1000.0": "x"} for both.
//
// Floats only. "? 007", "? 0x1f", "? ~" and "? true" are named 7, 31, null and
// true by both converters, so the resolution reaches the key -- only the
// canonical spelling of a float does not.
//
// Found on 2026-09-13, by Style.ExplicitKeys crossing a float once the
// yamlcorpus/25 draw put the two together.

// TestDefectToJSONNamesALongFormFloatKeyByItsText pins the pair.
func TestDefectToJSONNamesALongFormFloatKeyByItsText(t *testing.T) {
	t.Run("today the long form keeps the text", func(t *testing.T) {
		for _, tc := range []struct{ src, folds string }{
			{src: "? 1e3\n: x\n", folds: `{"1e3":"x"}`},
			{src: "{? 1e3\n: x}\n", folds: `{"1e3":"x"}`},
			{
				src:   "? 0.000000000000000000000000000000000022811873366677198\n: x\n",
				folds: `{"0.000000000000000000000000000000000022811873366677198":"x"}`,
			},
		} {
			out, err := codec.ToJSON([]byte(tc.src))
			require.NoErrorf(t, err, "%q", tc.src)
			assert.Equal(t, tc.folds, string(out), "today: %q", tc.src)
		}
	})

	t.Run("the short form resolves, and so does the decoder for both", func(t *testing.T) {
		out, err := codec.ToJSON([]byte("1e3: x\n"))
		require.NoError(t, err)
		assert.Equal(t, `{"1000.0":"x"}`, string(out))

		for _, src := range []string{"1e3: x\n", "? 1e3\n: x\n"} {
			var got map[string]any
			require.NoErrorf(t, codec.Unmarshal([]byte(src), &got), "%q", src)
			assert.Equalf(t, map[string]any{"1000.0": "x"}, got, "%q", src)
		}
	})

	t.Run("every other scalar kind is named the same both ways", func(t *testing.T) {
		for _, tc := range []struct{ long, short, folds string }{
			{long: "? 007\n: x\n", short: "007: x\n", folds: `{"7":"x"}`},
			{long: "? 0x1f\n: x\n", short: "0x1f: x\n", folds: `{"31":"x"}`},
			{long: "? ~\n: x\n", short: "~: x\n", folds: `{"null":"x"}`},
			{long: "? true\n: x\n", short: "true: x\n", folds: `{"true":"x"}`},
		} {
			for _, src := range []string{tc.long, tc.short} {
				out, err := codec.ToJSON([]byte(src))
				require.NoErrorf(t, err, "%q", src)
				assert.Equalf(t, tc.folds, string(out), "%q", src)
			}
		}
	})
}
