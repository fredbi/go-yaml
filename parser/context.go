package parser

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
	// arena hands out the nodes the descent builds. It is shared by every
	// context of one parse, so a copy carries the same one.
	arena *ast.Arena
	// keyBase is where the keys of the mapping being parsed start in the
	// parser's key stack. parseMap and parseFlowMap set it; every entry of
	// that mapping is parsed under it, and a nested mapping raises it.
	keyBase int
}

type tokenRef struct {
	tokens []*Token
	size   int
	idx    int
}

func (c context) currentToken() *Token {
	if c.tokenRef.idx >= c.tokenRef.size {
		return nil
	}
	return c.tokenRef.tokens[c.tokenRef.idx]
}

func (c context) isComment() bool {
	return c.currentToken().Type() == token.CommentType
}

func (c context) nextToken() *Token {
	if c.tokenRef.idx+1 >= c.tokenRef.size {
		return nil
	}
	return c.tokenRef.tokens[c.tokenRef.idx+1]
}

func (c context) nextNotCommentToken() *Token {
	for i := c.tokenRef.idx + 1; i < c.tokenRef.size; i++ {
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

func (c context) withGroup(g *TokenGroup) context {
	c.tokenRef = &tokenRef{
		tokens: g.Tokens,
		size:   len(g.Tokens),
	}

	return c
}

func (c context) withChild(p *parser, key string) context {
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

func (c context) withIndex(p *parser, idx uint) context {
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

func (p *parser) newContext() context {
	ctx := context{arena: ast.NewArena(len(p.tokens))}

	root := p.newPathNode()
	if root == nil {
		return ctx
	}
	root.Literal("$")
	ctx.path = root

	return ctx
}

func (c context) goNext() {
	ref := c.tokenRef
	if ref.size <= ref.idx+1 {
		ref.idx = ref.size
	} else {
		ref.idx++
	}
}

func (c context) next() bool {
	return c.tokenRef.idx < c.tokenRef.size
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
	pos := *(base.RawToken().Position)
	pos.Column++
	tk := token.New("null", " null", &pos)
	tk.Type = token.ImplicitNullType
	return &Token{Token: tk}
}

func (c context) insertToken(tk *Token) {
	ref := c.tokenRef
	idx := ref.idx
	if ref.size < idx {
		return
	}
	if ref.size == idx {
		curToken := ref.tokens[ref.size-1]
		tk.RawToken().Next = curToken.RawToken()
		curToken.RawToken().Prev = tk.RawToken()

		ref.tokens = append(ref.tokens, tk)
		ref.size = len(ref.tokens)
		return
	}

	curToken := ref.tokens[idx]
	tk.RawToken().Next = curToken.RawToken()
	curToken.RawToken().Prev = tk.RawToken()

	ref.tokens = append(ref.tokens[:idx+1], ref.tokens[idx:]...)
	ref.tokens[idx] = tk
	ref.size = len(ref.tokens)
}

func (c context) addToken(tk *Token) {
	ref := c.tokenRef
	lastTk := ref.tokens[ref.size-1]
	if lastTk.Group != nil {
		lastTk = lastTk.Group.Last()
	}
	lastTk.RawToken().Next = tk.RawToken()
	tk.RawToken().Prev = lastTk.RawToken()

	ref.tokens = append(ref.tokens, tk)
	ref.size = len(ref.tokens)
}
