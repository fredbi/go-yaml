// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/parser"
)

// TestParseBlockScalarAtTheDocumentRoot covers a block scalar that is the whole document.
//
// Nothing encloses it, so its content has no level to be indented past and may start at column 1,
// as in the specification's bare-documents example.
// The test catches a scanner that holds such a header to the level of one written under a key,
// and rejects its content for not being indented past a level that is not there.
func TestParseBlockScalarAtTheDocumentRoot(t *testing.T) {
	valid := map[string]string{
		"literal with content at column 1":  "|\n1\n",
		"folded with content at column 1":   ">\na\n",
		"literal indented anyway":           "|\n  a\n",
		"a stated indent counts from there": "|2-\n\n  text\n",
	}

	for name, source := range valid {
		t.Run(name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(source), parser.WithComments())
			assert.NoErrorf(t, err, "rejected %q", source)
		})
	}

	// Under a key there is a level, and content at column 1 is outside it.
	t.Run("not under a mapping key", func(t *testing.T) {
		_, err := parser.ParseBytes([]byte("a: |\nb\n"), parser.WithComments())
		assert.Error(t, err)
	})
}

// TestParseEmptyDocumentsKeepTheirStream covers a document with nothing in it between two others.
//
// An empty document is a document like any other, and so is everything after it.
// The test catches a parse that ends at one "---" straight after another and drops the rest of the stream,
// so that "a: 1\n---\n---\nb: 2\n" comes back as two documents.
func TestParseEmptyDocumentsKeepTheirStream(t *testing.T) {
	tests := map[string]struct {
		source string
		docs   int
	}{
		"an empty document in the middle":  {"a: 1\n---\n---\nb: 2\n", 3},
		"a blank line between the markers": {"a: 1\n---\n\n---\nb: 2\n", 3},
		"a comment between the markers":    {"a: 1\n---\n# c\n---\nb: 2\n", 3},
		"an empty document first":          {"---\n---\nb: 2\n", 2},
		"two in a row":                     {"---\n---\n---\nc: 3\n", 3},
		"an empty document last":           {"a: 1\n---\n", 2},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Without WithComments the comment is not a token,
			// so the case with a comment between the markers becomes two markers with nothing between them.
			for _, mode := range []struct {
				name string
				opts []parser.Option
			}{{"plain", nil}, {"comments", []parser.Option{parser.WithComments()}}} {
				file, err := parser.ParseBytes([]byte(test.source), mode.opts...)
				require.NoErrorf(t, err, "mode %s", mode.name)
				assert.Lenf(t, file.Docs, test.docs, "mode %s: %q", mode.name, test.source)
			}
		})
	}
}

// TestParseDocumentsAfterASuffix covers what may follow the "..." that ends a document.
//
// Only a comment may follow the "..." on its own line. The next line starts a new document, which may be a bare one.
// The test catches a scalar on that line rejected as content of the document that has just ended.
func TestParseDocumentsAfterASuffix(t *testing.T) {
	valid := map[string]string{
		"a scalar on the next line":       "a\n...\nb\n",
		"a block scalar on the next line": "a\n...\n|\nb\n",
		"two suffixes in a row":           "a\n...\n# note\n...\n|\nb\n",
		"a mapping on the next line":      "a\n...\nb: 1\n",
	}

	for name, source := range valid {
		t.Run(name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(source), parser.WithComments())
			require.NoErrorf(t, err, "rejected %q", source)
			assert.GreaterOrEqual(t, len(file.Docs), 2, "%q is more than one document", source)
		})
	}

	// On the "..." line itself there is no room for a second document.
	invalid := map[string]string{
		"a scalar on the same line":  "a\n... b\n",
		"a mapping on the same line": "a\n... b: 1\n",
	}

	for name, source := range invalid {
		t.Run(name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(source), parser.WithComments())
			assert.Errorf(t, err, "accepted %q", source)
		})
	}
}

// TestParseExplicitKeyComments covers a comment written on the "?" line.
//
// The comment belongs to the key. The group holding an explicit key ends on the key and not on a ':'.
// The test catches the comment carried over to the value as though written after a ':',
// which renders it on both lines and adds a comment each time the document is read and written.
func TestParseExplicitKeyComments(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"on a key with no value": {
			source: "? a # note\n",
			want:   "? a # note\n:\n",
		},
		"on a key with a value": {
			source: "? a # note\n: b\n",
			want:   "? a # note\n: b\n",
		},
		"on the value instead": {
			source: "? a\n: b # note\n",
			want:   "? a\n: b # note\n",
		},
		"on both": {
			source: "? a # key\n: b # value\n",
			want:   "? a # key\n: b # value\n",
		},
		"on a block scalar key": {
			source: "? |\n  a\n: b # note\n",
			want:   "? |\n  a\n: b # note\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(test.source), parser.WithComments())
			require.NoError(t, err)
			assert.Equal(t, test.want, file.String())

			// It settles: a second parse and render adds nothing.
			reread, err := parser.ParseBytes([]byte(test.want), parser.WithComments())
			require.NoErrorf(t, err, "cannot read back %q", test.want)
			assert.Equal(t, test.want, reread.String())
		})
	}
}

// TestAnExplicitEntryKeepsBothComments holds a comment on the ":" line and a head comment above the "?" in the tree.
//
// The ":" line comment goes to the entry's own LineComment.
// On the value it would collide with a head comment written under the ":",
// and on BaseNode.Comment with a head comment written above the "?".
//
// The test asserts the tree and not the rendered text.
func TestAnExplicitEntryKeepsBothComments(t *testing.T) {
	for name, tc := range map[string]struct {
		source string
		head   string
		line   string
	}{
		"a head comment above the '?' and one on the ':' line": {
			source: "# h\n? k\n: # c\n",
			head:   "# h",
			line:   "# c",
		},
		"both written empty": {
			source: "#\n?\n: #c4\n",
			head:   "#",
			line:   "#c4",
		},
		"the ':' line alone, so the head slot stays empty": {
			source: "? k\n: # c\n",
			head:   "", // LineComment alone holds the ':' line comment.
			line:   "# c",
		},
	} {
		t.Run(name, func(t *testing.T) {
			entry := firstMappingEntry(t, tc.source)

			require.NotNil(t, entry.LineComment, "the ':' line comment is not in the tree")
			assert.Equal(t, tc.line, entry.LineComment.String())

			if tc.head == "" {
				assert.Nil(t, entry.Comment,
					"the ':' line comment stands in LineComment alone, not in both slots")

				return
			}

			require.NotNil(t, entry.Comment, "the head comment is not in the tree")
			assert.Equal(t, tc.head, entry.Comment.String())
		})
	}

	// An entry written the short way puts its line comment on the value node, whose own slot is free.
	// It is held so that a change to the long form cannot move the short one.
	t.Run("the short spelling is untouched", func(t *testing.T) {
		entry := firstMappingEntry(t, "# h\na: v # c\n")

		require.NotNil(t, entry.Comment)
		assert.Equal(t, "# h", entry.Comment.String())
		assert.Nil(t, entry.LineComment, "a short entry writes no ':' line of its own")

		require.NotNil(t, entry.Value.GetComment())
		assert.Equal(t, "# c", entry.Value.GetComment().String())
	})
}

// firstMappingEntry parses src with comments and returns the first entry of the mapping at its root.
func firstMappingEntry(t *testing.T, src string) *ast.MappingValueNode {
	t.Helper()

	f, err := parser.ParseBytes([]byte(src), parser.WithComments())
	require.NoErrorf(t, err, "%q", src)
	require.Lenf(t, f.Docs, 1, "%q", src)

	switch body := f.Docs[0].Body.(type) {
	case *ast.MappingValueNode:
		return body
	case *ast.MappingNode:
		require.NotEmptyf(t, body.Values, "%q", src)

		return body.Values[0]
	default:
		t.Fatalf("%q: the document is a %T, not a mapping", src, body)

		return nil
	}
}

// TestACommentAfterTheExplicitKeyIndicatorIsKept holds a comment closing a "?" line through a parse and a render.
//
// stageLineComments records such a comment against the bare "?" token, before anything is grouped.
// By the time parseMapKey reaches the key, the "?" has been wrapped twice,
// once with the key's body and once with the entry's ":", so the token in hand is the outer wrapper.
// A group reports the type it opens with, so only pointer identity tells the wrapper from the "?".
//
// The comment goes on the key's node, where the renderer writes it, as for "? a # note".
// So "? # c" over "  k" comes back as "? k # c",
// and a key that cannot share the line of its "?" keeps the comment where it was written.
//
// Section 8.2.2 gives c-l-block-map-explicit-key(n) the shape "?" s-l+block-indented(n,block-out),
// and s-l-comments sits inside it, so the document may write the comment there.
func TestACommentAfterTheExplicitKeyIndicatorIsKept(t *testing.T) {
	for name, tc := range map[string]struct{ source, renders string }{
		"a scalar key, so the comment moves onto its line": {
			source:  "? # c\n  k\n: v\n",
			renders: "? k # c\n: v\n",
		},
		"a sequence key, which cannot share the '?'s line": {
			source:  "? # c\n  - a\n: v\n",
			renders: "? # c\n  - a\n: v\n",
		},
		"a mapping key written the long way": {
			source:  "? #\n  ? \"\"\n  :\n:\n",
			renders: "? #\n  ? \"\"\n  :\n:\n",
		},
		// Neighboring shapes, held so that a fix cannot move them.
		"the comment closing the key's own line": {
			source:  "? k # c\n: v\n",
			renders: "? k # c\n: v\n",
		},
		"with a value written after it": {
			source:  "? a # note\n: b\n",
			renders: "? a # note\n: b\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			f, err := parser.ParseBytes([]byte(tc.source), parser.WithComments())
			require.NoErrorf(t, err, "%q", tc.source)

			once := f.String()
			assert.Equal(t, tc.renders, once)

			// A second parse and render leaves the text unchanged.
			g, err := parser.ParseBytes([]byte(once), parser.WithComments())
			require.NoErrorf(t, err, "%q", once)
			assert.Equalf(t, once, g.String(), "%q renders to %q and then moves", tc.source, once)
		})
	}

	// A YAML Test Suite document with comments on the "?" line, the ":" line and a "-" line keeps all three.
	t.Run("a suite document stops losing one", func(t *testing.T) {
		const src = "? # lala\n - seq1\n: # lala\n - #lala\n  seq2\n"

		f, err := parser.ParseBytes([]byte(src), parser.WithComments())
		require.NoError(t, err)
		assert.Equal(t, 3, strings.Count(f.String(), "#"), "%q", f.String())
	})
}

// TestFixedAMarkerKeepsTheCommentClosingItsLine renders a comment on a "---" or "..." line back on that line.
//
// A marker is not a node, so no node takes its comment: ast.DocumentNode.StartComment and EndComment hold them.
// One document may carry both, so it has two slots and does not use the inherited BaseNode.Comment.
// A comment written above a "---" is different: it introduces the document and reaches its body.
func TestFixedAMarkerKeepsTheCommentClosingItsLine(t *testing.T) {
	const bom = "\ufeff"

	for _, tc := range []struct{ src, want string }{
		{src: "--- # c1\n", want: "--- # c1\n"},
		{src: "--- # c1\nk: v\n", want: "--- # c1\nk: v\n"},
		{src: "---\nk: v\n... # c2\n", want: "---\nk: v\n... # c2\n"},
		// Both markers of one document, which is why there are two slots.
		{src: "--- # c1\nk: v\n... # c2\n", want: "--- # c1\nk: v\n... # c2\n"},
		// Each document of a stream keeps its own.
		{src: "--- # c1\n--- # c2\n", want: "--- # c1\n--- # c2\n"},
		{
			src:  "%YAML 1.2\n---\nDocument\n... # Suffix\n",
			want: "%YAML 1.2\n---\nDocument\n... # Suffix\n",
		},
		// A comment above the marker introduces the document and is not this.
		{src: "# c1\n---\nk: v\n", want: "# c1\n---\nk: v\n"},
		// The line ends how it likes, and a byte order mark stands outside it.
		{src: bom + "--- # c1\r", want: "--- # c1\n"},
		{src: bom + "---\t# c1\r", want: "--- # c1\n"},
	} {
		f, err := parser.ParseBytes([]byte(tc.src), parser.WithComments())
		require.NoErrorf(t, err, "%q", tc.src)
		assert.Equalf(t, tc.want, f.String(), "%q", tc.src)
	}

	t.Run("and without WithComments nothing is kept", func(t *testing.T) {
		f, err := parser.ParseBytes([]byte("--- # c1\nk: v\n"))
		require.NoError(t, err)
		assert.Equal(t, "---\nk: v\n", f.String())
	})
}

// TestFixedARootNodeKeepsBothItsComments keeps a head comment and a line comment on the document body.
//
// A mapping entry and a sequence entry each have two comment fields, so both comments survive there.
// A node with no entry around it has one, ast.BaseNode.Comment, and SetComment assigns it,
// so the test catches "# c1" over "831 # c2" losing one of the two.
//
// ast.BaseNode.HeadComment holds the comment above a node for every node type, and Renderer.withHeadComment writes it.
// attachComment writes HeadComment where Comment is taken or renders beside the node.
// On a mapping, a sequence and a block under a key, Comment renders above the node and keeps the head comment.
//
// TestNoCommentIsReadAndThenDropped counts overwritten head comments across the corpus.
func TestFixedARootNodeKeepsBothItsComments(t *testing.T) {
	for _, src := range []string{
		"# c1\n831 # c2\n",
		"---\n# c1\n831 # c2\n",
		"# c1\n~ # c2\n",
		"# c1\n[1, 2] # c2\n",
		"# c1\n{a: 1} # c2\n",
		// A head comment is a run of lines, and stays one.
		"# head line 1\n# head line 2\nvalue # property comment\n",
		"# head line 1\n# head line 2\n[1, 2] # property comment\n",
	} {
		f, err := parser.ParseBytes([]byte(src), parser.WithComments())
		require.NoErrorf(t, err, "%q", src)
		assert.Equalf(t, src, f.String(), "%q should render as it was written", src)

		again, err := parser.ParseBytes([]byte(f.String()), parser.WithComments())
		require.NoErrorf(t, err, "%q", src)
		assert.Equalf(t, f.String(), again.String(), "%q should settle", src)
	}

	t.Run("and an entry was always fine", func(t *testing.T) {
		for _, src := range []string{
			"# c1\na: 1 # c2\n",
			"# c1\n- x # c2\n",
			"k:\n  # h1\n  # h2\n  value: ~ # p\n  ## t1\n  ## t2\n",
		} {
			f, err := parser.ParseBytes([]byte(src), parser.WithComments())
			require.NoErrorf(t, err, "%q", src)
			assert.Equalf(t, src, f.String(), "%q", src)
		}
	})
}

// TestFixedAnIndentedCommentBetweenAKeyAndItsColonIsKept keeps a comment between an explicit key and its ":".
//
// endsExplicitKeyBody ends the body at a token whose column is not past the "?".
// The test catches a comment written further in taken for part of the body,
// grouped with the key and never reaching a node, as in "? key" over "  # comment" over ": value".
//
// A comment is not a node, so it is held aside and handed on after the key.
// Where more of the body follows it, the comment goes back where it was written,
// so a comment inside a block collection under the key keeps its place.
func TestFixedAnIndentedCommentBetweenAKeyAndItsColonIsKept(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		// The comment stands between the key and the value, so it is written
		// above the value and the value goes under the ":".
		{src: "? key\n# comment\n: value\n", want: "? key\n:\n  # comment\n  value\n"},
		{src: "? key\n # comment\n: value\n", want: "? key\n:\n  # comment\n  value\n"},
		{src: "? key\n  # comment\n: value\n", want: "? key\n:\n  # comment\n  value\n"},
		{src: "? key\n    # comment\n: value\n", want: "? key\n:\n  # comment\n  value\n"},
		// A multi-line plain key, which the comment ends: see the scanner's TestFixedAPlainScalarEndsAtAComment.
		{src: "?\n  a\n      - b\n# c\n: v\n", want: "? a - b\n:\n  # c\n  v\n"},
		{src: "?\n  a\n      - b\n  # c\n: v\n", want: "? a - b\n:\n  # c\n  v\n"},
		{src: "?\n  a\n      - b\n      # c\n: v\n", want: "? a - b\n:\n  # c\n  v\n"},
	} {
		f, err := parser.ParseBytes([]byte(tc.src), parser.WithComments())
		require.NoErrorf(t, err, "%q", tc.src)
		assert.Equalf(t, tc.want, f.String(), "%q", tc.src)
	}

	t.Run("and a comment inside the body stays where it was written", func(t *testing.T) {
		// More of the body follows, so the comment belongs to the sequence and
		// not to the key.
		const src = "?\n  - a\n  # c\n  - b\n: v\n"

		f, err := parser.ParseBytes([]byte(src), parser.WithComments())
		require.NoError(t, err)
		assert.Contains(t, f.String(), "# c", "%q keeps its comment", src)
	})
}
