// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
)

// TestATabSeparatesAPropertyFromItsNode applies the rule of TestATabIsSeparationAndNotIndentation
// to the separation between a node property and its node.
//
// s-separate-in-line is s-white+, and s-white is a space or a tab,
// so a tab ends an anchor name, an alias name or a tag exactly as a space does.
// Scanner.endsProperty decides where a property ends, for both characters.
func TestATabSeparatesAPropertyFromItsNode(t *testing.T) {
	t.Run("after an anchor", func(t *testing.T) {
		for _, tc := range []struct {
			src  string
			want any
		}{
			{"a: &x\ty\n", map[string]any{"a": "y"}},
			{"a: &x\t\ty\n", map[string]any{"a": "y"}},
			{"a: &x\t[1]\n", map[string]any{"a": []any{uint64(1)}}},
			{"- &x\ty\n", []any{"y"}},

			// The space spelling, as a control.
			{"a: &x y\n", map[string]any{"a": "y"}},
		} {
			var got any
			require.NoErrorf(t, codec.Unmarshal([]byte(tc.src), &got), "%q", tc.src)
			assert.Equalf(t, tc.want, got, "%q", tc.src)
		}
	})

	t.Run("and the anchor is still named, so an alias finds it", func(t *testing.T) {
		// The tab ends the anchor name, so "*x" finds the anchor "x".
		var got any
		require.NoError(t, codec.Unmarshal([]byte("a: &x\ty\nb: *x\n"), &got))
		assert.Equal(t, map[string]any{"a": "y", "b": "y"}, got)
	})

	t.Run("after a tag", func(t *testing.T) {
		for _, tc := range []struct {
			src  string
			want any
		}{
			{"a: !!str\tx\n", map[string]any{"a": "x"}},
			{"a: !!int\t7\n", map[string]any{"a": uint64(7)}},
			{"a: !foo\tx\n", map[string]any{"a": "x"}},
			{"a: !\tx\n", map[string]any{"a": "x"}},
			{"a: !!str\t\tx\n", map[string]any{"a": "x"}},
			{"- !!str\tx\n", []any{"x"}},
			{"{a: !!str\tx}\n", map[string]any{"a": "x"}},

			{"a: !!str x\n", map[string]any{"a": "x"}},
		} {
			var got any
			require.NoErrorf(t, codec.Unmarshal([]byte(tc.src), &got), "%q", tc.src)
			assert.Equalf(t, tc.want, got, "%q", tc.src)
		}
	})

	t.Run("and the entry after it is still read", func(t *testing.T) {
		var got any
		require.NoError(t, codec.Unmarshal([]byte("a: !!str\ty\nb: 2\n"), &got))
		assert.Equal(t, map[string]any{"a": "y", "b": uint64(2)}, got)
	})

	t.Run("at the root of a document, where no delimiter stands before it", func(t *testing.T) {
		// At the root lastDelimColumn is 0, and the anchor's name is still in the buffer when the tab arrives.
		// The scan must end the property before either indentation test in the tab branch reads on,
		// or "&a1\t>-" cuts the anchor "a1>-" and never opens the block scalar.
		for _, src := range []string{
			"&a1\t>-\n , a\n",
			"&a1\t|-\n , a\n",
			"&a1\t>-\n   , a\n",
			"&a1\t>-\n x\n",

			// Three controls: a space, a tag, and no property at all.
			"&a1 >-\n , a\n",
			"!!str\t>-\n , a\n",
			">-\n , a\n",
		} {
			var got any
			require.NoErrorf(t, codec.Unmarshal([]byte(src), &got), "%q", src)
			assert.NotNilf(t, got, "%q", src)
		}
	})

	t.Run("an alias naming nothing is still refused", func(t *testing.T) {
		// The tab ends the alias name, so the alias names "x", which no anchor declares.
		var got any
		err := codec.Unmarshal([]byte("a: *x\ty\n"), &got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `could not find alias "x"`)
	})
}
