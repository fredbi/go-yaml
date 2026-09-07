// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
)

// The two converters write a `!!timestamp` two ways, and ToJSON's answer
// depends on how the document was spelled.
//
// ToJSON writes the scalar's source text: "2001-12-14t21:59:43.1Z" goes out
// with its lowercase "t", and "2001-12-14 21:59:43.1" goes out with its space.
// Neither is RFC 3339, so a JSON consumer reading dates gets a string it cannot
// parse -- and two documents denoting the same instant convert to two different
// JSON documents. The value converter goes through the decoder, which resolves
// the tag to a time.Time, and writes "2001-12-14T21:59:43.1Z" for all of them.
//
// The field is split on which answer is right. libfyaml 1.0.0b1 writes the
// source text and go.yaml.in/yaml/v3 v3.0.5 writes the normalized instant. The
// defect here is that one library gives both, and that the answer moves with
// the presentation.
//
// `!!binary` has no such split: both converters write the decoded bytes as a
// JSON array of numbers. That differs from encoding/json, which writes a []byte
// as its base64 string, and from libfyaml, which writes "aGVsbG8=" -- recorded
// rather than pinned, since the two paths in this library agree.
//
// Found on 2026-09-13, once yamlgen drew a Timestamp and the corpus carried one
// into TestToJSONMatchesTheValueConverter.

// TestDefectToJSONWritesATimestampAsItWasSpelled pins both converters.
func TestDefectToJSONWritesATimestampAsItWasSpelled(t *testing.T) {
	t.Run("today ToJSON writes the source text", func(t *testing.T) {
		for _, tc := range []struct{ src, folds, values string }{
			{
				src:    "a: !!timestamp 2001-12-14\n",
				folds:  `{"a":"2001-12-14"}`,
				values: `{"a": "2001-12-14T00:00:00Z"}`,
			},
			{
				src:    "a: !!timestamp 2001-12-14t21:59:43.1Z\n",
				folds:  `{"a":"2001-12-14t21:59:43.1Z"}`,
				values: `{"a": "2001-12-14T21:59:43.1Z"}`,
			},
			{
				src:    "a: !!timestamp 2001-12-14 21:59:43.1\n",
				folds:  `{"a":"2001-12-14 21:59:43.1"}`,
				values: `{"a": "2001-12-14T21:59:43.1Z"}`,
			},
		} {
			out, err := codec.ToJSON([]byte(tc.src))
			require.NoErrorf(t, err, "%q", tc.src)
			assert.Equal(t, tc.folds, string(out), "today: %q", tc.src)

			var v any
			require.NoErrorf(t, codec.UnmarshalWithOptions([]byte(tc.src), &v, codec.UseOrderedMap()), "%q", tc.src)
			through, err := codec.MarshalWithOptions(v, codec.JSON())
			require.NoErrorf(t, err, "%q", tc.src)
			assert.Equal(t, tc.values+"\n", string(through), "%q", tc.src)
		}
	})

	t.Run("a binary value is written the same way by both", func(t *testing.T) {
		const src = "a: !!binary aGVsbG8=\n"

		out, err := codec.ToJSON([]byte(src))
		require.NoError(t, err)
		assert.Equal(t, `{"a":[104,101,108,108,111]}`, string(out))

		var v any
		require.NoError(t, codec.UnmarshalWithOptions([]byte(src), &v, codec.UseOrderedMap()))
		through, err := codec.MarshalWithOptions(v, codec.JSON())
		require.NoError(t, err)
		assert.Equal(t, `{"a": [104, 101, 108, 108, 111]}`+"\n", string(through))
	})

	t.Run("as a key, both tags make the converters disagree", func(t *testing.T) {
		for _, tc := range []struct{ src, folds, values string }{
			{
				src:    "!!timestamp 2001-12-14: x\n",
				folds:  `{"2001-12-14":"x"}`,
				values: `{"2001-12-14 00:00:00 +0000 UTC": "x"}`,
			},
			{
				src:    "!!binary aGVsbG8=: x\n",
				folds:  `{"[104,101,108,108,111]":"x"}`,
				values: `{"[104 101 108 108 111]": "x"}`,
			},
		} {
			out, err := codec.ToJSON([]byte(tc.src))
			require.NoErrorf(t, err, "%q", tc.src)
			assert.Equal(t, tc.folds, string(out), "today: %q", tc.src)

			var v any
			require.NoErrorf(t, codec.UnmarshalWithOptions([]byte(tc.src), &v, codec.UseOrderedMap()), "%q", tc.src)
			through, err := codec.MarshalWithOptions(v, codec.JSON())
			require.NoErrorf(t, err, "%q", tc.src)
			assert.Equal(t, tc.values+"\n", string(through), "%q", tc.src)
		}
	})
}
