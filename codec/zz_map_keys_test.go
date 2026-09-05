// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"errors"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
)

// TestACollectionKeyIsRefusedNotPanicked records that a sequence or a mapping
// used as a mapping key is an error.
//
// It used to panic. A map[any]any takes any key the compiler can see, so
// nothing stopped a []interface{} reaching SetMapIndex, which paniced with
// "hash of unhashable type []interface {}" -- on a well-formed YAML document,
// so a fuzzer reached it.
func TestACollectionKeyIsRefusedNotPanicked(t *testing.T) {
	for _, src := range []string{"? [a]\n: 1\n", "? {a: 1}\n: 2\n", "? &x [a]\n: 1\n"} {
		t.Run(src, func(t *testing.T) {
			var into map[any]any
			err := codec.Unmarshal([]byte(src), &into)
			require.Error(t, err)
			assert.ErrorIs(t, err, yamlerrors.ErrUnhashableKey)
			assert.Contains(t, err.Error(), "as a map key: Go cannot hash it")

			// UseStringKeys does not make one usable either -- it reads keys as
			// text, and a collection has no text.
			into = nil
			err = codec.UnmarshalWithOptions([]byte(src), &into, codec.UseStringKeys())
			assert.Error(t, err)
		})
	}
}

// TestUseStringKeysReadsEveryKeyAsText records what the option changes and what
// it leaves alone.
func TestUseStringKeysReadsEveryKeyAsText(t *testing.T) {
	const src = "1.5: a\ntrue: b\n"

	t.Run("a map[any]any keeps the resolved type by default", func(t *testing.T) {
		var into map[any]any
		require.NoError(t, codec.Unmarshal([]byte(src), &into))
		assert.Equal(t, map[any]any{1.5: "a", true: "b"}, into)
	})

	t.Run("and takes strings with the option", func(t *testing.T) {
		var into map[any]any
		require.NoError(t, codec.UnmarshalWithOptions([]byte(src), &into, codec.UseStringKeys()))
		assert.Equal(t, map[any]any{"1.5": "a", "true": "b"}, into)
	})

	t.Run("which is the spelling a map[string]any already gets", func(t *testing.T) {
		var into map[string]any
		require.NoError(t, codec.Unmarshal([]byte(src), &into))
		assert.Equal(t, map[string]any{"1.5": "a", "true": "b"}, into)
	})

	t.Run("a named key type is untouched, with or without it", func(t *testing.T) {
		for _, opts := range [][]codec.DecodeOption{nil, {codec.UseStringKeys()}} {
			var into map[float64]any
			require.NoError(t, codec.UnmarshalWithOptions([]byte("1.5: a\n"), &into, opts...))
			assert.Equal(t, map[float64]any{1.5: "a"}, into)
		}
	})

	t.Run("and a key the named type cannot take is still an error", func(t *testing.T) {
		var into map[float64]any
		err := codec.UnmarshalWithOptions([]byte("true: b\n"), &into, codec.UseStringKeys())
		require.Error(t, err)
		assert.True(t, errors.Is(err, yamlerrors.ErrSyntax) || errors.Is(err, yamlerrors.ErrTypeMismatch),
			"unexpected kind: %v", err)
	})
}
