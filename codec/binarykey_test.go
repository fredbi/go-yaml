// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
)

// TestABinaryKeyIsAKindOfItsOwn covers a "!!binary" mapping key.
//
// YAML tells binary data from a string, so "!!binary AA==" and the string
// "AA==" are two keys, as "1" and "1.0" are. Two "!!binary" keys are one when
// their canonical base64 texts agree: the characters without the spaces and
// line breaks RFC 2045 lets a writer put in.
func TestABinaryKeyIsAKindOfItsOwn(t *testing.T) {
	t.Parallel()

	t.Run("beside a string that spells it, it is another key", func(t *testing.T) {
		t.Parallel()

		for _, src := range []string{
			"!!binary AA==: a\n\"AA==\": b\n",
			"!!binary AA==: a\n!!str AA==: b\n",
			"!!binary AA==: a\nAA==: b\n",
		} {
			var asAny any
			require.NoErrorf(t, codec.Unmarshal([]byte(src), &asAny), "%q", src)
			assert.Equalf(t, map[any]any{codec.Base64("AA=="): "a", "AA==": "b"}, asAny, "%q", src)

			// JSON has no binary type, so both keys write the member "AA==".
			_, err := codec.ToJSON([]byte(src))
			require.ErrorIsf(t, err, yamlerrors.ErrNotJSON, "ToJSON: %q", src)
		}
	})

	t.Run("two spellings of one base64 text are one key", func(t *testing.T) {
		t.Parallel()

		for _, src := range []string{
			"!!binary AA==: a\n!!binary AA==: b\n",
			"!!binary AA==: a\n? !!binary |\n  AA\n  ==\n: b\n",
		} {
			var asAny any
			require.ErrorIsf(t, codec.Unmarshal([]byte(src), &asAny), yamlerrors.ErrDuplicateKey, "%q", src)
		}
	})
}
