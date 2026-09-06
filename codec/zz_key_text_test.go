// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"encoding/json"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
)

// TestAKeyIsNamedByWhatItWouldBeWrittenAs covers the text a non-string mapping
// key addresses its entry by.
//
// keyText wrote fmt.Sprint of the resolved value, which spells a float and an
// integer alike: float64(1) and int(1) are both "1". So "1: a" beside "1.0: b"
// -- two keys, since an integer and a float are two nodes -- came out of ToJSON
// as {"1":"a","1":"b"}, one name twice, and the decoder read them as one entry.
// Under UseStringKeys it read them as one entry too.
//
// The key is now written the way the value would be written, which is what
// keeps the digits that make a float one. gopkg.in/yaml.v3 keeps the same two
// apart, holding "1.0" under "1.0".
func TestAKeyIsNamedByWhatItWouldBeWrittenAs(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want map[string]any
	}{
		// The two keys that started this.
		{src: "1: a\n1.0: b\n", want: map[string]any{"1": "a", "1.0": "b"}},

		// A float keeps the digits the document wrote, so two spellings of one
		// number stay two keys.
		{src: "1.0: a\n", want: map[string]any{"1.0": "a"}},
		{src: "1.00: a\n", want: map[string]any{"1.00": "a"}},
		{src: "1e3: a\n1000: b\n", want: map[string]any{"1e3": "a", "1000": "b"}},

		// A null addresses its entry by "null", which is what keeps it apart
		// from a key written empty.
		{src: "\"\": a\nnull: b\n", want: map[string]any{"": "a", "null": "b"}},
	} {
		t.Run(tc.src, func(t *testing.T) {
			// Every path that names an entry by text agrees.
			var decoded map[string]any
			require.NoError(t, codec.Unmarshal([]byte(tc.src), &decoded))
			assert.Equal(t, tc.want, decoded, "the decoder")

			var stringKeyed map[string]any
			require.NoError(t, codec.UnmarshalWithOptions([]byte(tc.src), &stringKeyed, codec.UseStringKeys()))
			assert.Equal(t, tc.want, stringKeyed, "UseStringKeys")

			converted, err := codec.ToJSON([]byte(tc.src))
			require.NoError(t, err)

			var fromJSON map[string]any
			require.NoError(t, json.Unmarshal(converted, &fromJSON))
			assert.Equal(t, tc.want, fromJSON, "ToJSON wrote %s", converted)

			// And what it wrote holds as many names as the document wrote keys,
			// which is the thing that broke: JSON with one name twice is read
			// back with one of them gone.
			assert.Len(t, fromJSON, len(tc.want), "ToJSON wrote %s", converted)
		})
	}
}

// TestAnInfinityKeyKeepsItsSpelling covers the one scalar the JSON writer has
// no number for.
//
// appendScalarNode writes an infinity as null, which as a key would put it
// under the same name as a null. codec.ToJSON refuses the document before that
// is reached -- parser.WithJSONCompatible has no spelling for it either -- and
// the decoder does not, so the key is named by the text the document wrote.
// fmt.Sprint gave Go's "+Inf".
func TestAnInfinityKeyKeepsItsSpelling(t *testing.T) {
	for src, want := range map[string]string{
		".inf: a\n":  ".inf",
		"-.inf: a\n": "-.inf",
		".nan: a\n":  ".nan",
	} {
		t.Run(src, func(t *testing.T) {
			var decoded map[string]any
			require.NoError(t, codec.Unmarshal([]byte(src), &decoded))
			assert.Equal(t, map[string]any{want: "a"}, decoded)

			// ToJSON refuses it rather than naming it at all.
			_, err := codec.ToJSON([]byte(src))
			assert.Error(t, err)
		})
	}
}
