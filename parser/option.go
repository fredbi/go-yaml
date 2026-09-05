// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser

import "github.com/go-openapi/go-yaml/ast"

// Option represents parser's option.
type Option func(p *Parser)

// WithComments keeps the comments a document holds. They are dropped by default,
// before the grouping ever sees them.
func WithComments() Option {
	return func(p *Parser) {
		p.mode |= parseComments
	}
}

// WithAllowDuplicateMapKey allow the use of keys with the same name in the same map,
// but by default, this is not permitted.
func WithAllowDuplicateMapKey() Option {
	return func(p *Parser) {
		p.allowDuplicateMapKey = true
	}
}

// WithOmitNodePaths stops the parser recording where each node sits in the
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
func WithOmitNodePaths() Option {
	return func(p *Parser) {
		p.omitNodePaths = true
	}
}

// WithOnComplete calls fn with each node as the parser finishes it, in completion
// order: a node's children are reported before the node. EXPERIMENT
// (2026-08-27) -- the hook a decoder folding nodes into Go values needs.
func WithOnComplete(fn func(ast.Node)) Option {
	return func(p *Parser) {
		p.onComplete = fn
	}
}

// WithChunkSize sets how many tokens one chunk of the token arena holds.
//
// Left unset it is [tokenarena.MaxChunk]. [ParseBytes] sizes it from the length
// of the document instead, with [tokenarena.SizeFor].
func WithChunkSize(size int) Option {
	return func(p *Parser) {
		p.chunkSize = size
	}
}

// WithYAMLVersion says which version of the specification a document is read
// as where it names none itself.
//
// It decides how a plain scalar resolves, and the two versions disagree about
// several: 1.1 reads "0100" as 64, "1_000" as 1000, "1:30" as 90 and "yes" as
// true, where 1.2 reads 100 and the three strings. The default is [YAML12].
//
// A "%YAML" directive overrides this for the document it opens, and the version
// goes back to what was asked for here when that document ends.
func WithYAMLVersion(v YAMLVersion) Option {
	return func(p *Parser) {
		p.version = v
	}
}
