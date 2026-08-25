// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser2

import (
	"fmt"
	"iter"
	"os"
	"strings"

	"github.com/go-openapi/go-yaml/internal/errors"
	"github.com/go-openapi/go-yaml/token"
)

type TokenGroupType uint8

const (
	TokenGroupNone TokenGroupType = iota
	TokenGroupDirective
	TokenGroupDirectiveName
	TokenGroupDocument
	TokenGroupDocumentBody
	TokenGroupAnchor
	TokenGroupAnchorName
	TokenGroupAlias
	TokenGroupLiteral
	TokenGroupFolded
	TokenGroupScalarTag
	TokenGroupMapKey
	TokenGroupMapKeyValue
)

func (t TokenGroupType) String() string {
	switch t {
	case TokenGroupNone:
		return "none"
	case TokenGroupDirective:
		return "directive"
	case TokenGroupDirectiveName:
		return "directive_name"
	case TokenGroupDocument:
		return "document"
	case TokenGroupDocumentBody:
		return "document_body"
	case TokenGroupAnchor:
		return "anchor"
	case TokenGroupAnchorName:
		return "anchor_name"
	case TokenGroupAlias:
		return "alias"
	case TokenGroupLiteral:
		return "literal"
	case TokenGroupFolded:
		return "folded"
	case TokenGroupScalarTag:
		return "scalar_tag"
	case TokenGroupMapKey:
		return "map_key"
	case TokenGroupMapKeyValue:
		return "map_key_value"
	}
	return "none"
}

type Token struct {
	Token *token.Token
	Group *TokenGroup
}

func (t *Token) RawToken() *token.Token {
	if t == nil {
		return nil
	}
	if t.Token != nil {
		return t.Token
	}
	return t.Group.RawToken()
}

func (t *Token) Type() token.Type {
	if t == nil {
		return 0
	}
	if t.Token != nil {
		return t.Token.Type
	}
	return t.Group.TokenType()
}

func (t *Token) GroupType() TokenGroupType {
	if t == nil {
		return TokenGroupNone
	}
	if t.Token != nil {
		return TokenGroupNone
	}
	return t.Group.Type
}

func (t *Token) Line() int {
	if t == nil {
		return 0
	}
	if t.Token != nil {
		return int(t.Token.Position.Line)
	}
	return t.Group.Line()
}

func (t *Token) Column() int {
	if t == nil {
		return 0
	}
	if t.Token != nil {
		return int(t.Token.Position.Column)
	}
	return t.Group.Column()
}

func (t *Token) SetGroupType(typ TokenGroupType) {
	if t.Group == nil {
		return
	}
	t.Group.Type = typ
}

func (t *Token) Dump() {
	ctx := new(groupTokenRenderContext)
	if t.Token != nil {
		fmt.Fprint(os.Stdout, t.Token.Value)
		return
	}
	t.Group.dump(ctx)
	fmt.Fprintf(os.Stdout, "\n")
}

func (t *Token) dump(ctx *groupTokenRenderContext) {
	if t.Token != nil {
		fmt.Fprint(os.Stdout, t.Token.Value)
		return
	}
	t.Group.dump(ctx)
}

type groupTokenRenderContext struct {
	num int
}

// TokenGroup is a run of tokens the parser reads as one.
//
// Nearly every group holds exactly two members -- a key and its ':', a key
// group and its value, an anchor and what it names -- so the two are held in
// the group itself. Only a document, an explicit key and a directive hold more,
// and those keep a slice: 446 of the 256,848 groups the corpus builds, 0.17%.
// Holding the common pair inline saves the run of pointers a slice would need.
type TokenGroup struct {
	a, b *Token
	// more holds the members where there are more than two, and a and b are
	// then unused. It is a pointer to a slice rather than a slice so that the
	// group stays 32 bytes.
	more *[]*Token
	Type TokenGroupType
	n    uint8
}

// newTokenGroup returns a group of typ over tks, on the heap. The grouper hands
// its own out from blocks; this is for the few the parser builds itself.
func newTokenGroup(typ TokenGroupType, tks []*Token) *TokenGroup {
	g := new(TokenGroup)
	g.set(typ, tks)

	return g
}

// set fills g with the members tks, keeping two of them in the group itself.
func (g *TokenGroup) set(typ TokenGroupType, tks []*Token) {
	g.Type = typ
	switch len(tks) {
	case 0:
		g.a, g.b, g.more, g.n = nil, nil, nil, 0
	case 1:
		g.a, g.b, g.more, g.n = tks[0], nil, nil, 1
	case 2:
		g.a, g.b, g.more, g.n = tks[0], tks[1], nil, 2
	default:
		held := tks
		g.a, g.b, g.more, g.n = nil, nil, &held, uint8(min(len(tks), 255))
	}
}

// Len returns how many members g holds.
func (g *TokenGroup) Len() int {
	if g.more != nil {
		return len(*g.more)
	}

	return int(g.n)
}

// At returns the i'th member.
func (g *TokenGroup) At(i int) *Token {
	if g.more != nil {
		return (*g.more)[i]
	}
	if i == 0 {
		return g.a
	}

	return g.b
}

// Members returns the members as a slice, writing the inline pair into pair
// where there is one. The caller owns pair, so nothing is allocated for it.
func (g *TokenGroup) Members(pair *[2]*Token) []*Token {
	if g.more != nil {
		return *g.more
	}
	pair[0], pair[1] = g.a, g.b

	return pair[:g.n]
}

func (g *TokenGroup) First() *Token {
	if g.Len() == 0 {
		return nil
	}

	return g.At(0)
}

func (g *TokenGroup) Last() *Token {
	n := g.Len()
	if n == 0 {
		return nil
	}

	return g.At(n - 1)
}

func (g *TokenGroup) dump(ctx *groupTokenRenderContext) {
	num := ctx.num
	fmt.Fprint(os.Stdout, colorize(num, "("))
	ctx.num++
	for i := range g.Len() {
		g.At(i).dump(ctx)
	}
	fmt.Fprint(os.Stdout, colorize(num, ")"))
}

func (g *TokenGroup) RawToken() *token.Token {
	if g.Len() == 0 {
		return nil
	}

	return g.At(0).RawToken()
}

func (g *TokenGroup) Line() int {
	if g.Len() == 0 {
		return 0
	}

	return g.At(0).Line()
}

func (g *TokenGroup) Column() int {
	if g.Len() == 0 {
		return 0
	}

	return g.At(0).Column()
}

func (g *TokenGroup) TokenType() token.Type {
	if g.Len() == 0 {
		return 0
	}

	return g.At(0).Type()
}

// grouper runs the passes that turn a flat token stream into grouped tokens.
//
// It hands out the [Token], [TokenGroup] and []*Token that grouping needs from
// blocks rather than one allocation each. A stream of N tokens groups into
// roughly N wrappers, groups and slices, and those three were the largest
// allocation sites of a parse by count.
type grouper struct {
	tokens []Token
	groups []TokenGroup
	// passA and passB are the two buffers the grouping passes write into. A
	// pass reads one and writes the other, so the nine of them cost two
	// allocations between them rather than one apiece.
	passA, passB []*Token
	writeB       bool
	// nested counts the passes running inside another pass.
	nested int
	// err is the first refusal a pass reported. A pass that fails stops
	// yielding, so the passes below it read a stream that ends early;
	// createGroupedTokens reads err rather than what they made of it.
	err error
	// lineComments holds the comment written at the end of a token's line,
	// against the token it belongs to. It stays nil where the mode did not ask
	// for comments, and then no token has one.
	lineComments map[*Token]*token.Token
	// block is how many of each one allocation covers.
	block int
}

// setLineComment records that comment closes the line tk stands on.
func (g *grouper) setLineComment(tk *Token, comment *token.Token) {
	if g.lineComments == nil {
		g.lineComments = make(map[*Token]*token.Token)
	}
	g.lineComments[tk] = comment
}

// fail records a refusal. The first stands: a pass that stops yielding leaves
// the ones below it reading a stream that ends early, and what they make of
// that says less than what went wrong here.
func (g *grouper) fail(err error) {
	if g.err == nil {
		g.err = err
	}
}

// out returns an empty slice with room for n tokens, taken from whichever
// buffer the caller is not reading.
//
// The result of the last pass is kept, and the groups createDocumentTokens
// builds address the buffer it read. Nothing may write either buffer after
// that, so a pass added to CreateGroupedTokens goes before that one.
func (g *grouper) out(n int) []*Token {
	g.writeB = !g.writeB

	if g.nested > 0 {
		// A pass running inside another takes a buffer of its own: both of the
		// grouper's are in hand, one being read and one being filled.
		return make([]*Token, 0, n)
	}

	g.writeB = !g.writeB

	buf := &g.passA
	if g.writeB {
		buf = &g.passB
	}
	if cap(*buf) < n {
		*buf = make([]*Token, 0, n)
	}

	return (*buf)[:0]
}

const (
	minGroupBlock = 16
	maxGroupBlock = 256
)

// newGrouper returns a grouper for a stream of n tokens.
//
// Grouping turns roughly one token in four into a group, so that is where the
// block size starts. It is capped both ways: a long document allocates more
// blocks rather than one huge one, and a short document does not pay for a
// block it will use a tenth of.
func newGrouper(n int) grouper {
	block := n / 4
	if block < minGroupBlock {
		block = minGroupBlock
	}
	if block > maxGroupBlock {
		block = maxGroupBlock
	}

	return grouper{block: block}
}

func (g *grouper) token() *Token {
	if len(g.tokens) == 0 {
		g.tokens = make([]Token, g.block)
	}
	tk := &g.tokens[0]
	g.tokens = g.tokens[1:]

	return tk
}

// newGroup1 and newGroup2 return a group over one and two tokens, taken from
// the block and filled without a list.
func (g *grouper) newGroup1(typ TokenGroupType, a *Token) *TokenGroup {
	grp := g.nextGroup()
	grp.Type, grp.a, grp.b, grp.more, grp.n = typ, a, nil, nil, 1

	return grp
}

func (g *grouper) newGroup2(typ TokenGroupType, a, b *Token) *TokenGroup {
	grp := g.nextGroup()
	grp.Type, grp.a, grp.b, grp.more, grp.n = typ, a, b, nil, 2

	return grp
}

// nextGroup returns the next unused group of the block.
func (g *grouper) nextGroup() *TokenGroup {
	if len(g.groups) == 0 {
		g.groups = make([]TokenGroup, g.block)
	}
	grp := &g.groups[0]
	g.groups = g.groups[1:]

	return grp
}

// newGroup returns a group of typ over tks.
func (g *grouper) newGroup(typ TokenGroupType, tks []*Token) *TokenGroup {
	grp := g.nextGroup()
	grp.set(typ, tks)

	return grp
}

// group returns a token holding a group of typ over tks.
func (g *grouper) group(typ TokenGroupType, tks []*Token) *Token {
	tk := g.token()
	tk.Group = g.newGroup(typ, tks)

	return tk
}

// group1 and group2 are group over one and two tokens, which is most of them.
// Both members are held in the group itself, so neither builds a list.
func (g *grouper) group1(typ TokenGroupType, a *Token) *Token {
	tk := g.token()
	tk.Group = g.newGroup1(typ, a)

	return tk
}

func (g *grouper) group2(typ TokenGroupType, a, b *Token) *Token {
	tk := g.token()
	tk.Group = g.newGroup2(typ, a, b)

	return tk
}

// createGroupedTokens reads the tokens of a stream into the groups the parser
// walks. Each pass takes the tokens the one before it left and groups a little
// more of them.
func createGroupedTokens(raw *rawTokens) ([]*Token, map[*Token]*token.Token, error) {
	g := newGrouper(raw.n)
	tks := g.collect(raw.n, g.groupAnchorsWithScalarTags(
		g.groupScalarTags(
			g.groupAnchors(
				g.groupBlockScalars(
					g.attachLineComments(
						g.stream(raw)))))))
	if g.err != nil {
		return nil, nil, g.err
	}

	tks, err := g.createMapKeyTokenGroups(tks)
	if err != nil {
		return nil, nil, err
	}
	tks = g.createMapKeyValueTokenGroups(tks)
	tks, err = g.createDirectiveTokenGroups(tks)
	if err != nil {
		return nil, nil, err
	}
	tks, err = g.createDocumentTokens(tks)
	if err != nil {
		return nil, nil, err
	}
	return tks, g.lineComments, nil
}

// newTokens wraps every raw token in the [Token] the grouping passes work on.
//
// The wrappers come from one block rather than one allocation each: a stream of
// N tokens then costs two allocations instead of N+1. They are addressed by
// pointer either way, and the block lives exactly as long as any token in it.
// stream yields a Token for each of raw's tokens, wrapping them where they
// stand. The tokens are not moved, and the wrappers come from one block.
//
// Read it once: a second read wraps the same tokens again, in wrappers of its
// own, and the groups built over the first set would not know about them.
func (g *grouper) stream(raw *rawTokens) iter.Seq[*Token] {
	return func(yield func(*Token) bool) {
		block := make([]Token, raw.n)

		var i int
		for _, b := range raw.blocks {
			for j := range b {
				block[i].Token = &b[j]
				if !yield(&block[i]) {
					return
				}
				i++
			}
		}
	}
}

// collect reads a stream into a buffer, for the passes that still read a slice.
func (g *grouper) collect(n int, in iter.Seq[*Token]) []*Token {
	out := g.out(n)
	for tk := range in {
		out = append(out, tk)
	}

	return out
}

// attachLineComments attaches the comment closing a token's line to that token, and
// drops it from the stream.
//
// Nothing is held back. The comment arrives after the token it belongs to, and
// the attachment is recorded against the token rather than written into it, so
// the token may already have been handed on.
func (g *grouper) attachLineComments(in iter.Seq[*Token]) iter.Seq[*Token] {
	return func(yield func(*Token) bool) {
		var prev *Token
		for tk := range in {
			if tk.Type() == token.CommentType && prev != nil && prev.Line() == tk.Line() {
				g.setLineComment(prev, tk.RawToken())

				continue
			}
			if !yield(tk) {
				return
			}
			prev = tk
		}
	}
}

// groupBlockScalars joins a "|" or ">" header with the content that follows it.
//
// One token is held: the header, until the content arrives. A header ending the
// stream has no content, and the group is the header alone -- which is what
// "a: |" with nothing after it is.
func (g *grouper) groupBlockScalars(in iter.Seq[*Token]) iter.Seq[*Token] {
	return func(yield func(*Token) bool) {
		var (
			header *Token
			typ    TokenGroupType
		)

		for tk := range in {
			if header != nil {
				// Whatever follows the header is its content, read as it
				// stands: a second "|" is content, not another header.
				if !yield(g.group2(typ, header, tk)) {
					return
				}
				header = nil

				continue
			}

			switch tk.Type() {
			case token.LiteralType:
				header, typ = tk, TokenGroupLiteral
			case token.FoldedType:
				header, typ = tk, TokenGroupFolded
			default:
				if !yield(tk) {
					return
				}
			}
		}

		if header != nil {
			yield(g.group1(typ, header))
		}
	}
}

// groupAnchors joins "&" with the name after it, that name with what it names,
// and "*" with the name after it.
//
// Two tokens are held at the most: the "&" until its name arrives, and then the
// name group until the token after it says whether the anchor names a scalar on
// the same line or an empty node.
func (g *grouper) groupAnchors(in iter.Seq[*Token]) iter.Seq[*Token] {
	return func(yield func(*Token) bool) {
		var (
			anchor *Token // a "&", waiting for its name
			name   *Token // an anchor name, waiting to see what it names
			alias  *Token // a "*", waiting for its name
		)

		for tk := range in {
			switch {
			case alias != nil:
				if !yield(g.group2(TokenGroupAlias, alias, tk)) {
					return
				}
				alias = nil

				continue
			case anchor != nil:
				name, anchor = g.group2(TokenGroupAnchorName, anchor, tk), nil

				continue
			case name != nil:
				sameLine := name.Line() == tk.Line()
				if sameLine && tk.Type() == token.SequenceEntryType {
					g.fail(errors.ErrSyntax("sequence entries are not allowed after anchor on the same line", tk.RawToken()))

					return
				}
				if sameLine && isScalarType(tk) {
					if !yield(g.group2(TokenGroupAnchor, name, tk)) {
						return
					}
					name = nil

					continue
				}
				// The anchor names the empty node, and tk is read as any other
				// token would be.
				if !yield(name) {
					return
				}
				name = nil
			}

			switch tk.Type() {
			case token.AnchorType:
				anchor = tk
			case token.AliasType:
				alias = tk
			default:
				if !yield(tk) {
					return
				}
			}
		}

		switch {
		case anchor != nil:
			g.fail(errors.ErrSyntax("undefined anchor name", anchor.RawToken()))
		case alias != nil:
			g.fail(errors.ErrSyntax("undefined alias name", alias.RawToken()))
		case name != nil:
			// An anchor with nothing after it names the empty node. The parser
			// supplies that null; there is nothing to group here.
			yield(name)
		}
	}
}

// groupScalarTags joins a tag with the scalar it tags.
//
// One token is held: the tag, until the token after it says whether it tags
// that one or stands on its own. A tag on its own is left in the stream and the
// parser reads what it tags from there -- a tag on its own line, or one in
// front of a collection.
func (g *grouper) groupScalarTags(in iter.Seq[*Token]) iter.Seq[*Token] {
	return func(yield func(*Token) bool) {
		var tag *Token // a tag, waiting to see what it tags

		for tk := range in {
			if tag != nil {
				grouped, ok := g.taggedScalar(tag, tk)
				if !ok {
					return
				}
				if grouped != nil {
					if !yield(grouped) {
						return
					}
					tag = nil

					continue
				}
				// The tag stands on its own, and tk is read as any other token
				// would be -- including as the next tag.
				if !yield(tag) {
					return
				}
				tag = nil
			}

			if tk.Type() == token.TagType {
				tag = tk

				continue
			}
			if !yield(tk) {
				return
			}
		}

		if tag != nil {
			yield(tag)
		}
	}
}

// taggedScalar returns the group joining tag with next, or nil where the tag
// stands on its own. It reports false where the document is refused.
//
// A tag never reaches past its own line, and never takes an anchor name: the
// anchor is what holds the tag, and groupAnchorsWithScalarTags joins those.
func (g *grouper) taggedScalar(tag, next *Token) (*Token, bool) {
	if tag.Line() != next.Line() || next.GroupType() == TokenGroupAnchorName {
		return nil, true
	}

	value := tag.RawToken().Value
	if !strings.HasPrefix(value, "!!") {
		// A tag the document defines. It tags a scalar, and a flow indicator is
		// not one.
		if isFlowType(next) {
			return nil, true
		}

		return g.group2(TokenGroupScalarTag, tag, next), true
	}

	switch token.ReservedTagKeyword(value) {
	case token.IntegerTag, token.FloatTag, token.StringTag,
		token.BinaryTag, token.TimestampTag, token.BooleanTag, token.NullTag:
		if !isScalarType(next) {
			return nil, true
		}

		return g.group2(TokenGroupScalarTag, tag, next), true
	case token.MergeTag:
		if next.Type() != token.MergeKeyType {
			g.fail(errors.ErrSyntax("could not find merge key", next.RawToken()))

			return nil, false
		}

		return g.group2(TokenGroupScalarTag, tag, next), true
	default:
		// A reserved tag that resolves to a collection, or one we do not read:
		// it stands on its own and the parser reads what it tags.
		return nil, true
	}
}

// groupAnchorsWithScalarTags joins an anchor name with a tagged scalar.
//
// groupAnchors could not: the tag was still a token of its own when it ran, and
// only groupScalarTags turns it into the scalar the anchor names. One token is
// held, the anchor name, until the token after it says whether that is what it
// names.
func (g *grouper) groupAnchorsWithScalarTags(in iter.Seq[*Token]) iter.Seq[*Token] {
	return func(yield func(*Token) bool) {
		var name *Token // an anchor name, waiting to see whether a tagged scalar follows

		for tk := range in {
			if name != nil {
				if name.Line() == tk.Line() && tk.GroupType() == TokenGroupScalarTag {
					if !yield(g.group2(TokenGroupAnchor, name, tk)) {
						return
					}
					name = nil

					continue
				}
				// The anchor names something else, or the empty node, and tk is
				// read as any other token would be.
				if !yield(name) {
					return
				}
				name = nil
			}

			if tk.GroupType() == TokenGroupAnchorName {
				name = tk

				continue
			}
			if !yield(tk) {
				return
			}
		}

		if name != nil {
			// An anchor with nothing after it names the empty node. The parser
			// supplies that null; there is nothing to group here.
			yield(name)
		}
	}
}

func (g *grouper) createMapKeyTokenGroups(tokens []*Token) ([]*Token, error) {
	tks, err := g.createMapKeyByMappingKey(tokens)
	if err != nil {
		return nil, err
	}
	return g.createMapKeyByMappingValue(tks)
}

func (g *grouper) createMapKeyByMappingKey(tokens []*Token) ([]*Token, error) {
	ret := g.out(len(tokens))
	var flowDepth int
	for i := 0; i < len(tokens); i++ {
		tk := tokens[i]
		switch tk.Type() {
		case token.MappingStartType, token.SequenceStartType:
			flowDepth++
			ret = append(ret, tk)
		case token.MappingEndType, token.SequenceEndType:
			if flowDepth > 0 {
				flowDepth--
			}
			ret = append(ret, tk)
		case token.MappingKeyType:
			// A '?' with nothing after it opens an entry whose key is e-node,
			// which is what "? \n" and "?\n: v\n" are. The group holds the
			// indicator alone and the parser supplies the null.
			end := explicitKeyEnd(tokens, i, flowDepth > 0)
			body, err := g.groupExplicitKeyBody(tokens[i+1 : end])
			if err != nil {
				return nil, err
			}
			group := []*Token{tk}
			if len(body) == 0 {
				group = append(group, g.implicitNullKeyToken(tk))
			}
			ret = append(ret, g.group(TokenGroupMapKey, append(group, body...)))
			i = end - 1
		default:
			ret = append(ret, tk)
		}
	}
	return ret, nil
}

func (g *grouper) createMapKeyByMappingValue(tokens []*Token) ([]*Token, error) {
	ret := g.out(len(tokens))

	// One entry per flow collection still open, innermost last, recording
	// whether it is a sequence. A pair written directly inside a sequence is an
	// implicit key and has to fit on one line with its ':'; inside a mapping the
	// same pair may span lines.
	var flow []bool
	for i := 0; i < len(tokens); i++ {
		tk := tokens[i]
		switch tk.Type() {
		case token.MappingStartType:
			flow = append(flow, false)
			ret = append(ret, tk)
		case token.SequenceStartType:
			flow = append(flow, true)
			ret = append(ret, tk)
		case token.MappingEndType, token.SequenceEndType:
			if len(flow) > 0 {
				flow = flow[:len(flow)-1]
			}
			ret = append(ret, tk)
		case token.MappingValueType:
			flowDepth := len(flow)
			if hasNoKey(tokens, i, flowDepth > 0) {
				// The key is absent: ": value", "- :", "{ : }", "{a: 1, : 2}".
				// YAML 1.2 allows it, and an absent key is the null node -- so
				// there is nothing to reject here, only a node to supply.
				ret = append(ret, g.group2(TokenGroupMapKey, g.implicitNullKeyToken(tk), tk))

				continue
			}
			mapKeyTk := tokens[keyCandidateIndex(tokens, i)]
			if closesFlowCollection(mapKeyTk) {
				// The key is the flow collection that just closed, so it has
				// to be taken whole: "[a, b]: v" keys on the sequence, not on
				// the ']' that ends it.
				start := flowCollectionStart(ret)
				if start < 0 {
					return nil, errors.ErrSyntax("found an invalid key for this map", tk.RawToken())
				}
				start = withKeyProperties(ret, start)
				if ret[start].Line() != mapKeyTk.Line() {
					// An implicit key has to be a single-line node, so a
					// collection spanning lines cannot be one.
					return nil, errors.ErrSyntax("map key definition includes an implicit line break", tk.RawToken())
				}
				if flowDepth > 0 && flow[flowDepth-1] && mapKeyTk.Line() != tk.Line() {
					// Directly inside a sequence the ':' is part of that one
					// line too. Inside a mapping it is separation like any
					// other, and may follow on the next line.
					return nil, errors.ErrSyntax("map key definition includes an implicit line break", tk.RawToken())
				}
				keyTokens := append(append([]*Token{}, ret[start:]...), tk)
				ret = append(ret[:start], g.group(TokenGroupMapKey, keyTokens))

				continue
			}
			if isNotMapKeyType(mapKeyTk) {
				return nil, errors.ErrSyntax("found an invalid key for this map", tk.RawToken())
			}
			newTk := g.token()
			newTk.Token, newTk.Group = mapKeyTk.Token, mapKeyTk.Group
			mapKeyTk.Token = nil
			mapKeyTk.Group = g.newGroup2(TokenGroupMapKey, newTk, tk)
		default:
			ret = append(ret, tk)
		}
	}
	return ret, nil
}

func (g *grouper) createMapKeyValueTokenGroups(tokens []*Token) []*Token {
	ret := g.out(len(tokens))
	for i := 0; i < len(tokens); i++ {
		tk := tokens[i]
		switch tk.GroupType() {
		case TokenGroupMapKey:
			if len(tokens) <= i+1 {
				ret = append(ret, tk)
				continue
			}
			valueTk := tokens[i+1]
			if tk.Line() != valueTk.Line() {
				ret = append(ret, tk)
				continue
			}
			if valueTk.GroupType() == TokenGroupAnchorName {
				ret = append(ret, tk)
				continue
			}
			if valueTk.Type() == token.TagType && valueTk.GroupType() != TokenGroupScalarTag {
				ret = append(ret, tk)
				continue
			}

			if isScalarType(valueTk) || valueTk.Type() == token.TagType {
				ret = append(ret, g.group2(TokenGroupMapKeyValue, tk, valueTk))
				i++
			} else {
				ret = append(ret, tk)
				continue
			}
		default:
			ret = append(ret, tk)
		}
	}
	return ret
}

func (g *grouper) createDirectiveTokenGroups(tokens []*Token) ([]*Token, error) {
	ret := g.out(len(tokens))
	for i := 0; i < len(tokens); i++ {
		tk := tokens[i]
		switch tk.Type() {
		case token.DirectiveType:
			if i+1 >= len(tokens) {
				return nil, errors.ErrSyntax("undefined directive value", tk.RawToken())
			}
			directiveName := g.group2(TokenGroupDirectiveName, tk, tokens[i+1])
			i++
			var valueTks []*Token
			for j := i + 1; j < len(tokens); j++ {
				if tokens[j].Line() != tk.Line() {
					break
				}
				valueTks = append(valueTks, tokens[j])
				i++
			}
			// A directive may be followed by comment lines before the '---' that
			// opens the document. They belong to neither, and are the reason a
			// perfectly ordinary "%YAML 1.2" with a note above the header was
			// refused whenever comments were being parsed.
			var comments []*Token
			for j := i + 1; j < len(tokens) && tokens[j].Type() == token.CommentType; j++ {
				comments = append(comments, tokens[j])
				i++
			}

			if i+1 >= len(tokens) || tokens[i+1].Type() != token.DocumentHeaderType {
				return nil, errors.ErrSyntax("unexpected directive value. document not started", tk.RawToken())
			}
			if len(valueTks) != 0 {
				ret = append(ret, g.group(TokenGroupDirective, append([]*Token{directiveName}, valueTks...)))
			} else {
				ret = append(ret, directiveName)
			}
			ret = append(ret, comments...)
		default:
			ret = append(ret, tk)
		}
	}
	return ret, nil
}

func (g *grouper) createDocumentTokens(tokens []*Token) ([]*Token, error) {
	var ret []*Token
	for i := 0; i < len(tokens); i++ {
		tk := tokens[i]
		switch tk.Type() {
		case token.DocumentHeaderType:
			if i != 0 {
				ret = append(ret, g.group(TokenGroupNone, tokens[:i]))
			}
			if i+1 == len(tokens) {
				// if current token is last token, add DocumentHeader only tokens to ret.
				return append(ret, g.group1(TokenGroupDocument, tk)), nil
			}
			if tokens[i+1].Type() == token.DocumentHeaderType {
				// One "---" straight after another: this document holds
				// nothing. It is a document all the same, and so is everything
				// after it -- stopping here returned the empty one and dropped
				// the rest of the stream without a word.
				rest, err := g.createDocumentTokens(tokens[i+1:])
				if err != nil {
					return nil, err
				}

				empty := g.group1(TokenGroupDocument, tk)

				return append(append(ret, empty), rest...), nil
			}
			if tokens[i].Line() == tokens[i+1].Line() {
				switch tokens[i+1].GroupType() {
				case TokenGroupMapKey, TokenGroupMapKeyValue:
					return nil, errors.ErrSyntax("value cannot be placed after document separator", tokens[i+1].RawToken())
				}
				switch tokens[i+1].Type() {
				case token.SequenceEntryType:
					return nil, errors.ErrSyntax("value cannot be placed after document separator", tokens[i+1].RawToken())
				}
			}
			tks, err := g.createDocumentTokens(tokens[i+1:])
			if err != nil {
				return nil, err
			}
			if len(tks) != 0 {
				tks[0].SetGroupType(TokenGroupDocument)
				var pair [2]*Token
				tks[0].Group.set(TokenGroupDocument, append([]*Token{tk}, tks[0].Group.Members(&pair)...))
				return append(ret, tks...), nil
			}
			return append(ret, g.group1(TokenGroupDocument, tk)), nil
		case token.DocumentEndType:
			if i != 0 {
				ret = append(ret, g.group(TokenGroupDocument, tokens[0:i+1]))
			}
			if i+1 == len(tokens) {
				return ret, nil
			}
			if tokens[i].Line() == tokens[i+1].Line() {
				// "..." ends the document and takes the rest of its line: only
				// a comment may follow it there. On the next line a new
				// document begins, and it may be a bare one -- a scalar, or a
				// block scalar as in the spec's own bare-documents example.
				return nil, errors.ErrSyntax("unexpected end content", tokens[i+1].RawToken())
			}

			tks, err := g.createDocumentTokens(tokens[i+1:])
			if err != nil {
				return nil, err
			}
			return append(ret, tks...), nil
		}
	}
	return append(ret, g.group(TokenGroupDocument, tokens)), nil
}

func isScalarType(tk *Token) bool {
	switch tk.GroupType() {
	case TokenGroupMapKey, TokenGroupMapKeyValue:
		return false
	}
	typ := tk.Type()
	return typ == token.AnchorType ||
		typ == token.AliasType ||
		typ == token.LiteralType ||
		typ == token.FoldedType ||
		typ == token.NullType ||
		typ == token.ImplicitNullType ||
		typ == token.BoolType ||
		typ == token.IntegerType ||
		typ == token.BinaryIntegerType ||
		typ == token.OctetIntegerType ||
		typ == token.HexIntegerType ||
		typ == token.FloatType ||
		typ == token.InfinityType ||
		typ == token.NanType ||
		typ == token.StringType ||
		typ == token.SingleQuoteType ||
		typ == token.DoubleQuoteType
}

// groupExplicitKeyBody applies to an explicit key's body the grouping passes it
// would otherwise miss.
//
// Absorbing the body into the key's own group hides it from the passes that run
// after this one, and a key is a document in miniature: "? []: x" has a mapping
// for its key, whose own ':' has to be paired here or it is silently dropped.
// The passes before this one -- literals, anchors, tags -- have already run over
// these tokens, so only the mapping ones are needed.
func (g *grouper) groupExplicitKeyBody(body []*Token) ([]*Token, error) {
	// Called from inside createMapKeyByMappingKey, which is filling a buffer of
	// its own and reading another.
	g.nested++
	defer func() { g.nested-- }()

	grouped, err := g.createMapKeyByMappingValue(body)
	if err != nil {
		return nil, err
	}

	return g.createMapKeyValueTokenGroups(grouped), nil
}

// explicitKeyEnd returns the index just past the body of the explicit key
// introduced by the '?' at tokens[i].
//
// In block context the body is everything indented deeper than the '?' itself,
// and nothing else bounds it -- in particular a ':' on the same line does not.
// "? []: x" has the mapping {[]: x} for its key and no value at all, which is
// what the test suite records for it, so taking the whole indented run is both
// simpler and right.
//
// A flow collection is not indentation-sensitive, so there the body runs to the
// punctuation that ends it: its ':', a ',', or the bracket closing the
// collection it sits in.
func explicitKeyEnd(tokens []*Token, i int, inFlow bool) int {
	if inFlow {
		return explicitFlowKeyEnd(tokens, i)
	}

	col := tokens[i].Column()

	j := i + 1
	for ; j < len(tokens); j++ {
		if tokens[j].Column() <= col {
			break
		}
	}

	return j
}

func explicitFlowKeyEnd(tokens []*Token, i int) int {
	var depth int

	j := i + 1
	for ; j < len(tokens); j++ {
		switch tokens[j].Type() {
		case token.MappingStartType, token.SequenceStartType:
			depth++
		case token.MappingEndType, token.SequenceEndType:
			if depth == 0 {
				return j
			}
			depth--
		case token.MappingValueType, token.CollectEntryType:
			if depth == 0 {
				return j
			}
		}
	}

	return j
}

// hasNoKey reports whether the ':' at tokens[i] has no key in front of it.
//
// Three ways that happens. There is nothing before it at all; what is before it
// is punctuation that cannot be a key; or -- in block context only -- the
// candidate sits on an earlier line, and an implicit key must share its line
// with its ':'. A flow collection is not line-sensitive, so the last rule does
// not apply inside one, and an explicit "?" key is exempt everywhere: naming
// the key separately is precisely what "?" is for.
func hasNoKey(tokens []*Token, i int, inFlow bool) bool {
	j := keyCandidateIndex(tokens, i)
	if j < 0 {
		return true
	}

	candidate := tokens[j]
	if precedesAbsentKey(candidate) {
		return true
	}
	if inFlow || candidate.Group != nil {
		return false
	}

	return keyEndLine(candidate) != tokens[i].Line()
}

// keyEndLine reports the line on which a key token ends.
//
// A plain scalar may span several lines and its position records where it
// starts, so the line that matters for the implicit-key rule has to be
// computed. Whitespace on either side of the origin belongs to the neighboring
// tokens rather than to this one -- a trailing newline in particular would
// otherwise push the end line one past where the token really finishes.
func keyEndLine(tk *Token) int {
	raw := tk.RawToken()
	if raw == nil {
		return tk.Line()
	}

	return tk.Line() + strings.Count(strings.Trim(raw.Origin, " \r\n"), "\n")
}

// closesFlowCollection reports whether tk ends a flow collection.
func closesFlowCollection(tk *Token) bool {
	return tk.Type() == token.MappingEndType || tk.Type() == token.SequenceEndType
}

// flowCollectionStart finds where the flow collection ending at the last token
// of ret begins, so the whole of it can be taken as a mapping key. It reports
// -1 when the brackets do not balance.
func flowCollectionStart(ret []*Token) int {
	var depth int
	for i := len(ret) - 1; i >= 0; i-- {
		if ret[i].GroupType() != TokenGroupNone {
			// A group is balanced within itself, and reports the type of the
			// token it opens with. A key already grouped as "[b]: d" would
			// otherwise read as one more '[' with no ']' to match it.
			continue
		}
		switch ret[i].Type() {
		case token.MappingEndType, token.SequenceEndType:
			depth++
		case token.MappingStartType, token.SequenceStartType:
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// withKeyProperties widens a key that starts at start to take in the anchors
// and tags written before it on the same line.
//
// They are the key's, not the mapping's: the anchor of "&k [a, b]: v" names the
// sequence used as the key. Left outside, it was read as naming the mapping the
// entry belongs to -- silently, and as a second anchor on that mapping when the
// mapping already had one of its own.
func withKeyProperties(ret []*Token, start int) int {
	for start > 0 && ret[start-1].Line() == ret[start].Line() {
		prev := ret[start-1]
		if prev.GroupType() != TokenGroupAnchorName && prev.Type() != token.TagType {
			break
		}
		start--
	}

	return start
}

// precedesAbsentKey reports whether tk is punctuation that cannot itself be a
// key and does not close a collection, so that a ':' right after it has no key
// at all.
//
// The exclusion of '}' and ']' is the point of this being separate from
// isNotMapKeyType: a ':' after them has a key -- the collection that just
// closed -- which is a different feature, and still unsupported. Treating those
// as absent keys would turn a clear "found an invalid key for this map" into a
// null key and a misleading error further on.
func precedesAbsentKey(tk *Token) bool {
	switch tk.Type() {
	case token.CollectEntryType,
		token.MappingStartType,
		token.SequenceStartType,
		token.SequenceEntryType,
		token.MappingValueType,
		token.DirectiveType,
		token.DocumentHeaderType,
		token.DocumentEndType:
		return true
	default:
		return false
	}
}

// keyCandidateIndex finds what would be the key of the ':' at tokens[i],
// skipping comments -- a comment is never a key, and one may sit between an
// explicit "?" key and its ':'. It reports -1 when there is nothing before it.
func keyCandidateIndex(tokens []*Token, i int) int {
	for j := i - 1; j >= 0; j-- {
		if tokens[j].Type() != token.CommentType {
			return j
		}
	}

	return -1
}

// implicitNullKeyToken builds the null node standing in for an absent mapping
// key, positioned where the key would have been -- immediately before its ':'.
func (g *grouper) implicitNullKeyToken(colon *Token) *Token {
	pos := colon.RawToken().Position
	tk := token.New("null", "null", pos)
	tk.Type = token.ImplicitNullType

	wrapped := g.token()
	wrapped.Token = tk

	return wrapped
}

func isNotMapKeyType(tk *Token) bool {
	typ := tk.Type()
	return typ == token.DirectiveType ||
		typ == token.DocumentHeaderType ||
		typ == token.DocumentEndType ||
		typ == token.CollectEntryType ||
		typ == token.MappingStartType ||
		typ == token.MappingValueType ||
		typ == token.MappingEndType ||
		typ == token.SequenceStartType ||
		typ == token.SequenceEntryType ||
		typ == token.SequenceEndType
}

// isFlowType reports whether a token is punctuation a tag cannot be grouped
// with, because the token belongs to the collection around the tag rather than
// naming what the tag is on.
//
// The two closers and the ',' are here for the same reason as the openers: "[!]"
// is the non-specific tag on the empty node followed by the closer, and grouping
// the two swallowed the ']' -- the sequence then ran to the end of the stream
// looking for it.
func isFlowType(tk *Token) bool {
	typ := tk.Type()
	return typ == token.MappingStartType ||
		typ == token.MappingEndType ||
		typ == token.SequenceStartType ||
		typ == token.SequenceEndType ||
		typ == token.SequenceEntryType ||
		typ == token.CollectEntryType
}
