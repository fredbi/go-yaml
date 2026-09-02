// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"runtime"
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
	"github.com/go-openapi/go-yaml/parser"
	"github.com/go-openapi/go-yaml/token"
)

// TestStressShape reports what each stress document asks of a token store.
//
// A store that reclaims behind the parse is sized by three things, and none of
// them is the length of the document: the widest level, because a level's
// entries are gathered before the node above them is built; the depth, because
// every open level holds a token for its column; and the anchors, because an
// anchored subtree outlives the parse that read it.
//
// The five ordinary workloads answer all three with small numbers -- no
// anchors at all, and a widest level of 1,789 against 293,142 tokens. These
// documents are here to answer them with large ones. Run with -v for the table.
func TestStressShape(t *testing.T) {
	all, err := workloads.Stress()
	require.NoError(t, err)

	// Read with ParseComments throughout: comments_dense is about comments, and
	// a mode that drops them at the door measures a different document.
	t.Logf("%-18s %8s %8s %7s %10s %10s %6s %8s %8s %9s %7s",
		"document", "bytes", "tokens", "B/token", "widest map", "widest seq",
		"depth", "anchors", "comments", "peak", "x source")

	for _, w := range all {
		tokens := tokenize(t, string(w.Data))

		var comments int
		for _, tk := range tokens {
			if tk.Type == token.CommentType {
				comments++
			}
		}

		file, err := parser.ParseBytes(w.Data, parser.ParseComments)
		require.NoError(t, err)

		var s stressShape
		depth := 0
		for _, doc := range file.Docs {
			ast.Walk(&s, doc)
			if d := nodeDepth(doc, 0); d > depth {
				depth = d
			}
		}

		peak := peakLiveMax(peakLiveRuns(), func() {
			f, err := parser.ParseBytes(w.Data, parser.ParseComments)
			require.NoError(t, err)
			runtime.KeepAlive(f)
		})

		t.Logf("%-18s %8d %8d %7.1f %10d %10d %6d %8d %8d %8dK %6.0fx",
			w.Name, len(w.Data), len(tokens),
			float64(len(w.Data))/float64(len(tokens)),
			s.widestMap, s.widestSeq, depth, s.anchors, comments, peak/1024,
			float64(peak)/float64(len(w.Data)))
	}
}

// stressShape counts what sizes a store: how wide a level gets, and how many
// anchors have to outlive the parse.
type stressShape struct {
	widestMap int
	widestSeq int
	anchors   int
}

func (s *stressShape) Visit(n ast.Node) ast.Visitor {
	switch t := n.(type) {
	case *ast.MappingNode:
		s.widestMap = max(s.widestMap, len(t.Values))
	case *ast.SequenceNode:
		s.widestSeq = max(s.widestSeq, len(t.Values))
	case *ast.AnchorNode:
		s.anchors++
	}

	return s
}

// nodeDepth is how far the tree nests below n.
func nodeDepth(n ast.Node, at int) int {
	deepest := at

	switch t := n.(type) {
	case *ast.DocumentNode:
		if t.Body != nil {
			deepest = nodeDepth(t.Body, at+1)
		}
	case *ast.MappingNode:
		for _, value := range t.Values {
			deepest = max(deepest, nodeDepth(value, at+1))
		}
	case *ast.MappingValueNode:
		if t.Value != nil {
			deepest = nodeDepth(t.Value, at+1)
		}
	case *ast.SequenceNode:
		for _, value := range t.Values {
			deepest = max(deepest, nodeDepth(value, at+1))
		}
	case *ast.AnchorNode:
		if t.Value != nil {
			deepest = nodeDepth(t.Value, at+1)
		}
	}

	return deepest
}
