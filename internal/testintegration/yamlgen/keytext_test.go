// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
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
// A "!!binary" key is named by its canonical base64 text -- "!!binary AA==" is
// the key "AA==", not the byte it decodes to -- and has a kind of its own, so it
// and the string "AA==" are two keys. Emit writes that same base64 through
// base64.StdEncoding, and quoting does not reach it: "!!binary AA==" and
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

// TestSameKeyAgreesWithTheDuplicateCheck holds yamlgen.SameKey to the library:
// a mapping written with both keys of a pair is refused as a repeat exactly when
// SameKey calls them one key.
//
// SameKey states its own rule and borrows none of the library's, so this
// compares two models and not one model with itself. Each pair sits on a
// boundary the rule draws: a YAML type against the string that spells it, one
// value in two Go types, zero's sign, and one instant in two zones.
func TestSameKeyAgreesWithTheDuplicateCheck(t *testing.T) {
	instant := time.Date(2001, 12, 15, 2, 59, 43, 100_000_000, time.UTC)
	stamp := func(at time.Time) yamlgen.Value {
		return yamlgen.Tagged{Tag: yamlgen.TagTimestamp, V: yamlgen.Timestamp{V: at}}
	}
	binary := func(b ...byte) yamlgen.Value {
		return yamlgen.Tagged{Tag: yamlgen.TagBinary, V: yamlgen.Binary{V: b}}
	}

	for _, tc := range []struct {
		name string
		a, b yamlgen.Value
	}{
		{"an integer and the string that spells it", yamlgen.Int{V: 1}, yamlgen.Str{V: "1"}},
		{"an integer and a float of one value", yamlgen.Int{V: 1}, yamlgen.Float{V: 1}},
		{"zero and negative zero", yamlgen.Float{V: 0}, yamlgen.Float{V: math.Copysign(0, -1)}},
		{"null and the string null", yamlgen.Null{}, yamlgen.Str{V: "null"}},
		{"a boolean and the string that spells it", yamlgen.Bool{V: true}, yamlgen.Str{V: "true"}},
		{"binary and the string of its base64", binary(0), yamlgen.Str{V: "AA=="}},
		{"one binary twice", binary(0, 1, 2), binary(0, 1, 2)},
		{"one instant in two zones", stamp(instant), stamp(instant.In(time.FixedZone("", -5*60*60)))},
		{"two instants", stamp(instant), stamp(instant.Add(time.Second))},
		{"a timestamp and the string that names it", stamp(instant), yamlgen.Str{V: "2001-12-15T02:59:43.1Z"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := yamlgen.Map{Pairs: []yamlgen.Pair{
				{Key: tc.a, Val: yamlgen.Str{V: "a"}},
				{Key: tc.b, Val: yamlgen.Str{V: "b"}},
			}}
			w := yamlgen.Write(m, yamlgen.Style{NullSpelling: "null", Quoting: yamlgen.QuotePlain})

			// Each side is held to its own outcome, so a document refused for
			// some other reason cannot pass as "two keys".
			var got any
			err := codec.Unmarshal([]byte(w.Text), &got)
			if yamlgen.SameKey(tc.a, tc.b) {
				assert.ErrorIs(t, err, yamlerrors.ErrDuplicateKey,
					"SameKey calls them one key, and the library read %q as %#v", w.Text, got)

				return
			}

			assert.NoError(t, err, "SameKey calls them two keys, and the library refused %q", w.Text)
		})
	}
}

// TestAFloatKeyIsNamedAsTheLibraryNamesIt holds KeyText's two float rules that
// moved on 2026-09-11 to the name a string-keyed map gives the key the emitter
// wrote. Zero is named without its sign, since §10.2.1.4 writes its canonical
// form as 0. A float too wide for a float64 is named by its value, and not by
// the "1.0e+330" the emitter writes for 1.1's sake.
//
// The wide floats are built as bigFloats builds them, with SetString.
func TestAFloatKeyIsNamedAsTheLibraryNamesIt(t *testing.T) {
	wide := func(text string) *big.Float {
		f, ok := new(big.Float).SetString(text)
		require.Truef(t, ok, "%q", text)

		return f
	}

	for _, key := range []yamlgen.Value{
		yamlgen.Float{V: math.Copysign(0, -1)},
		yamlgen.Float{V: 0},
		yamlgen.BigFloat{V: wide("1e400")},
		yamlgen.BigFloat{V: wide("-2500e-403")},
		yamlgen.BigFloat{V: wide("9999e330")},
	} {
		w := yamlgen.Write(yamlgen.Map{Pairs: []yamlgen.Pair{{Key: key, Val: yamlgen.Str{V: "v"}}}},
			yamlgen.Style{NullSpelling: "null", Quoting: yamlgen.QuotePlain})

		var named map[string]any
		require.NoErrorf(t, codec.Unmarshal([]byte(w.Text), &named), "%q", w.Text)
		assert.Equalf(t, map[string]any{yamlgen.KeyText(key): "v"}, named, "%q", w.Text)
	}
}
