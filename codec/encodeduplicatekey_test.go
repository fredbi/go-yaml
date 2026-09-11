// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"math"
	"testing"
	"time"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
)

// TestTheEncoderRefusesTwoKeysThatAreOneYAMLKey covers a Go map whose keys are
// two Go values and one YAML key.
//
// Go's == tells int(1) from uint64(1), one NaN from another and one instant in
// two zones from itself; YAML does not. Written as they stand, such a map
// gave a document with a key twice, which the decoder then refused. The
// encoder refuses it first, as the decoder does, unless SkipDuplicateMapKey
// asks it to write the first and drop the rest.
func TestTheEncoderRefusesTwoKeysThatAreOneYAMLKey(t *testing.T) {
	t.Parallel()

	instant := time.Date(2001, time.December, 15, 2, 59, 43, 0, time.UTC)

	t.Run("one YAML key twice is refused", func(t *testing.T) {
		t.Parallel()

		for name, v := range map[string]any{
			"int and uint64":            map[any]any{1: "a", uint64(1): "b"},
			"int64 and int":             map[any]any{int64(-1): "a", -1: "b"},
			"float32 and float64":       map[any]any{float32(0.5): "a", 0.5: "b"},
			"two NaN":                   map[float64]string{math.NaN(): "a", math.NaN(): "b"},
			"one instant in two zones":  map[time.Time]string{instant: "a", instant.In(time.FixedZone("", -5*3600)): "b"},
			"a Base64 written two ways": map[any]any{codec.Base64("AA=="): "a", codec.Base64("AA\n=="): "b"},
		} {
			_, err := codec.Marshal(v)
			require.ErrorIsf(t, err, yamlerrors.ErrDuplicateKey, "%s", name)
		}

		_, err := codec.MarshalWithOptions(map[any]any{1: "a", uint64(1): "b"}, codec.JSON())
		require.ErrorIs(t, err, yamlerrors.ErrDuplicateKey)
	})

	t.Run("two YAML keys of one spelling are written", func(t *testing.T) {
		t.Parallel()

		// An integer and a float are two keys, and so are binary data and the
		// string its base64 spells.
		for _, v := range []map[any]any{
			{1: "a", 1.0: "b"},
			{codec.Base64("AA=="): "a", "AA==": "b"},
		} {
			out, err := codec.Marshal(v)
			require.NoError(t, err)

			var back map[any]any
			require.NoErrorf(t, codec.Unmarshal(out, &back), "%q", out)
			assert.Lenf(t, back, 2, "%q", out)
		}
	})

	t.Run("SkipDuplicateMapKey writes the first", func(t *testing.T) {
		t.Parallel()

		// The keys are ordered by their text and then by their Go type, so
		// int comes before uint64 whatever order the map iterates in.
		for range 20 {
			out, err := codec.MarshalWithOptions(map[any]any{1: "a", uint64(1): "b"}, codec.SkipDuplicateMapKey())
			require.NoError(t, err)
			assert.Equal(t, "1: a\n", string(out))
		}
	})
}
