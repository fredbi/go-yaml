// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
	"github.com/go-openapi/go-yaml/parser"
)

// fragment keeps the bytes a bytes unmarshaler is handed, and what a fresh
// Unmarshal reads them as.
type fragment struct {
	text  string
	value any
}

func (f *fragment) UnmarshalYAML(b []byte) error {
	f.text = string(b)

	return codec.Unmarshal(b, &f.value)
}

// TestABytesUnmarshalerReadsItsFragmentUnderTheDocumentsVersion covers the
// fragment handed to an UnmarshalYAML([]byte).
//
// The fragment is the node rendered again, and a nested Unmarshal parses it on
// its own, so the version the document was read under did not travel with it:
// under "%YAML 1.1", "<<" merges and "yes" is true, and the fragment read both
// as 1.2 does. It now opens with "%YAML 1.1" wherever the document was read
// under 1.1, by its own directive or by parser.WithYAMLVersion.
func TestABytesUnmarshalerReadsItsFragmentUnderTheDocumentsVersion(t *testing.T) {
	const body = "b: &b {x: 1}\nf:\n  <<: *b\n  on: yes\n"
	// 1.1 reads "on" and "yes" as true, so the key is a boolean.
	merged := map[any]any{"x": uint64(1), true: true}

	t.Run("under the document's own directive", func(t *testing.T) {
		var v struct {
			F fragment `yaml:"f"`
		}
		require.NoError(t, codec.Unmarshal([]byte("%YAML 1.1\n---\n"+body), &v))
		assert.Contains(t, v.F.text, "%YAML 1.1\n---\n")
		assert.Equal(t, merged, v.F.value)
	})

	t.Run("under parser.WithYAMLVersion", func(t *testing.T) {
		var v struct {
			F fragment `yaml:"f"`
		}
		require.NoError(t, codec.UnmarshalWithOptions([]byte(body), &v,
			codec.WithParserOptions(parser.WithYAMLVersion(parser.YAML11))))
		assert.Contains(t, v.F.text, "%YAML 1.1\n---\n")
		assert.Equal(t, merged, v.F.value)
	})

	t.Run("and not under 1.2, where nothing merges", func(t *testing.T) {
		var v struct {
			F fragment `yaml:"f"`
		}
		require.NoError(t, codec.Unmarshal([]byte(body), &v))
		assert.NotContains(t, v.F.text, "%YAML")
		assert.Equal(t, map[string]any{"<<": map[string]any{"x": uint64(1)}, "on": "yes"}, v.F.value)
	})

	t.Run("the directive scopes one document of a stream", func(t *testing.T) {
		dec := codec.NewDecoder(strings.NewReader("%YAML 1.1\n---\nf: {on: yes}\n---\nf: {on: yes}\n"))

		var first, second struct {
			F fragment `yaml:"f"`
		}
		require.NoError(t, dec.Decode(&first))
		require.NoError(t, dec.Decode(&second))
		assert.Equal(t, map[any]any{true: true}, first.F.value, "the first document is 1.1")
		assert.Equal(t, map[string]any{"on": "yes"}, second.F.value, "the second is back to 1.2")
	})
}
