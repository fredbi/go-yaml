// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package refparser

import (
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser/scanner"
)

// TestLineCommentIndexEmptiesAsCommentsAreAttached checks that a parse hands
// every line comment over and keeps none.
//
// The index is keyed by token, so an entry left in it holds that token and the
// comment closing its line for as long as the parse runs. A reader that peeks
// with lineComment where it should take with takeLineComment leaves one behind,
// and nothing else would notice: the tree comes out the same either way.
func TestLineCommentIndexEmptiesAsCommentsAreAttached(t *testing.T) {
	const src = `# a heading over the document
top: 1 # closes the line
nested:
  # a heading over the entry
  inner: two # closes this line too
  list:
  # a heading over an entry
  - one # closes an entry's line
  - two
  # closes the block
flow: {a: 1, b: 2} # closes a flow mapping
seq: [1, 2] # closes a flow sequence
last: done # the last one
`

	var s scanner.Scanner
	s.Init(src)

	p, err := New(s.Tokens(), ParseComments)
	require.NoError(t, err)
	require.NoError(t, s.Err())
	require.NotEmpty(t, p.lineComments, "the document holds line comments to hand over")

	handed := len(p.lineComments)

	file, err := p.Parse()
	require.NoError(t, err)
	require.Empty(t, p.lineComments,
		"%d of %d line comments were peeked at rather than taken", len(p.lineComments), handed)

	// Every comment reaches the tree, so the document renders back as itself.
	require.Equal(t, src, file.String())
}
