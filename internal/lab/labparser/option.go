// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package labparser

// Option represents parser's option.
type Option func(p *Parser)

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
