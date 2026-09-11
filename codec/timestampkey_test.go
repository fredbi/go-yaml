// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"testing"
	"time"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
)

// TestATimestampKeyIsItsInstant covers a "!!timestamp" mapping key, which every
// destination holds as the instant it resolves to.
//
// The duplicate check compares the instant in UTC, the canonical form
// yaml.org/type/timestamp.html gives, so two spellings of one instant are one
// key written twice and the document is refused. Compared by the text, they
// passed the check and met in the decoder as one time.Time, which kept the
// second value and dropped the first with nothing reported.
func TestATimestampKeyIsItsInstant(t *testing.T) {
	t.Parallel()

	t.Run("an any and a MapSlice hold the same time.Time", func(t *testing.T) {
		t.Parallel()

		const src = "? !!timestamp \"1900-01-01T00:00:00Z\"\n: one\n"
		want := time.Date(1900, time.January, 1, 0, 0, 0, 0, time.UTC)

		var asAny any
		require.NoError(t, codec.Unmarshal([]byte(src), &asAny))
		assert.Equal(t, map[any]any{want: "one"}, asAny)

		var ordered codec.MapSlice
		require.NoError(t, codec.UnmarshalWithOptions([]byte(src), &ordered, codec.UseOrderedMap()))
		require.Equal(t, 1, ordered.Len())
		assert.Equal(t, want, ordered.At(0).Key)
	})

	t.Run("a string-keyed map and ToJSON name it alike", func(t *testing.T) {
		t.Parallel()

		// RFC 3339 in the zone the document wrote. A date with no zone is in
		// UTC.
		for src, name := range map[string]string{
			"!!timestamp 2001-12-14t21:59:43.10-05:00: one\n": "2001-12-14T21:59:43.1-05:00",
			"!!timestamp 2001-12-14: one\n":                   "2001-12-14T00:00:00Z",
		} {
			var named map[string]any
			require.NoError(t, codec.Unmarshal([]byte(src), &named))
			assert.Equal(t, map[string]any{name: "one"}, named)

			out, err := codec.ToJSON([]byte(src))
			require.NoError(t, err)
			assert.JSONEq(t, `{"`+name+`": "one"}`, string(out))
		}
	})

	t.Run("two spellings of one instant are refused", func(t *testing.T) {
		t.Parallel()

		for _, pair := range [][2]string{
			{"1900-01-01T00:00:00Z", "1900-01-01 00:00:00Z"},
			{"2001-12-14", "2001-12-14 00:00:00"},
			{"2001-12-15 02:59:43.10", "2001-12-15T02:59:43.1Z"},
			{"2001-12-15 2:59:43.10", "2001-12-15 02:59:43.10"},
			// One offset written two ways. Go keeps one *time.Location per
			// whole-hour offset, so the two time.Time values are ==.
			{"2001-12-14t21:59:43.10-05:00", "2001-12-14 21:59:43.10 -05:00"},
			// Offset zero written two ways. "+00:00" builds a fixed zone and
			// "Z" builds UTC, so the two time.Time values are not ==, and the
			// canonical form still makes them one key.
			{"2001-12-15T02:59:43.1+00:00", "2001-12-15T02:59:43.1Z"},
			// One instant in two zones: the canonical form is in UTC.
			{"2001-12-14t21:59:43.10-05:00", "2001-12-15 02:59:43.10"},
		} {
			src := "? !!timestamp \"" + pair[0] + "\"\n: one\n" +
				"? !!timestamp \"" + pair[1] + "\"\n: two\n"

			var asAny any
			require.ErrorIsf(t, codec.Unmarshal([]byte(src), &asAny), yamlerrors.ErrDuplicateKey,
				"into an any: %q", src)

			var named map[string]any
			require.ErrorIsf(t, codec.Unmarshal([]byte(src), &named), yamlerrors.ErrDuplicateKey,
				"into a map[string]any: %q", src)

			var ordered codec.MapSlice
			require.ErrorIsf(t, codec.UnmarshalWithOptions([]byte(src), &ordered, codec.UseOrderedMap()),
				yamlerrors.ErrDuplicateKey, "into a MapSlice: %q", src)
		}
	})

	t.Run("two instants are two keys", func(t *testing.T) {
		t.Parallel()

		const src = "!!timestamp 2001-12-14: one\n!!timestamp 2001-12-14 00:00:01: two\n"

		var asAny map[any]any
		require.NoError(t, codec.Unmarshal([]byte(src), &asAny))
		assert.Len(t, asAny, 2)
	})

	t.Run("a string spelling the canonical form is another key", func(t *testing.T) {
		t.Parallel()

		const src = "\"2001-12-14T00:00:00Z\": one\n!!timestamp 2001-12-14: two\n"

		var asAny any
		require.NoError(t, codec.Unmarshal([]byte(src), &asAny))
		assert.Equal(t, map[any]any{
			"2001-12-14T00:00:00Z": "one",
			time.Date(2001, time.December, 14, 0, 0, 0, 0, time.UTC): "two",
		}, asAny)
	})
}
