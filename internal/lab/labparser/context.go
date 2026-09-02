// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package labparser

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

// tokenRef is where the parser stands in a run of tokens.
//
// The run is either in hand -- a group's members, which are two tokens most of
// the time -- or drawn from a stream as it is read. Either way the parser only
// ever reads forward from idx, one token ahead at the most, so the tokens
// before it are never asked for again.
type tokenRef struct {
	tokens []*Token
	// pair is where a group's two members are copied to, so that reading a
	// group needs no slice of its own. tokens points into it.
	pair [2]*Token
	// idx is where the parser stands, counted from the start of the run. base
	// is where tokens[0] stands, so tokens holds [base, base+len) and idx is
	// never below base.
	idx  int
	base int
	// pull draws the next token of a stream, where the run is one. It is nil
	// for a run already in hand, and drained once the stream has ended.
	pull    func() (*Token, bool)
	drained bool
}

// at returns the i'th token of the run, drawing from the stream where it has to
// and where there is one. It returns nil past the end of the run.
func (r *tokenRef) at(i int) *Token {
	for r.pull != nil && !r.drained && i >= r.base+len(r.tokens) {
		tk, ok := r.pull()
		if !ok {
			r.drained = true

			break
		}
		r.tokens = append(r.tokens, tk)
	}

	if i >= r.base && i-r.base < len(r.tokens) {
		return r.tokens[i-r.base]
	}

	return nil
}

// forget drops what the run holds below idx.
//
// A run drawn from a stream would otherwise keep every token it drew, which is
// the document over again. The parser reads forward from idx and one token
// ahead at most, so nothing below idx is asked for again -- except by
// nextNotCommentToken, which reads forward from idx and not back.
//
// A run already in hand is left alone: it holds a group's members, which the
// group owns and this does not.
func (r *tokenRef) forget() {
	if r.pull == nil {
		return
	}

	drop := r.idx - r.base
	if drop <= 0 {
		return
	}
	r.tokens = append(r.tokens[:0], r.tokens[drop:]...)
	r.base = r.idx
}

// end returns the index just past the run, drawing the rest of the stream where
// there is one.
func (r *tokenRef) end() int {
	for r.pull != nil && !r.drained {
		tk, ok := r.pull()
		if !ok {
			r.drained = true

			break
		}
		r.tokens = append(r.tokens, tk)
	}

	return r.base + len(r.tokens)
}

func (c context) currentToken() *Token {
	return c.tokenRef.at(c.tokenRef.idx)
}

func (c context) isComment() bool {
	return c.currentToken().Type() == token.CommentType
}

func (c context) nextToken() *Token {
	return c.tokenRef.at(c.tokenRef.idx + 1)
}

func (c context) nextNotCommentToken() *Token {
	for i := c.tokenRef.idx + 1; ; i++ {
		tk := c.tokenRef.at(i)
		if tk == nil {
			break
		}
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

// withPull returns a context reading a run drawn one token at a time, rather
// than one already in hand.
func (c context) withPull(p *Parser, pull func() (*Token, bool)) context {
	c.depth++
	c.tokenRef = p.tokenRefFrom(c.depth, pull)
	p.body = c.tokenRef

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
	// Sized from the tokens of the stream, not from the documents it holds:
	// len(p.documents) is the document count, which is one for most streams and
	// left every block at its floor of sixteen nodes.
	p.arena = ast.NewArena(p.tokens.Len())
	ctx := context{arena: p.arena, lineComments: p.lineComments}

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

// takeLineComment returns the comment closing the line tk stands on and drops
// it from the index.
//
// A comment goes to one node, so the index needs it only until the parse
// reaches that node. Dropping it there releases the token it points at: left in
// place, every commented token of the document stays reachable until the parse
// ends, which on an annotated specification is one token in every fifty.
func (c context) takeLineComment(tk *Token) *token.Token {
	if tk == nil {
		return nil
	}

	comment := c.lineComments[tk]
	if comment != nil {
		delete(c.lineComments, tk)
	}

	return comment
}

func (c context) goNext() {
	ref := c.tokenRef
	if ref.at(ref.idx+1) == nil {
		ref.idx = ref.end()
	} else {
		ref.idx++
	}
	ref.forget()
}

func (c context) next() bool {
	return c.tokenRef.at(c.tokenRef.idx) != nil
}

// insertNullToken returns the implicit null a mapping or sequence entry written
// without a value stands for.
//
// The token is not put into the run. The descent reads forward from where it
// stands and never asks for a token again, so the only reader of this one is
// the node built from it, which holds it directly.
func (c context) insertNullToken(tk *Token) *Token {
	return c.createImplicitNullToken(tk)
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
	return newSynthetic(tk)
}

func (c context) addToken(tk *Token) {
	ref := c.tokenRef
	ref.end() // the token goes after everything the run holds
	ref.tokens = append(ref.tokens, tk)
}
