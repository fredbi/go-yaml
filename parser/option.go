// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser

import "github.com/go-openapi/go-yaml/ast"

// Option represents parser's option.
type Option func(p *Parser)

// Comments keeps the comments a document holds. They are dropped by default,
// before the grouping ever sees them.
func Comments() Option {
	return func(p *Parser) {
		p.mode |= parseComments
	}
}

// AllowDuplicateMapKey allow the use of keys with the same name in the same map,
// but by default, this is not permitted.
func AllowDuplicateMapKey() Option {
	return func(p *Parser) {
		p.allowDuplicateMapKey = true
	}
}

// OmitNodePaths stops the parser recording where each node sits in the
// document. [github.com/go-openapi/go-yaml/ast.Node.GetPath] then returns "",
// and so does the CommentMap that [github.com/go-openapi/go-yaml.CommentToMap]
// fills, which is keyed by those paths.
//
// Paths are recorded by default and cost little: they are a trie of 32-byte
// steps, sharing every prefix, built from a slab. Omitting them saves about 1%
// of the bytes a parse allocates, and all of what the tree holds for them --
// 454 KB on a 544 KB OpenAPI specification. Reach for this only when the
// document is large, the paths go unread, and that memory is worth the
// accessor going quiet.
func OmitNodePaths() Option {
	return func(p *Parser) {
		p.omitNodePaths = true
	}
}

// OnComplete calls fn with each node as the parser finishes it, in completion
// order: a node's children are reported before the node. EXPERIMENT
// (2026-08-27) -- the hook a decoder folding nodes into Go values needs.
func OnComplete(fn func(ast.Node)) Option {
	return func(p *Parser) {
		p.onComplete = fn
	}
}

// ChunkSize sets how many tokens one chunk of the token arena holds.
//
// Left unset it is [tokenarena.MaxChunk]. [ParseBytes] sizes it from the length
// of the document instead, with [tokenarena.SizeFor].
func ChunkSize(size int) Option {
	return func(p *Parser) {
		p.chunkSize = size
	}
}
