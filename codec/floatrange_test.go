// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
)

// TestAFloatPastTheExponentBoundIsAnInfinityOrZero covers a float whose
// decimal exponent is past ±1000, which every path reads as an infinity of its
// sign, or zero.
//
// Two faults sat past the bound. A number past big.Float's own range decoded to
// 0 -- "1e2147483647" read as zero on the walk and the tree. And a number
// inside big.Float's range stalled every reader when written as a key: naming
// it wrote it as decimal, in time that grew about with the square of the
// exponent, so "1e10000000: a" took over a minute to parse.
func TestAFloatPastTheExponentBoundIsAnInfinityOrZero(t *testing.T) {
	inf, negInf := math.Inf(1), math.Inf(-1)

	for src, want := range map[string]float64{
		"a: 1e1001\n":          inf,
		"a: -1e1001\n":         negInf,
		"a: 1e2147483647\n":    inf,
		"a: -1e2147483647\n":   negInf,
		"a: 1e10000000\n":      inf,
		"a: 1e-1001\n":         0,
		"a: 1e-2147483647\n":   0,
		"a: !!float 1e2000\n":  inf,
		"a: !!float -1e2000\n": negInf,
	} {
		// An any goes down the walk, a MapSlice under UseOrderedMap builds the
		// tree, and a struct takes the typed walk.
		var walked any
		require.NoErrorf(t, codec.Unmarshal([]byte(src), &walked), "the walk: %q", src)
		asMap, isMap := walked.(map[string]any)
		require.Truef(t, isMap, "the walk: %q", src)
		assert.Equalf(t, want, asMap["a"], "the walk: %q", src)

		var tree codec.MapSlice
		require.NoErrorf(t, codec.UnmarshalWithOptions([]byte(src), &tree, codec.UseOrderedMap()), "the tree: %q", src)
		v, _ := tree.Get("a")
		assert.Equalf(t, want, v, "the tree: %q", src)

		var typed struct{ A float64 }
		require.NoErrorf(t, codec.Unmarshal([]byte(src), &typed), "the typed walk: %q", src)
		assert.Equalf(t, want, typed.A, "the typed walk: %q", src)
	}

	t.Run("inside the bound a float too wide for a float64 stays a *big.Float", func(t *testing.T) {
		var walked any
		require.NoError(t, codec.Unmarshal([]byte("a: 1e1000\n"), &walked))
		asMap, isMap := walked.(map[string]any)
		require.True(t, isMap)
		assert.IsType(t, (*big.Float)(nil), asMap["a"], "the walk")
	})

	t.Run("as a key it is the key .inf is", func(t *testing.T) {
		var named map[string]string
		require.NoError(t, codec.Unmarshal([]byte("1e10000000: a\n"), &named))
		assert.Equal(t, map[string]string{".inf": "a"}, named)

		var keyed map[float64]string
		require.NoError(t, codec.Unmarshal([]byte("1e10000000: a\n"), &keyed))
		assert.Equal(t, map[float64]string{inf: "a"}, keyed)

		for _, src := range []string{"1e10000000: a\n.inf: b\n", "1e2000: a\n1e3000: b\n", "1e-2000: a\n0.0: b\n"} {
			var got any
			err := codec.Unmarshal([]byte(src), &got)
			assert.ErrorIsf(t, err, yamlerrors.ErrDuplicateKey, "%q", src)
		}
	})
}

// TestAFloatPastTheExponentBoundConvertsToTheNumberWritten covers ToJSON and
// ToJSONTokens on a float past ±1000.
//
// The decoder reads one as an infinity or zero, as Go's types require. JSON
// bounds no number, so both converters write the digits -- but only where the
// document spelled them as JSON does. "+1e1001", "1.e1001" and every spelling
// under "!!float" were read as the infinity and written as null, and
// "!!float 1e-1001" as 0.0.
func TestAFloatPastTheExponentBoundConvertsToTheNumberWritten(t *testing.T) {
	for src, want := range map[string]string{
		"k: !!float 1e1001\n":   "1e1001",
		"k: !!float -1e1001\n":  "-1e1001",
		"k: +1e1001\n":          "1e1001",
		"k: !!float +1e1001\n":  "1e1001",
		"k: 1.e1001\n":          "1e1001",
		"k: -1.e1001\n":         "-1e1001",
		"k: !!float 1e-1001\n":  "1e-1001",
		"k: !!float 1.5e1001\n": "1.5e1001",
		"k: !!float 007e1001\n": "7e1001",

		// Shapes outside the fault: JSON spells these as the document did.
		"k: 1e1001\n":  "1e1001",
		"k: 1E+1001\n": "1E+1001",
	} {
		t.Run(src, func(t *testing.T) {
			out, err := codec.ToJSON([]byte(src))
			require.NoError(t, err)
			assert.Equal(t, `{"k":`+want+`}`, string(out), "the digits as written")

			s := codec.ToJSONTokens([]byte(src))
			var got []string
			for tk := range s.Tokens() {
				got = append(got, fmt.Sprintf("%v:%s", tk.Kind, tk.Value))
			}
			require.NoError(t, s.Err())
			assert.Contains(t, got, "number:"+want)
		})
	}
}
