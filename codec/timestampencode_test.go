// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"strings"
	"testing"
	"time"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
)

// TestATimeTimeInAnInterfaceSlotIsWrittenTagged covers the encoder's spelling of
// a time.Time.
//
// The core schema resolves no timestamp, so a plain "2001-12-14T00:00:00Z"
// reads back as a string wherever the destination does not say time.Time. The
// encoder writes "!!timestamp" where the time.Time stands in an interface slot,
// and leaves a slot typed time.Time plain.
func TestATimeTimeInAnInterfaceSlotIsWrittenTagged(t *testing.T) {
	t.Parallel()

	utc := time.Date(2001, time.December, 14, 0, 0, 0, 0, time.UTC)
	zoned := time.Date(2001, time.December, 14, 21, 59, 43, 100000000, time.FixedZone("", -5*3600))

	t.Run("an interface slot reads back as the time.Time", func(t *testing.T) {
		t.Parallel()

		ordered, err := codec.NewMapSlice(codec.MapItem{Key: zoned, Value: utc})
		require.NoError(t, err)

		for name, tc := range map[string]struct {
			in   any
			tags int
		}{
			"a map[any]any key and value": {map[any]any{utc: zoned}, 2},
			"a value in a map[string]any": {map[string]any{"k": zoned}, 1},
			"an element of a []any":       {[]any{utc}, 1},
			"a MapSlice entry":            {ordered, 2},
		} {
			out, err := codec.Marshal(tc.in)
			require.NoErrorf(t, err, "%s", name)
			assert.Equalf(t, tc.tags, strings.Count(string(out), "!!timestamp"), "%s: %q", name, out)

			var back any
			require.NoErrorf(t, codec.Unmarshal(out, &back), "%s: %q", name, out)
			want := tc.in
			if name == "a MapSlice entry" {
				want = map[any]any{zoned: utc}
			}
			assert.Equalf(t, want, back, "%s: %q", name, out)
		}
	})

	t.Run("a slot typed time.Time stays plain", func(t *testing.T) {
		t.Parallel()

		type holder struct {
			Typed time.Time `yaml:"typed"`
			Held  any       `yaml:"held"`
		}
		in := holder{Typed: zoned, Held: zoned}

		out, err := codec.Marshal(in)
		require.NoError(t, err)
		assert.Equal(t, "typed: 2001-12-14T21:59:43.1-05:00\nheld: !!timestamp 2001-12-14T21:59:43.1-05:00\n", string(out))

		var back holder
		require.NoError(t, codec.Unmarshal(out, &back))
		assert.Equal(t, in, back)

		keyed, err := codec.Marshal(map[time.Time]string{utc: "v"})
		require.NoError(t, err)
		assert.Equal(t, "2001-12-14T00:00:00Z: v\n", string(keyed))
	})

	t.Run("JSON has no tag to write", func(t *testing.T) {
		t.Parallel()

		out, err := codec.MarshalWithOptions(map[string]any{"k": utc}, codec.JSON())
		require.NoError(t, err)
		assert.NotContains(t, string(out), "!!timestamp")
		assert.Contains(t, string(out), `"2001-12-14T00:00:00Z"`)
	})
}
