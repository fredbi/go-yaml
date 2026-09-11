// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package ast_test

import (
	"bytes"
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/parser"
)

// withoutCommentsCensus records the corpus rebuilt through VerbatimFile from a
// parse without parser.WithComments: how many documents the parse accepts, and
// how many of those do not come back byte for byte.
//
// TestVerbatimRebuildsEveryDocument parses with comments, and a tree parsed
// without them differs in ways that census cannot see: it holds no comment and
// no SequenceNode.Entries. Leaving out a removed node took a "-" no entry hands
// over, a "..." no document holds and every comment for a removed node's text,
// and 27 documents no caller had touched came back wrong or were refused before
// this census was kept.
//
// The 30 recorded were there before any of that, measured on 2026-09-11. A lone
// "-" comes back as "- " among them.
var withoutCommentsCensus = struct{ tested, differing int }{tested: 12708, differing: 30}

func TestVerbatimRebuildsTheCorpusParsedWithoutComments(t *testing.T) {
	t.Parallel()

	var tested, differing int
	var first []string
	for _, src := range renderSources(t) {
		file, err := parser.ParseBytes([]byte(src.text))
		if err != nil {
			continue
		}
		tested++

		var out bytes.Buffer
		err = ast.NewRenderer(ast.WithSource([]byte(src.text))).VerbatimFile(&out, file)
		if err != nil || out.String() != src.text {
			differing++
			if len(first) < 10 {
				first = append(first, src.name)
			}
		}
	}

	t.Logf("without comments: %d of %d accepted documents do not come back byte for byte (recorded %d of %d), starting with %v",
		differing, tested, withoutCommentsCensus.differing, withoutCommentsCensus.tested, first)

	require.GreaterOrEqualf(t, tested, withoutCommentsCensus.tested,
		"%d documents are measured where %d were: fewer are accepted than were, so re-measure before reading the count below",
		tested, withoutCommentsCensus.tested)
	mustHold(t, "the count of documents that do not come back byte for byte",
		differing, withoutCommentsCensus.differing, tested, withoutCommentsCensus.tested)
}
