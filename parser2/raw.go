// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser2

import "github.com/go-openapi/go-yaml/token"

// rawTokenBlockSizes gives the size of each block in turn, the last of them for
// every block after the fourth.
var rawTokenBlockSizes = [...]int{64, 128, 256, 512}

// rawTokens holds the tokens read from a scanner, as values, in blocks that are
// never copied or resized.
//
// The parser addresses tokens throughout -- a Token wraps one, and a node keeps
// the one it was built from -- so a token has to stay where it was put. Holding
// them in blocks costs one allocation per block rather than one per token, and
// keeps the whole stream out of the scanner's hands.
type rawTokens struct {
	blocks [][]token.Token
	n      int
}

// add copies tk into the store and returns where it now stands.
func (r *rawTokens) add(tk token.Token) *token.Token {
	last := len(r.blocks) - 1
	if last < 0 || len(r.blocks[last]) == cap(r.blocks[last]) {
		size := rawTokenBlockSizes[min(len(r.blocks), len(rawTokenBlockSizes)-1)]
		r.blocks = append(r.blocks, make([]token.Token, 0, size))
		last++
	}

	block := &r.blocks[last]
	*block = append(*block, tk)
	r.n++

	return &(*block)[len(*block)-1]
}
