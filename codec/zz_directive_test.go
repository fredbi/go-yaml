// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	yaml "github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/codec"
)

// A directive whose name begins with "&" or "*" is read as a node property.
//
// 6.8 makes a directive a "%", a name of ns-chars, and its parameters, and says
// an unknown one is ignored with a warning. "&" and "*" are ns-chars, so
// "%&AML 1.2" is a well-formed directive named "&AML" -- and this library reads
// the "&AML" as an anchor and the "1.2" as the node it names.
//
// ToJSON then writes that node as the whole document: "%&AML 1.2" over "---"
// over "k: v" converts to 1.2, the mapping gone. With "*" both paths refuse.
//
// libfyaml 1.0.0b1 reads the document in every case and the reference parser
// passes them. Found on 2026-09-07 by a mutation of the "%YAML" line
// Style.Version writes -- a "Y" turned into a "&".

// TestDefectADirectiveNamedLikeAPropertyIsReadAsOne pins both halves.
func TestDefectADirectiveNamedLikeAPropertyIsReadAsOne(t *testing.T) {
	t.Run("an anchor name: ToJSON writes the parameter as the document", func(t *testing.T) {
		for _, tc := range []struct{ src, writes string }{
			{src: "%&AML 1.2\n---\n-8.5\n", writes: "1.2"},
			{src: "%&AML 1.2\n---\nk: v\n", writes: "1.2"},
			{src: "%&x y\n---\n-8.5\n", writes: `"y"`},
			// No parameter, so the anchor names the empty node.
			{src: "%&AML\n---\n-8.5\n", writes: "null"},
		} {
			out, err := codec.ToJSON([]byte(tc.src))
			require.NoErrorf(t, err, "%q", tc.src)
			assert.Equal(t, tc.writes, string(out), "today: %q converts to its directive", tc.src)

			// The decoder reads the document, which is what makes this the
			// converter's defect rather than the parser's.
			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(tc.src), &got), "%q", tc.src)
			assert.NotEqual(t, tc.writes, got, "%q", tc.src)
		}
	})

	t.Run("an alias name: both paths refuse", func(t *testing.T) {
		const src = "%*x y\n---\n-8.5\n"

		var got any
		err := yaml.Unmarshal([]byte(src), &got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `could not find alias "x"`)

		_, jerr := codec.ToJSON([]byte(src))
		assert.Error(t, jerr)
	})

	t.Run("any other name is a directive and is ignored", func(t *testing.T) {
		for _, src := range []string{
			"%FOO 1.2\n---\n-8.5\n",
			"%!x y\n---\n-8.5\n",
			// The indicator has to lead: "A&ML" is an ordinary name.
			"%A&ML 1.2\n---\n-8.5\n",
		} {
			out, err := codec.ToJSON([]byte(src))
			require.NoErrorf(t, err, "%q", src)
			assert.Equal(t, "-8.5", string(out), "%q", src)
		}
	})
}
