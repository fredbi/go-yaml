// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"github.com/go-openapi/testify/v2/require"
)

// TestLineCommentIndexEmptiesAsCommentsAreAttached checks that a parse takes every line comment out of the index.
//
// The index is keyed by token, so an entry left in it keeps that token and its comment reachable,
// and on a tape that recycles also the chunk the token sits in.
// A reader that peeks with lineComment where it should take with takeLineComment leaves an entry behind,
// and the tree comes out the same either way, so only this test catches it.
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

	p := New(WithComments())

	file, err := p.Parse([]byte(src))
	require.NoError(t, err)

	// The reader fills the index as it groups, and the parse must leave it empty.
	// The render below checks that the comments reached the tree.
	require.Empty(t, p.lineComments,
		"%d line comments were peeked at rather than taken", len(p.lineComments))

	// Every comment reaches the tree, so the document renders back as itself.
	require.Equal(t, src, file.String())
}
