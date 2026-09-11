// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"testing"
	"time"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"
)

// What the generator calls a key, held against what the library calls it.
//
// yamlgen.KeyText states the name the library gives a key: the key of a
// string-keyed map and the member name ToJSON writes. The stored corpus names
// every key through it and yamlgen.SameKey compares keys by it, so a name this
// package gets wrong puts a wrong answer in the corpus.

// TestABinaryKeyIsNamedByTheCharactersTheDocumentWrote pins both halves of one
// name.
//
// ast.TaggedKeyName resolves !!str, !!null, !!bool, !!int and !!float and hands
// every other tag back to the scalar under it, so "!!binary AA==" is the key
// "AA==" -- the base64 as written, not the byte it decodes to. Emit writes that
// same base64 through base64.StdEncoding, and quoting does not reach it: the
// identity comes from the token's unquoted value, so "!!binary AA==" and
// "!!binary \"AA==\"" are one key.
//
// KeyText had no Binary case until 2026-09-10, so a Binary fell to the
// collection branch and was named "[0]", Go's %v of []byte{0}. Keys() draws no
// Binary; aliasAKey put one in a key position by taking it from the anchor
// pool, which holds the Timestamp or Binary drawTextual makes one textual value
// in eight. That is why it surfaced as a rare draw rather than as a table.
func TestABinaryKeyIsNamedByTheCharactersTheDocumentWrote(t *testing.T) {
	for name, tc := range map[string]struct {
		bytes []byte
		src   string
		want  string
	}{
		"one byte, which base64 pads": {
			bytes: []byte{0},
			src:   "!!binary \"AA==\": v\n",
			want:  "AA==",
		},
		"three bytes, which it does not": {
			bytes: []byte{0, 1, 2},
			src:   "!!binary \"AAEC\": v\n",
			want:  "AAEC",
		},
		"the same base64 written plain": {
			bytes: []byte{0, 1, 2},
			src:   "!!binary AAEC: v\n",
			want:  "AAEC",
		},
	} {
		t.Run(name, func(t *testing.T) {
			binary := yamlgen.Tagged{Tag: yamlgen.TagBinary, V: yamlgen.Binary{V: tc.bytes}}
			assert.Equal(t, tc.want, yamlgen.KeyText(binary))

			var got any
			require.NoError(t, codec.Unmarshal([]byte(tc.src), &got))
			assert.Equal(t, map[any]any{codec.Base64(tc.want): "v"}, got,
				"the library keys it by the same characters, as a codec.Base64")
		})
	}
}

// TestATimestampKeyIsNamedAsTheLibraryNamesIt holds KeyText's Timestamp case to
// the library's two names for the key: the key of a string-keyed map and the
// member name ToJSON writes. Both are RFC 3339 in the zone the document wrote,
// and a date with no zone is UTC.
//
// Keys() draws no Timestamp. aliasAKey puts one in a key position from the
// anchor pool, where drawTextual makes one textual value in eight, so a table
// reaches it where a draw rarely does.
func TestATimestampKeyIsNamedAsTheLibraryNamesIt(t *testing.T) {
	for _, tc := range []struct {
		stamp time.Time
		src   string
	}{
		{time.Date(2001, 12, 14, 0, 0, 0, 0, time.UTC), "!!timestamp 2001-12-14: v\n"},
		{time.Date(2001, 12, 15, 2, 59, 43, 100_000_000, time.UTC), "!!timestamp 2001-12-15 2:59:43.10: v\n"},
		{time.Date(2001, 12, 15, 2, 59, 43, 100_000_000, time.UTC), "!!timestamp \"2001-12-15T02:59:43.1Z\": v\n"},
	} {
		key := yamlgen.KeyText(yamlgen.Tagged{Tag: yamlgen.TagTimestamp, V: yamlgen.Timestamp{V: tc.stamp}})

		var named map[string]any
		require.NoErrorf(t, codec.Unmarshal([]byte(tc.src), &named), "%q", tc.src)
		assert.Equalf(t, map[string]any{key: "v"}, named, "a string-keyed map names it so: %q", tc.src)

		out, err := codec.ToJSON([]byte(tc.src))
		require.NoErrorf(t, err, "%q", tc.src)
		assert.JSONEq(t, `{"`+key+`":"v"}`, string(out), "and so does ToJSON: %q", tc.src)
	}
}
