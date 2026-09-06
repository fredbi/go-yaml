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
		p.keepComments = true
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

// WithJSONCompatible refuses a document JSON has no spelling for, so a
// converter fails on the document rather than inventing one.
//
// Two things go. A sequence or a mapping used as a mapping key:
// [github.com/go-openapi/go-yaml/codec.ToJSON] wrote the key's own JSON text,
// {"[\"a\",\"b\"]":1}, and the decoder wrote what Go printed, {"[a b]":1} --
// two spellings, neither of which reads back as the key. And the infinities and
// NaN, which JSON has no number for and which both wrote as null.
//
// Everything else YAML holds converts and is left alone: a non-string scalar
// key is quoted, so "1.5: a" is {"1.5":"a"}; an alias writes what its anchor
// wrote; a "<<" folds the mapping it names into the one holding it; and a tag
// resolves.
//
// One case it does not catch: an alias standing as a key, "? *x", where the
// anchor names a collection. The parser does not substitute aliases, so it
// cannot see what the key will be, and the conversion invents a spelling as
// before.
//
// [github.com/go-openapi/go-yaml/codec.ToJSON] parses with this on. Use it on a
// parse of your own to find out whether a document converts before converting
// it.
func WithJSONCompatible() Option {
	return func(p *Parser) {
		p.jsonCompatible = true
	}
}

// WithAnchors publishes anchors declared elsewhere, which an alias of any
// document of this stream may name.
//
// The parser resolves every alias against the anchors of the document holding
// it and refuses one that names none, so a document meant to be read alongside
// others -- the files [github.com/go-openapi/go-yaml.Decoder] is given by
// ReferenceFiles, which exist to publish their anchors -- needs the outside
// names handed to it. Pass [ast.DocumentNode.Anchors] from the parse that
// declared them.
//
// What this publishes is not what a document declares: it does not reach
// [ast.DocumentNode.Anchors], and a name the document declares itself hides the
// published one for that document.
func WithAnchors(anchors map[string]ast.Node) Option {
	return func(p *Parser) {
		p.declaredAnchors = anchors
	}
}
