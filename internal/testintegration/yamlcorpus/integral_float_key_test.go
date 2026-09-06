// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus_test

import (
	"bytes"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
)

// A mapping key that resolves to a float with a whole value.
//
// Recorded as a Departure with libfyaml corroborating, and pinned here so the
// fix breaks the test that says it was broken.

func keyed(t *testing.T, src string) map[string]any {
	t.Helper()

	var v any
	require.NoError(t, codec.NewDecoder(bytes.NewReader([]byte(src))).Decode(&v), "%q", src)

	m, ok := v.(map[string]any)
	require.True(t, ok, "%q decoded to %T", src, v)

	return m
}

// TestDefectAWholeValuedFloatKeyIsNamedAsAnInteger pins today's behavior.
//
// The right answers, which libfyaml 1.0.0b1 gives, are in the want column.
func TestDefectAWholeValuedFloatKeyIsNamedAsAnInteger(t *testing.T) {
	for _, tc := range []struct{ src, today, want string }{
		{src: "1.0: a\n", today: "1", want: "1.0"},
		{src: "1e3: a\n", today: "1000", want: "1000.0"},
		{src: "1000.0: a\n", today: "1000", want: "1000.0"},
		{src: "-0.0: a\n", today: "-0", want: "-0.0"},
	} {
		got := keyed(t, tc.src)
		assert.Contains(t, got, tc.today,
			"today: %q is keyed %q, and libfyaml keys it %q", tc.src, tc.today, tc.want)
	}
}

// TestDefectAWholeValuedFloatKeySwallowsTheIntegerBesideIt is the consequence,
// and the reason this is worth a departure rather than a note about spelling.
//
// Two keys of different types, one of which loses its type when it is named.
// The document is read without complaint and one of its entries is gone.
func TestDefectAWholeValuedFloatKeySwallowsTheIntegerBesideIt(t *testing.T) {
	got := keyed(t, "1.0: a\n1: b\n")

	assert.Len(t, got, 1, "today: two entries come back as one")
	assert.Equal(t, "b", got["1"], "today: the float's value is gone and nothing reported it")

	t.Run("and the same pair is refused when both are written alike", func(t *testing.T) {
		var v any
		err := codec.NewDecoder(bytes.NewReader([]byte("1.0: a\n\"1.0\": b\n"))).Decode(&v)
		require.Error(t, err, "a duplicate is caught when the texts match")
		assert.Contains(t, err.Error(), `mapping key "1.0" already defined`)
	})
}

// TestAFractionalFloatKeyIsFine is what narrows it.
//
// The bug is Go's %v on a float64 and not float keys in general, so a key with
// a fraction keeps its spelling and agrees with libfyaml.
func TestAFractionalFloatKeyIsFine(t *testing.T) {
	for _, tc := range []struct{ src, key string }{
		{src: "0.5: a\n", key: "0.5"},
		{src: "2.50: a\n", key: "2.5"},
		{src: "-1.25: a\n", key: "-1.25"},
	} {
		assert.Contains(t, keyed(t, tc.src), tc.key, "%q", tc.src)
	}
}
