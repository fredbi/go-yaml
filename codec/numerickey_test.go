// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"math"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
)

// TestANumberKeyIsOneKeyWhateverItsGoType covers a number used as a mapping
// key.
//
// YAML has one integer type and one float type. Go has several of each, and
// == tells int64(0) from uint64(0) and one *big.Int from another holding the
// same number. A key's identity follows YAML's types: two spellings of one
// number are one key, whatever Go value each decodes to.
func TestANumberKeyIsOneKeyWhateverItsGoType(t *testing.T) {
	t.Parallel()

	t.Run("two spellings of one number are refused", func(t *testing.T) {
		t.Parallel()

		for _, src := range []string{
			"-0: a\n0: b\n",
			"!!int -0: a\n0: b\n",
			// Go's == holds -0.0 and 0.0 as one map key, so named apart the
			// pair passed the parser and the decoder kept "b" alone.
			"-0.0: a\n0.0: b\n",
			// Too wide for a float64, and named by the text they were.
			"1e400: a\n10e399: b\n",
		} {
			var asAny any
			require.ErrorIsf(t, codec.Unmarshal([]byte(src), &asAny), yamlerrors.ErrDuplicateKey, "into an any: %q", src)

			var ordered codec.MapSlice
			require.ErrorIsf(t, codec.UnmarshalWithOptions([]byte(src), &ordered, codec.UseOrderedMap()),
				yamlerrors.ErrDuplicateKey, "into a MapSlice: %q", src)

			_, err := codec.ToJSON([]byte(src))
			require.ErrorIsf(t, err, yamlerrors.ErrDuplicateKey, "ToJSON: %q", src)
		}
	})

	t.Run("a negative zero decodes as zero and keeps its sign as a float", func(t *testing.T) {
		t.Parallel()

		var v map[string]any
		require.NoError(t, codec.Unmarshal([]byte("i: -0\nf: -0.0\n"), &v))
		assert.Equal(t, uint64(0), v["i"])
		f, isFloat := v["f"].(float64)
		require.True(t, isFloat)
		assert.True(t, math.Signbit(f), "the float value -0.0 keeps its sign")
	})

	t.Run("an own key beats a merged key of the same value", func(t *testing.T) {
		t.Parallel()

		for _, pair := range [][2]string{
			{"0", "-0"},
			{"0.0", "-0.0"},
			{"123456789012345678901", "+123456789012345678901"},
			// A merge is 1.1's, and a 1.1 float wants a point and a signed
			// exponent: "1e400" is a string there.
			{"1.0e+400", "10.0e+399"},
		} {
			anchor := "%YAML 1.1\n---\na: &m\n  ? " + pair[0] + "\n  : merged\n"
			own := "  ? " + pair[1] + "\n  : own\n"
			for _, src := range []string{
				anchor + "b:\n  <<: *m\n" + own,
				anchor + "b:\n" + own + "  <<: *m\n",
			} {
				var asAny map[string]any
				require.NoErrorf(t, codec.Unmarshal([]byte(src), &asAny), "%q", src)
				b, isMap := asAny["b"].(map[any]any)
				require.Truef(t, isMap, "%q: b is %T", src, asAny["b"])
				assert.Lenf(t, b, 1, "into an any: %q", src)
				for _, value := range b {
					assert.Equalf(t, "own", value, "into an any: %q", src)
				}

				var ordered map[string]codec.MapSlice
				require.NoErrorf(t, codec.UnmarshalWithOptions([]byte(src), &ordered, codec.UseOrderedMap()), "%q", src)
				require.Equalf(t, 1, ordered["b"].Len(), "into a MapSlice: %q", src)
				assert.Equalf(t, "own", ordered["b"].At(0).Value, "into a MapSlice: %q", src)
			}
		}
	})

	t.Run("a MapSlice holds one entry per YAML key", func(t *testing.T) {
		t.Parallel()

		// Both would write "1:" and "0.5:" twice, which is no YAML mapping.
		_, err := codec.NewMapSlice(codec.MapItem{Key: 1, Value: "a"}, codec.MapItem{Key: uint64(1), Value: "b"})
		require.ErrorIs(t, err, codec.ErrDuplicateKey)
		_, err = codec.NewMapSlice(codec.MapItem{Key: float32(0.5), Value: "a"}, codec.MapItem{Key: 0.5, Value: "b"})
		require.ErrorIs(t, err, codec.ErrDuplicateKey)

		// An integer and a float are two keys, as "1" and "1.0" are.
		m, err := codec.NewMapSlice(codec.MapItem{Key: 1, Value: "a"}, codec.MapItem{Key: 1.0, Value: "b"})
		require.NoError(t, err)
		assert.Equal(t, 2, m.Len())

		// Set finds the entry whatever Go type the key is given as.
		require.NoError(t, m.Set(int64(1), "c"))
		assert.Equal(t, 2, m.Len())
		got, held := m.Get(uint8(1))
		assert.True(t, held)
		assert.Equal(t, "c", got)
	})
}
