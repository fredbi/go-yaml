package parser

import (
	"strconv"
	"strings"

	"github.com/go-openapi/go-yaml/token"
)

// context context at parsing
type context struct {
	tokenRef *tokenRef
	path     string
	isFlow   bool
	// inFlowSequence distinguishes "[a: b]" from "{a: b}". A pair written
	// inside a flow sequence is an implicit key, which a flow mapping's key is
	// not, and the two are held to different rules.
	inFlowSequence bool
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

// pathSpecialChars are the characters a YAMLPath reads as syntax. A key
// containing one of them is quoted so the path still addresses that one key.
const pathSpecialChars = "$*.[]"

func normalizePath(path string) string {
	if strings.ContainsAny(path, pathSpecialChars) {
		return "'" + path + "'"
	}
	return path
}

func (c *context) currentToken() *Token {
	if c.tokenRef.idx >= c.tokenRef.size {
		return nil
	}
	return c.tokenRef.tokens[c.tokenRef.idx]
}

func (c *context) isComment() bool {
	return c.currentToken().Type() == token.CommentType
}

func (c *context) nextToken() *Token {
	if c.tokenRef.idx+1 >= c.tokenRef.size {
		return nil
	}
	return c.tokenRef.tokens[c.tokenRef.idx+1]
}

func (c *context) nextNotCommentToken() *Token {
	for i := c.tokenRef.idx + 1; i < c.tokenRef.size; i++ {
		tk := c.tokenRef.tokens[i]
		if tk.Type() == token.CommentType {
			continue
		}
		return tk
	}
	return nil
}

func (c *context) isTokenNotFound() bool {
	return c.currentToken() == nil
}

func (c *context) withGroup(g *TokenGroup) *context {
	ctx := *c
	ctx.tokenRef = &tokenRef{
		tokens: g.Tokens,
		size:   len(g.Tokens),
	}
	return &ctx
}

func (c *context) withChild(path string) *context {
	ctx := *c
	ctx.path = c.path + "." + normalizePath(path)
	return &ctx
}

// withPath returns a context at path, which the caller has already built.
// parseMapKey stores a key's path on the key node; the entry's value hangs
// under the same path, so reusing it saves building the same string twice.
func (c *context) withPath(path string) *context {
	ctx := *c
	ctx.path = path
	return &ctx
}

func (c *context) withIndex(idx uint) *context {
	ctx := *c
	ctx.path = c.path + "[" + strconv.FormatUint(uint64(idx), 10) + "]"
	return &ctx
}

// withMapping returns a context whose recorded keys start at base. The keys of
// the mapping opened there are compared against each other and against no
// others.
func (c *context) withMapping(base int) *context {
	ctx := *c
	ctx.keyBase = base

	return &ctx
}

func (c *context) withFlow(isFlow bool) *context {
	ctx := *c
	ctx.isFlow = isFlow
	ctx.inFlowSequence = false
	return &ctx
}

func (c *context) withFlowSequence() *context {
	ctx := *c
	ctx.isFlow = true
	ctx.inFlowSequence = true
	return &ctx
}

func newContext() *context {
	return &context{
		path: "$",
	}
}

func (c *context) goNext() {
	ref := c.tokenRef
	if ref.size <= ref.idx+1 {
		ref.idx = ref.size
	} else {
		ref.idx++
	}
}

func (c *context) next() bool {
	return c.tokenRef.idx < c.tokenRef.size
}

func (c *context) insertNullToken(tk *Token) *Token {
	nullToken := c.createImplicitNullToken(tk)
	c.insertToken(nullToken)
	c.goNext()

	return nullToken
}

func (c *context) addNullValueToken(tk *Token) *Token {
	nullToken := c.createImplicitNullToken(tk)
	rawTk := nullToken.RawToken()

	// add space for map or sequence value.
	rawTk.Position.Column++

	c.addToken(nullToken)
	c.goNext()

	return nullToken
}

func (c *context) createImplicitNullToken(base *Token) *Token {
	pos := *(base.RawToken().Position)
	pos.Column++
	tk := token.New("null", " null", &pos)
	tk.Type = token.ImplicitNullType
	return &Token{Token: tk}
}

func (c *context) insertToken(tk *Token) {
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

func (c *context) addToken(tk *Token) {
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
