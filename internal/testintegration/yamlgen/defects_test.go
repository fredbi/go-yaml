// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/testintegration/grammar"
	"github.com/go-openapi/go-yaml/parser"
)

// Shapes the generator found that still diverge.
//
// Each one pins today's behavior rather than the correct behavior, so that a
// fix breaks the test that says it was broken. The corresponding entry in
// [yamlgen.Ledger] is what keeps the property tests from failing on it
// meanwhile; when both go, the case moves to fixed_test.go.
//
// The first two below came from [yamlgen.Style.Break] on 2026-09-03, the axis that
// writes one document with LF, CRLF and a lone CR. Neither shape is exotic and
// neither was reachable before it. The third came from [yamlgen.DeepDocument]
// the same day, and is the one no verdict could have found: the documents parse
// correctly and cost quadratic time doing it. The rest came from [Tagged]
// and [yamlgen.Style.PropertyOrder] on the same day again.

// wellFormed asserts src is a YAML 1.2 document before anything is asked of the
// library, so that a case here is a claim about the library and not about a
// document nobody has to read.
func wellFormed(t *testing.T, src string) {
	t.Helper()

	require.True(t, grammar.NewRecognizer(1024).Stream([]byte(src)).OK,
		"the document is not YAML 1.2, so there is nothing to hold the library to")
}

// renderOnce reads a document with comments kept and writes it back.
func renderOnce(t *testing.T, src string) string {
	t.Helper()

	file, err := parser.ParseBytes([]byte(src), parser.WithComments())
	require.NoError(t, err)

	return file.String()
}

// TestDefectTagBeforeAnchorIsDropped: YAML 1.2 lets a node's tag and anchor
// appear in either order. Written second the tag holds; written first it is
// dropped from the node the anchor names.
//
// Two shapes left, and the first is the reason this is worth more than a
// curiosity: a document loses entries and nobody is told. The third -- a
// collection tag stopping the parse outright -- was fixed on 2026-09-07 and
// moved to TestFixedACollectionTagBeforeAnAnchorParses.
func TestDefectTagBeforeAnchorIsDropped(t *testing.T) {
	t.Run("on an empty node it swallows what follows, and says so", func(t *testing.T) {
		// The swallowing is the parse's and has not been fixed: the tag still
		// takes the entry below it. What has changed is that the document no
		// longer comes back short with a nil error. ast.TagNode.Resolve reports
		// a scalar tag standing on a collection, which is what the swallowed
		// entry turns the node into, so the load refuses.
		for _, src := range []string{
			"- !!null &a1\n- x\n",
			"- !!str &a1\n- x\n",
			"k: !!null &a1\nj: x\n",
		} {
			wellFormed(t, src)

			var got any
			err := yaml.Unmarshal([]byte(src), &got)
			require.Errorf(t, err, "%q", src)
			assert.Containsf(t, err.Error(), "names a kind this node is not", "%q", src)
		}

		// Anchor first, and all three read correctly.
		var got any
		require.NoError(t, yaml.Unmarshal([]byte("- &a1 !!null\n- x\n"), &got))
		assert.Equal(t, []any{nil, "x"}, got)
	})

	t.Run("otherwise the anchor names the untagged value", func(t *testing.T) {
		wellFormed(t, "a: !!int &a1 \"5\"\nb: *a1\n")

		var got any
		require.NoError(t, yaml.Unmarshal([]byte("a: !!int &a1 \"5\"\nb: *a1\n"), &got))
		assert.Equal(t, map[string]any{"a": int(5), "b": "5"}, got,
			"today: one node, read as a number where it stands and as a string through the alias")

		got = nil
		require.NoError(t, yaml.Unmarshal([]byte("a: &a1 !!int \"5\"\nb: *a1\n"), &got))
		assert.Equal(t, map[string]any{"a": int(5), "b": int(5)}, got)

		// "!!str" is the one tag that survives the alias: the decoder replaces
		// what the anchor recorded with the string, since that is the value
		// the tag names. See TestFixedStrTagKeepsTheSpelling.
		got = nil
		require.NoError(t, yaml.Unmarshal([]byte("a: !!str &a1 5\nb: *a1\n"), &got))
		assert.Equal(t, map[string]any{"a": "5", "b": "5"}, got)
	})
}

