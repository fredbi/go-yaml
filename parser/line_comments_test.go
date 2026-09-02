// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"github.com/go-openapi/testify/v2/require"
)

// TestLineCommentIndexEmptiesAsCommentsAreAttached checks that a parse hands
// every line comment over and keeps none.
//
// The index is keyed by token, so an entry left in it holds that token and the
// comment closing its line, and on a tape that recycles it also holds the chunk
// the token sits in. A reader that peeks with lineComment where it should take
// with takeLineComment leaves one behind, and nothing else would notice: the
// tree comes out the same either way.
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

	p := New(Comments())

	file, err := p.Parse([]byte(src))
	require.NoError(t, err)

	// The reader fills the index as it groups, so it is empty before the parse
	// and has to be empty again after it. What proves the parse handed the
	// comments over is the render: every one of them is back in the document.
	require.Empty(t, p.lineComments,
		"%d line comments were peeked at rather than taken", len(p.lineComments))

	// Every comment reaches the tree, so the document renders back as itself.
	require.Equal(t, src, file.String())
}
