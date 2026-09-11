// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/internal/scanner"
	"github.com/go-openapi/go-yaml/internal/tokenarena"
	"github.com/go-openapi/go-yaml/parser/group"
	"github.com/go-openapi/go-yaml/parser/key"
	"github.com/go-openapi/go-yaml/token"
)

// ParseBytes reads the YAML stream src with a new [Parser] configured with opts.
//
// src is not copied. The returned [ast.File] keeps slices of it, so do not write to src while the file is in use.
// On error the file is nil.
func ParseBytes(src []byte, opts ...Option) (*ast.File, error) {
	return New(opts...).Parse(src)
}

// Parser reads a YAML stream into an [ast.File].
//
// Build one with [New], and use it for one stream: call [Parser.Parse] or [Parser.Walk] once.
// A Parser is not safe for concurrent use.
type Parser struct {
	// tokens holds every token.Token the tree points at, in chunks it can fill
	// again once the parse has finished reading them. A full scan pins it and
	// never lets go, so nothing is recycled and every token stands.
	tokens *tokenarena.TokenArena[group.TapeToken]
	// src is the document being read, kept so that a node can be given the
	// text it was written as. A folded block scalar is the one that needs it.
	src string
	// lineComments holds the comment closing a token's line, against that
	// token. It is nil where the parse was not asked for comments.
	lineComments map[*group.TapeToken]*token.Token
	// opts holds what the [Option] arguments to [New] wrote, and nothing
	// changes it after that. A document's own %YAML and %TAG declarations go to
	// yamlVersion and tagHandles instead.
	opts options

	// yamlVersion is the version the document being read named. Where it names
	// none, opts.version stands.
	yamlVersion YAMLVersion
	// tagHandles maps a handle a TAG directive declared to the prefix it
	// expands to.
	tagHandles map[string]string

	// keys records the keys of the mapping being read and notes a repeat on
	// that mapping. The parser names the keys it records; see keys.go.
	keys key.Ledger

	// walk is where a Walk stands, and nil for a parse that gathers a tree
	// rather than handing it over.
	walk *walkState

	// anchorFrom holds where each anchor open at this point in the descent
	// begins, innermost last. Anchors nest, so it is a stack.
	anchorFrom []int32

	// descent holds the collections the parse has open, the entry it is
	// reading, and the two depths it counts. See descent.go.
	descent descentState

	// anchors holds the anchors of the document being read, and the aliases
	// that named one before its node existed. See anchors.go.
	anchors anchorTable

	// scan reads src into tokens, one at a time, as the reader asks.
	scan scanner.Scanner
	// reader groups what the scanner reads and hands over a document at a time.
	reader *reader
	// body is the run the document's own tokens are drawn from, which is the
	// outermost of the descent. The tail follows it: every level below holds
	// tokens at or behind where it stands.
	body *tokenRef

	// arena is where the nodes of the parse in hand come from. It is kept so
	// that what a tree cost can be read after the parse rather than guessed at
	// from a heap profile -- see [Parser.ArenaStats].
	arena *ast.Arena

	// pathSlab hands out path trie nodes in blocks, so a document of N keys
	// costs N/pathSlabSize allocations rather than N.
	pathSlab []ast.PathNode
	// refs holds one token reference per depth of the descent. They are held by
	// pointer, so growing the slice leaves the ones in hand where they are.
	refs []*tokenRef
}

// New returns a [Parser] configured with opts.
//
// It reads nothing. Pass a stream to [Parser.Parse] or [Parser.Walk].
func New(opts ...Option) *Parser {
	p := &Parser{}
	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Parse reads the YAML stream src and returns its documents as an [ast.File].
//
// src is not copied. The file keeps slices of it, so do not write to src while the file is in use.
// On error the file is nil, and the error carries the source around the failure.
//
// Call Parse once per Parser.
// The parse attaches each comment to its node as it builds the tree, and a second call finds no comment left to attach.
func (p *Parser) Parse(src []byte) (*ast.File, error) {
	p.begin(src)

	file, err := p.parse(p.newContext())
	if err != nil {
		return nil, drawUnder(src, err)
	}

	return file, nil
}
