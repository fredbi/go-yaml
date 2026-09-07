// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
)

// ToJSON skips an empty first document where the decoder keeps it.
//
// "---" over "---" over "b: 2" is a stream of two documents, the first empty.
// The decoder reads the first and gives null; ToJSON writes {"b":2}, which is
// the second. They agree on every stream whose first document has content --
// both take the first -- so it is the empty one they part company over.
//
// Which is right is a question about what converting a *stream* to JSON should
// mean at all, and the two answers here are the two readings: take the first
// document, or take the first that has anything in it. The defect is that one
// library gives both.
//
// Found on 2026-09-13 by the stream axis, once the corpus carried a document
// suffix into TestToJSONMatchesTheValueConverter.

// TestDefectToJSONSkipsAnEmptyFirstDocument pins both sides.
func TestDefectToJSONSkipsAnEmptyFirstDocument(t *testing.T) {
	t.Run("today the two converters take different documents", func(t *testing.T) {
		const src = "---\n---\nb: 2\n"

		out, err := codec.ToJSON([]byte(src))
		require.NoError(t, err)
		assert.Equal(t, `{"b":2}`, string(out), "today: ToJSON takes the second")

		assert.Equal(t, "null\n", throughValues(t, src), "the decoder takes the first")
	})

	t.Run("a first document with content is taken by both", func(t *testing.T) {
		for _, tc := range []struct{ src, writes string }{
			{src: "a: 1\n---\nb: 2\n", writes: `{"a":1}`},
			{src: "a: 1\n...\n---\nb: 2\n", writes: `{"a":1}`},
			{src: "1\n---\n2\n---\n3\n", writes: "1"},
		} {
			out, err := codec.ToJSON([]byte(tc.src))
			require.NoErrorf(t, err, "%q", tc.src)
			assert.Equalf(t, tc.writes, string(out), "%q", tc.src)
		}
	})
}

// throughValues is the converter ToJSON is held against: decode, then marshal.
func throughValues(t *testing.T, src string) string {
	t.Helper()

	var v any
	require.NoError(t, codec.UnmarshalWithOptions([]byte(src), &v, codec.UseOrderedMap()))

	out, err := codec.MarshalWithOptions(v, codec.JSON())
	require.NoError(t, err)

	return string(out)
}
