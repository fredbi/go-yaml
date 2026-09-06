// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus_test

import (
	"bytes"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
	"github.com/go-openapi/go-yaml/internal/testintegration/goyaml"
)

// What a decoder hands back for a mapping key that is not a string.
//
// Three behaviors are wanted and one is implemented. The decoder should keep
// the key's type, the way go.yaml.in/yaml/v3 does; codec.UseStringKeys should
// be what turns stringification on; and codec.ToJSON has to stringify, because
// a JSON member name is a string. Today all three stringify and the option
// turns nothing on.
//
// Pinned so the fix breaks the tests that say it was broken. Each case states
// the target beside today's answer.

func decodeInto(t *testing.T, src string, opts ...codec.DecodeOption) any {
	t.Helper()

	var v any
	require.NoError(t, codec.NewDecoder(bytes.NewReader([]byte(src)), opts...).Decode(&v), "%q", src)

	return v
}

// TestDefectTheDecoderStringifiesEveryKey pins the default.
//
// want is what yaml.v3 gives and what this decoder should give.
func TestDefectTheDecoderStringifiesEveryKey(t *testing.T) {
	for _, tc := range []struct {
		src   string
		today map[string]any
		want  any
	}{
		{src: "true: a\n", today: map[string]any{"true": "a"}, want: map[any]any{true: "a"}},
		{src: "~: a\n", today: map[string]any{"null": "a"}, want: map[any]any{nil: "a"}},
		{src: "0.5: a\n", today: map[string]any{"0.5": "a"}, want: map[any]any{0.5: "a"}},
	} {
		assert.Equal(t, tc.today, decodeInto(t, tc.src),
			"today: %q is stringified; it should keep the type, as %#v", tc.src, tc.want)
	}
}

// TestDefectUseStringKeysTurnsNothingOn is the sharper half.
//
// An option that documents a behavior and does not change one is worse than an
// absent option: a caller reading the godoc believes the default is the other
// thing.
func TestDefectUseStringKeysTurnsNothingOn(t *testing.T) {
	for _, src := range []string{"true: a\n", "~: a\n", "0.5: a\n", "1: a\n", "a: 1\n"} {
		assert.Equal(t, decodeInto(t, src), decodeInto(t, src, codec.UseStringKeys()),
			"today: %q reads the same with and without UseStringKeys", src)
	}
}

// TestToJSONStringifies is the one of the three that is right to stringify, and
// it is here so a fix to the other two does not take it with them.
//
// A JSON member name is a string, so ToJSON has no choice. What it must not do
// is lose the type on the way -- see the integral-float departure, which is the
// same stringification and is where the bug actually lives.
func TestToJSONStringifies(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{src: "true: a\n", want: `{"true":"a"}`},
		{src: "~: a\n", want: `{"null":"a"}`},
		{src: "0.5: a\n", want: `{"0.5":"a"}`},
	} {
		out, err := codec.ToJSON([]byte(tc.src))
		require.NoError(t, err, "%q", tc.src)
		assert.JSONEq(t, tc.want, string(out), "%q", tc.src)
	}
}

// TestYAMLv3KeepsTheKeyType is the corroboration, pinned so that an upgrade
// changing its mind is visible rather than silently re-scoring the departures.
func TestYAMLv3KeepsTheKeyType(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want any
	}{
		{src: "true: a\n", want: map[any]any{true: "a"}},
		{src: "~: a\n", want: map[any]any{nil: "a"}},
		{src: "0.5: a\n", want: map[any]any{0.5: "a"}},
		{src: "1.0: a\n", want: map[any]any{float64(1): "a"}},
		{src: "1: a\n", want: map[any]any{1: "a"}},
	} {
		docs, err := goyaml.Load([]byte(tc.src))
		require.NoError(t, err, "%q", tc.src)
		require.Len(t, docs, 1)
		assert.Equal(t, tc.want, docs[0], "%q", tc.src)
	}

	t.Run("so a float and an integer of the same value are two keys", func(t *testing.T) {
		docs, err := goyaml.Load([]byte("1.0: a\n1: b\n"))
		require.NoError(t, err)
		require.Len(t, docs, 1)
		assert.Len(t, docs[0], 2, "yaml.v3 keeps both, where this library keeps one")

		assert.Len(t, decodeInto(t, "1.0: a\n1: b\n"), 1, "today: this library collapses them")
	})
}
