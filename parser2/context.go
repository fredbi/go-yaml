// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser2

import (
	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/token"
)

// context is the parser's state at one point in the descent.
//
// It is passed and returned by value: the struct is five words, and copying it
// costs less than the heap allocation and collection a pointer would need. The
// withX methods each return a copy with one field changed, so a context handed
// to a child cannot be seen by its parent. tokenRef is the exception -- it is a
// pointer, so goNext and insertToken advance the position every context in the
// descent shares.
type context struct {
	tokenRef *tokenRef
	path     *ast.PathNode
	isFlow   bool
	// inFlowSequence distinguishes "[a: b]" from "{a: b}". A pair written
	// inside a flow sequence is an implicit key, which a flow mapping's key is
	// not, and the two are held to different rules.
	inFlowSequence bool
	// depth counts the groups stepped into to reach here, and says which token
	// reference this context reads. It sits beside the flags, in room the
	// struct was padding out anyway.
	depth int32
	// arena hands out the nodes the descent builds. It is shared by every
	// context of one parse, so a copy carries the same one.
	arena *ast.Arena
	// lineComments holds the comment closing a token's line, against that
	// token. It is nil where the mode did not ask for comments, and reading a
	// nil map costs nothing.
	lineComments map[*Token]*token.Token
	// keyBase is where the keys of the mapping being parsed start in the
	// parser's key stack. parseMap and parseFlowMap set it; every entry of
	// that mapping is parsed under it, and a nested mapping raises it.
	keyBase int
}

type tokenRef struct {
	tokens []*Token
	// pair is where a group's two members are copied to, so that reading a
	// group needs no slice of its own. tokens points into it.
	pair [2]*Token
	idx  int
}

func (c context) currentToken() *Token {
	if c.tokenRef.idx >= len(c.tokenRef.tokens) {
		return nil
	}
	return c.tokenRef.tokens[c.tokenRef.idx]
}

func (c context) isComment() bool {
	return c.currentToken().Type() == token.CommentType
}

func (c context) nextToken() *Token {
	if c.tokenRef.idx+1 >= len(c.tokenRef.tokens) {
		return nil
	}
	return c.tokenRef.tokens[c.tokenRef.idx+1]
}

func (c context) nextNotCommentToken() *Token {
	for i := c.tokenRef.idx + 1; i < len(c.tokenRef.tokens); i++ {
		tk := c.tokenRef.tokens[i]
		if tk.Type() == token.CommentType {
			continue
		}
		return tk
	}
	return nil
}

func (c context) isTokenNotFound() bool {
	return c.currentToken() == nil
}

func (c context) withGroup(p *Parser, g *TokenGroup) context {
	c.depth++
	c.tokenRef = p.tokenRefAt(c.depth, g)

	return c
}

func (c context) withChild(p *Parser, key string) context {
	n := p.newPathNode()
	if n == nil {
		return c
	}
	n.Key(c.path, key)
	c.path = n

	return c
}

// withPath returns a context at path, which the caller has already built.
// parseMapKey stores a key's path on the key node; the entry's value hangs
// under the same path, so reusing it saves building the same string twice.
func (c context) withPath(path *ast.PathNode) context {
	c.path = path

	return c
}

func (c context) withIndex(p *Parser, idx uint) context {
	n := p.newPathNode()
	if n == nil {
		return c
	}
	n.Index(c.path, idx)
	c.path = n

	return c
}

// withMapping returns a context whose recorded keys start at base. The keys of
// the mapping opened there are compared against each other and against no
// others.
func (c context) withMapping(base int) context {
	c.keyBase = base

	return c
}

func (c context) withFlow(isFlow bool) context {
	c.isFlow = isFlow
	c.inFlowSequence = false

	return c
}

func (c context) withFlowSequence() context {
	c.isFlow = true
	c.inFlowSequence = true

	return c
}

func (p *Parser) newContext() context {
	ctx := context{arena: ast.NewArena(len(p.tokens)), lineComments: p.lineComments}

	root := p.newPathNode()
	if root == nil {
		return ctx
	}
	root.Literal("$")
	ctx.path = root

	return ctx
}

// lineComment returns the comment closing the line tk stands on, or nil where
// there is none. A stream read without ParseComments has none at all.
func (c context) lineComment(tk *Token) *token.Token {
	return c.lineComments[tk]
}

func (c context) goNext() {
	ref := c.tokenRef
	if len(ref.tokens) <= ref.idx+1 {
		ref.idx = len(ref.tokens)
	} else {
		ref.idx++
	}
}

func (c context) next() bool {
	return c.tokenRef.idx < len(c.tokenRef.tokens)
}

func (c context) insertNullToken(tk *Token) *Token {
	nullToken := c.createImplicitNullToken(tk)
	c.insertToken(nullToken)
	c.goNext()

	return nullToken
}

func (c context) addNullValueToken(tk *Token) *Token {
	nullToken := c.createImplicitNullToken(tk)
	rawTk := nullToken.RawToken()

	// add space for map or sequence value.
	rawTk.Position.Column++

	c.addToken(nullToken)
	c.goNext()

	return nullToken
}

func (c context) createImplicitNullToken(base *Token) *Token {
	pos := base.RawToken().Position
	pos.Column++
	tk := token.New("null", " null", pos)
	tk.Type = token.ImplicitNullType
	return &Token{Token: tk}
}

func (c context) insertToken(tk *Token) {
	ref := c.tokenRef
	idx := ref.idx
	if len(ref.tokens) < idx {
		return
	}
	if len(ref.tokens) == idx {
		ref.tokens = append(ref.tokens, tk)

		return
	}

	ref.tokens = append(ref.tokens[:idx+1], ref.tokens[idx:]...)
	ref.tokens[idx] = tk
}

func (c context) addToken(tk *Token) {
	ref := c.tokenRef
	ref.tokens = append(ref.tokens, tk)
}
