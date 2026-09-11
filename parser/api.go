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
	// tokens holds every token.Token the tree points at, in chunks the arena refills once the parse has read them.
	// Parse pins it for the whole parse, so no chunk is refilled and every token survives.
	tokens *tokenarena.TokenArena[group.TapeToken]
	// src is the document being read.
	// parseLiteral takes from it the text a block scalar was written as.
	src string
	// lineComments maps a token to the comment closing its line. It is nil unless [WithComments] was passed.
	lineComments map[*group.TapeToken]*token.Token
	// opts holds the settings the [Option] arguments to [New] wrote. Only begin changes it, to fill chunkSize.
	// A document's own %YAML and %TAG declarations go to yamlVersion and tagHandles.
	opts options

	// yamlVersion is the version the current document's "%YAML" line names, and empty where it names none.
	yamlVersion YAMLVersion
	// tagHandles maps each handle a "%TAG" directive declares to the prefix it expands to.
	tagHandles map[string]string

	// keys records the keys of the mapping being read and notes a repeated key on it.
	// keys.go computes the names it records.
	keys key.Ledger

	// walk holds the state of a [Parser.Walk], and is nil during [Parser.Parse].
	walk *walkState

	// anchorFrom holds the token sequence at which each open anchor begins, innermost last.
	anchorFrom []int32

	// descent holds the collections the parse has open, the entry it is reading, and the two depths it counts.
	// See descent.go.
	descent descentState

	// anchors holds the anchors of the document being read, and the aliases that named one before its node existed.
	// See anchors.go.
	anchors anchorTable

	// scan cuts src into tokens, one at a time, as reader requests them.
	scan scanner.Scanner
	// reader groups the scanner's tokens and returns them one document at a time.
	reader *reader
	// body is the outermost run of the descent, which holds the document's own tokens.
	// readTo moves the arena's tail with it, because every deeper run holds tokens at or behind its position.
	body *tokenRef

	// arena allocates the nodes of the current parse.
	arena *ast.Arena

	// pathSlab provides path trie nodes in blocks, so a document of N keys costs N/pathSlabSize allocations.
	pathSlab []ast.PathNode
	// refs holds one token reference per depth of the descent.
	// They are held by pointer, so growing the slice does not move the references already handed out.
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
