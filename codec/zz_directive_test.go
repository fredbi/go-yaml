// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"fmt"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
)

// The walking reader takes a directive whose name begins with "&" or "*" for a
// node property.
//
// 6.8 makes a directive a "%", a name of ns-chars, and its parameters, and says
// an unknown one is ignored with a warning. "&" and "*" are ns-chars, so
// "%&AML 1.2" is a well-formed directive named "&AML" -- and the walk reads the
// "&AML" as an anchor and the "1.2" as the node it names, then hands that node
// back as the whole document. So "%&AML 1.2" over "---" over "k: v" walks to
// 1.2, the mapping gone, and ToJSON writes 1.2. The tree reads the mapping.
//
// With "*" the name is an alias, and both paths refuse.
//
// libfyaml 1.0.0b1 reads the document in every case and the reference parser
// passes them; go.yaml.in/yaml/v3 refuses the directive line outright, which is
// the other defensible answer. Found on 2026-09-07 by a mutation of the "%YAML"
// line Style.Version writes -- a "Y" turned into a "&".

// TestDefectADirectiveNamedLikeAPropertyIsReadAsOne pins both halves.
func TestDefectADirectiveNamedLikeAPropertyIsReadAsOne(t *testing.T) {
	t.Run("an anchor name: the walk hands back the parameter as the document", func(t *testing.T) {
		for _, tc := range []struct{ src, walks, writes, tree string }{
			{src: "%&AML 1.2\n---\n-8.5\n", walks: "1.2", writes: "1.2", tree: "-8.5"},
			{src: "%&AML 1.2\n---\nk: v\n", walks: "1.2", writes: "1.2", tree: `codec.MapSlice{codec.MapItem{Key:"k", Value:"v"}}`},
			{src: "%&x y\n---\n-8.5\n", walks: `"y"`, writes: `"y"`, tree: "-8.5"},
			// No parameter, so the anchor names the empty node.
			{src: "%&AML\n---\n-8.5\n", walks: "<nil>", writes: "null", tree: "-8.5"},
		} {
			var walked any
			require.NoErrorf(t, codec.Unmarshal([]byte(tc.src), &walked), "%q", tc.src)
			assert.Equalf(t, tc.walks, fmt.Sprintf("%#v", walked), "today: the walk reads the directive: %q", tc.src)

			out, err := codec.ToJSON([]byte(tc.src))
			require.NoErrorf(t, err, "%q", tc.src)
			assert.Equal(t, tc.writes, string(out), "today: %q converts to its directive", tc.src)

			// The tree reads the document, which is what makes this the walk's
			// defect rather than the scanner's.
			assert.Equalf(t, tc.tree, fmt.Sprintf("%#v", treeRead(t, tc.src)), "%q", tc.src)
		}
	})

	t.Run("an alias name: both paths refuse", func(t *testing.T) {
		const src = "%*x y\n---\n-8.5\n"

		var got any
		err := codec.Unmarshal([]byte(src), &got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `could not find alias "x"`)

		terr := codec.UnmarshalWithOptions([]byte(src), &got, codec.UseOrderedMap())
		require.Error(t, terr)
		assert.Contains(t, terr.Error(), `could not find alias "x"`)

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

			assert.Equalf(t, "-8.5", fmt.Sprintf("%#v", treeRead(t, src)), "%q", src)
		}
	})
}
