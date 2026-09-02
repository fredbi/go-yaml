// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"os"
	"strings"

	yamlerrors "github.com/go-openapi/go-yaml/errors"
	"github.com/go-openapi/go-yaml/token"
)

type tokenGroupType uint8

const (
	TokenGroupNone tokenGroupType = iota
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

func (t tokenGroupType) String() string {
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

// tapeToken is one token as the grouping sees it: the token the scanner read, and
// the group it was joined into where a pass joined it.
//
// The scanner's token is held here by value rather than by pointer, so a token
// of a document is one thing on the tape and not two. The tree points into it
// with RawToken, which stays good for as long as the chunk it sits in does.
//
// A pass turns a token into a group in place, by hanging the group on it. Group
// therefore answers before raw does: raw is what this token was read as, and
// Group what it became.
type tapeToken struct {
	raw   token.Token
	Group *tokenGroup
	// seq is where this token stands on the tape, counted from the first the
	// scanner handed over. A walk tells the arena how far the descent has read
	// with it, and the arena reclaims what is behind that.
	seq int32
}

// newSynthetic returns a token holding tk, for a token the parse makes rather
// than reads: an implicit null, or one standing in for a tag's absent value.
//
// It is off the tape, so it outlives whatever the tail does. There are few of
// them and each is one small allocation.
func newSynthetic(tk *token.Token) *tapeToken {
	if tk == nil {
		return nil
	}

	return &tapeToken{raw: *tk}
}

// Raw fills this token in from what the scanner read.
func (t *tapeToken) Raw(tk token.Token, seq int) {
	t.raw, t.Group, t.seq = tk, nil, int32(seq)
}

// Seq returns where this token stands on the tape.
//
// A grouping pass turns a token into a group by hanging the group on it and
// clearing the raw token, so a group token keeps the place the token it was
// made from held -- which is where the group begins. Its own seq is therefore
// the answer, and the group is only read where a token was made by the grouping
// rather than drawn from the stream, which leaves seq at zero.
func (t *tapeToken) Seq() int32 {
	if t == nil {
		return 0
	}
	if t.seq > 0 {
		return t.seq
	}
	if t.Group != nil {
		return t.Group.First().Seq()
	}

	return 0
}

func (t *tapeToken) RawToken() *token.Token {
	if t == nil {
		return nil
	}
	if t.Group != nil {
		return t.Group.RawToken()
	}

	return &t.raw
}

func (t *tapeToken) Type() token.Type {
	if t == nil {
		return 0
	}
	if t.Group != nil {
		return t.Group.TokenType()
	}

	return t.raw.Type
}

func (t *tapeToken) GroupType() tokenGroupType {
	if t == nil {
		return TokenGroupNone
	}
	if t.Group == nil {
		return TokenGroupNone
	}

	return t.Group.Type
}

func (t *tapeToken) Line() int {
	if t == nil {
		return 0
	}
	if t.Group != nil {
		return t.Group.Line()
	}

	return int(t.raw.Position.Line)
}

func (t *tapeToken) Column() int {
	if t == nil {
		return 0
	}
	if t.Group != nil {
		return t.Group.Column()
	}

	return int(t.raw.Position.Column)
}

func (t *tapeToken) SetGroupType(typ tokenGroupType) {
	if t.Group == nil {
		return
	}
	t.Group.Type = typ
}

func (t *tapeToken) Dump() {
	ctx := new(groupTokenRenderContext)
	if t.Group == nil {
		fmt.Fprint(os.Stdout, t.raw.Value)

		return
	}
	t.Group.dump(ctx)
	fmt.Fprintf(os.Stdout, "\n")
}

func (t *tapeToken) dump(ctx *groupTokenRenderContext) {
	if t.Group == nil {
		fmt.Fprint(os.Stdout, t.raw.Value)

		return
	}
	t.Group.dump(ctx)
}

type groupTokenRenderContext struct {
	num int
}

// tokenGroup is a run of tokens the parser reads as one.
//
// Nearly every group holds exactly two members -- a key and its ':', a key
// group and its value, an anchor and what it names -- so the two are held in
// the group itself. Only a document, an explicit key and a directive hold more,
// and those keep a slice: 446 of the 256,848 groups the corpus builds, 0.17%.
// Holding the common pair inline saves the run of pointers a slice would need.
type tokenGroup struct {
	a, b *tapeToken
	// more holds the members where there are more than two, and a and b are
	// then unused. It is a pointer to a slice rather than a slice so that the
	// group stays 32 bytes.
	more *[]*tapeToken
	Type tokenGroupType
	n    uint8
}

// newTokenGroup returns a group of typ over tks, on the heap. The grouper hands
// its own out from blocks; this is for the few the parser builds itself.
func newTokenGroup(typ tokenGroupType, tks []*tapeToken) *tokenGroup {
	g := new(tokenGroup)
	g.set(typ, tks)

	return g
}

// set fills g with the members tks, keeping two of them in the group itself.
func (g *tokenGroup) set(typ tokenGroupType, tks []*tapeToken) {
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
func (g *tokenGroup) Len() int {
	if g.more != nil {
		return len(*g.more)
	}

	return int(g.n)
}

// At returns the i'th member.
func (g *tokenGroup) At(i int) *tapeToken {
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
func (g *tokenGroup) Members(pair *[2]*tapeToken) []*tapeToken {
	if g.more != nil {
		return *g.more
	}
	pair[0], pair[1] = g.a, g.b

	return pair[:g.n]
}

func (g *tokenGroup) First() *tapeToken {
	if g.Len() == 0 {
		return nil
	}

	return g.At(0)
}

func (g *tokenGroup) Last() *tapeToken {
	n := g.Len()
	if n == 0 {
		return nil
	}

	return g.At(n - 1)
}

func (g *tokenGroup) dump(ctx *groupTokenRenderContext) {
	num := ctx.num
	fmt.Fprint(os.Stdout, colorize(num, "("))
	ctx.num++
	for i := range g.Len() {
		g.At(i).dump(ctx)
	}
	fmt.Fprint(os.Stdout, colorize(num, ")"))
}

func (g *tokenGroup) RawToken() *token.Token {
	if g.Len() == 0 {
		return nil
	}

	return g.At(0).RawToken()
}

func (g *tokenGroup) Line() int {
	if g.Len() == 0 {
		return 0
	}

	return g.At(0).Line()
}

func (g *tokenGroup) Column() int {
	if g.Len() == 0 {
		return 0
	}

	return g.At(0).Column()
}

func (g *tokenGroup) TokenType() token.Type {
	if g.Len() == 0 {
		return 0
	}

	return g.At(0).Type()
}

// grouper runs the passes that turn a flat token stream into grouped tokens.
//
// It hands out the [tapeToken], [tokenGroup] and []*tapeToken that grouping needs from
// blocks rather than one allocation each. A stream of N tokens groups into
// roughly N wrappers, groups and slices, and those three were the largest
// allocation sites of a parse by count.
type grouper struct {
	// The state a pass holds between two tokens lives here rather than in the
	// pass's own closure, so that a pass may be run over one run of tokens,
	// stopped, and run again over the next with what it was holding still in
	// hand. A group straddling the join is then grouped as one.
	//
	// ending says the run in hand is the last, so a pass hands over whatever it
	// still holds. Between two runs it is false and a pass keeps hold.
	ending bool

	// ⚠️ Only a pass that runs once may keep its state here.
	// groupMapKeysByValue and groupExplicitKeys re-enter themselves --
	// groupExplicitKeyBody groups an explicit key's body with a nested run of
	// the same passes on this grouper -- so a nested run would write over what
	// the outer one was holding. Their state stays in their own closures until
	// there is a stack for it, one frame per depth of nesting.

	lineComment *tapeToken // the token whose line a comment may close

	blockHeader *tapeToken     // a "|" or ">", waiting for its content
	blockType   tokenGroupType // which of the two it is

	anchor *tapeToken // a "&", waiting for its name
	name   *tapeToken // an anchor name, waiting to see what it names
	alias  *tapeToken // a "*", waiting for its name

	tag    *tapeToken // a tag, waiting to see what it tags
	tagged *tapeToken // an anchor name, waiting to see whether a tagged scalar follows

	// explicit is what groupExplicitKeys holds while it reads the body naming
	// a '?' key, and keys what groupMapKeysByValue holds while it waits to see
	// whether a ':' follows.
	explicit explicitKey
	keys     keyWindow

	keyed *tapeToken // a map key, waiting to see whether its value follows

	// directive is what groupDirectives holds while it reads a '%' line.
	directive directiveState

	// heldHigh is the most tokens a pass has held at once, which is how far
	// ahead of the descent the grouping has to keep the tape.
	heldHigh int

	tokens []tapeToken
	groups []tokenGroup
	// passA and passB are the two buffers the grouping passes write into. A
	// pass reads one and writes the other, so the nine of them cost two
	// allocations between them rather than one apiece.
	passA, passB []*tapeToken
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
	lineComments map[*tapeToken]*token.Token
	// block is how many of each one allocation covers.
	block int
}

// setLineComment records that comment closes the line tk stands on.
func (g *grouper) setLineComment(tk *tapeToken, comment *token.Token) {
	if g.lineComments == nil {
		g.lineComments = make(map[*tapeToken]*token.Token)
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
// that, so a pass added to createGroupedTokens goes before that one.
// out returns the buffer the next pass writes into.
//
// The passes of one run hand a slice from one to the next, so two buffers go
// round: a pass reads the one before it and fills the other, and the pass after
// it fills the first again. What a pass two steps back wrote is finished with
// by then.
func (g *grouper) out(n int) []*tapeToken {
	if g.nested > 0 {
		// A pass running inside another takes a buffer of its own: both of the
		// grouper's are in hand, one being read and one being filled.
		return make([]*tapeToken, 0, n)
	}

	g.writeB = !g.writeB

	buf := &g.passA
	if g.writeB {
		buf = &g.passB
	}
	if cap(*buf) < n {
		*buf = make([]*tapeToken, 0, n)
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

func (g *grouper) token() *tapeToken {
	if len(g.tokens) == 0 {
		g.tokens = make([]tapeToken, g.block)
	}
	tk := &g.tokens[0]
	g.tokens = g.tokens[1:]

	return tk
}

// newGroup1 and newGroup2 return a group over one and two tokens, taken from
// the block and filled without a list.
func (g *grouper) newGroup1(typ tokenGroupType, a *tapeToken) *tokenGroup {
	grp := g.nextGroup()
	grp.Type, grp.a, grp.b, grp.more, grp.n = typ, a, nil, nil, 1

	return grp
}

func (g *grouper) newGroup2(typ tokenGroupType, a, b *tapeToken) *tokenGroup {
	grp := g.nextGroup()
	grp.Type, grp.a, grp.b, grp.more, grp.n = typ, a, b, nil, 2

	return grp
}

// nextGroup returns the next unused group of the block.
func (g *grouper) nextGroup() *tokenGroup {
	if len(g.groups) == 0 {
		g.groups = make([]tokenGroup, g.block)
	}
	grp := &g.groups[0]
	g.groups = g.groups[1:]

	return grp
}

// newGroup returns a group of typ over tks.
func (g *grouper) newGroup(typ tokenGroupType, tks []*tapeToken) *tokenGroup {
	grp := g.nextGroup()
	grp.set(typ, tks)

	return grp
}

// group returns a token holding a group of typ over tks.
func (g *grouper) group(typ tokenGroupType, tks []*tapeToken) *tapeToken {
	tk := g.token()
	tk.Group = g.newGroup(typ, tks)

	return tk
}

// group1 and group2 are group over one and two tokens, which is most of them.
// Both members are held in the group itself, so neither builds a list.
func (g *grouper) group1(typ tokenGroupType, a *tapeToken) *tapeToken {
	tk := g.token()
	tk.Group = g.newGroup1(typ, a)

	return tk
}

func (g *grouper) group2(typ tokenGroupType, a, b *tapeToken) *tapeToken {
	tk := g.token()
	tk.Group = g.newGroup2(typ, a, b)

	return tk
}

// createGroupedTokens reads the tokens of a stream into the groups the parser
// walks. Each pass takes the tokens the one before it left and groups a little
// more of them.

// attachLineComments attaches the comment closing a token's line to that token, and
// drops it from the stream.
//
// Nothing is held back. The comment arrives after the token it belongs to, and
// the attachment is recorded against the token rather than written into it, so
// the token may already have been handed on.
func (g *grouper) attachLineComments(in []*tapeToken) []*tapeToken {
	out := g.out(len(in))
	for _, tk := range in {
		if tk.Type() == token.CommentType && g.lineComment != nil && g.lineComment.Line() == tk.Line() {
			g.setLineComment(g.lineComment, tk.RawToken())

			continue
		}
		out = append(out, tk)
		g.lineComment = tk
	}

	return out
}

// groupBlockScalars joins a "|" or ">" header with the content that follows it.
//
// One token is held: the header, until the content arrives. A header ending the
// stream has no content, and the group is the header alone -- which is what
// "a: |" with nothing after it is.
func (g *grouper) groupBlockScalars(in []*tapeToken) []*tapeToken {
	out := g.out(len(in))
	for _, tk := range in {
		if g.blockHeader != nil {
			// Whatever follows the header is its content, read as it stands: a
			// second "|" is content, not another header.
			out = append(out, g.group2(g.blockType, g.blockHeader, tk))
			g.blockHeader = nil

			continue
		}

		switch tk.Type() {
		case token.LiteralType:
			g.blockHeader, g.blockType = tk, TokenGroupLiteral
		case token.FoldedType:
			g.blockHeader, g.blockType = tk, TokenGroupFolded
		default:
			out = append(out, tk)
		}
	}

	// A header ending the stream has no content, so the group is the header
	// alone. Between two runs it waits for the next one.
	if g.ending && g.blockHeader != nil {
		out = append(out, g.group1(g.blockType, g.blockHeader))
		g.blockHeader = nil
	}

	return out
}

// groupAnchors joins "&" with the name after it, that name with what it names,
// and "*" with the name after it.
//
// Two tokens are held at the most: the "&" until its name arrives, and then the
// name group until the token after it says whether the anchor names a scalar on
// the same line or an empty node.
func (g *grouper) groupAnchors(in []*tapeToken) []*tapeToken {
	out := g.out(len(in))
	for _, tk := range in {
		switch {
		case g.alias != nil:
			out = append(out, g.group2(TokenGroupAlias, g.alias, tk))
			g.alias = nil

			continue
		case g.anchor != nil:
			g.name, g.anchor = g.group2(TokenGroupAnchorName, g.anchor, tk), nil

			continue
		case g.name != nil:
			sameLine := g.name.Line() == tk.Line()
			if sameLine && tk.Type() == token.SequenceEntryType {
				g.fail(yamlerrors.NewSyntax("sequence entries are not allowed after anchor on the same line", tk.RawToken()))

				return out
			}
			if sameLine && isScalarType(tk) {
				out = append(out, g.group2(TokenGroupAnchor, g.name, tk))
				g.name = nil

				continue
			}
			// The anchor names the empty node, and tk is read as any other
			// token would be.
			out = append(out, g.name)
			g.name = nil
		}

		switch tk.Type() {
		case token.AnchorType:
			g.anchor = tk
		case token.AliasType:
			g.alias = tk
		default:
			out = append(out, tk)
		}
	}

	if !g.ending {
		return out
	}

	switch {
	case g.anchor != nil:
		g.fail(yamlerrors.NewSyntax("undefined anchor name", g.anchor.RawToken()))
	case g.alias != nil:
		g.fail(yamlerrors.NewSyntax("undefined alias name", g.alias.RawToken()))
	case g.name != nil:
		// An anchor with nothing after it names the empty node. The parser
		// supplies that null; there is nothing to group here.
		out = append(out, g.name)
		g.name = nil
	}

	return out
}

// groupScalarTags joins a tag with the scalar it tags.
//
// One token is held: the tag, until the token after it says whether it tags
// that one or stands on its own. A tag on its own is left in the stream and the
// parser reads what it tags from there -- a tag on its own line, or one in
// front of a collection.
func (g *grouper) groupScalarTags(in []*tapeToken) []*tapeToken {
	out := g.out(len(in))
	for _, tk := range in {
		if g.tag != nil {
			grouped, ok := g.taggedScalar(g.tag, tk)
			if !ok {
				return out
			}
			if grouped != nil {
				out = append(out, grouped)
				g.tag = nil

				continue
			}
			// The tag stands on its own, and tk is read as any other token
			// would be -- including as the next tag.
			out = append(out, g.tag)
			g.tag = nil
		}

		if tk.Type() == token.TagType {
			g.tag = tk

			continue
		}
		out = append(out, tk)
	}

	if g.ending && g.tag != nil {
		out = append(out, g.tag)
		g.tag = nil
	}

	return out
}

// taggedScalar returns the group joining tag with next, or nil where the tag
// stands on its own. It reports false where the document is refused.
//
// A tag never reaches past its own line, and never takes an anchor name: the
// anchor is what holds the tag, and groupAnchorsWithScalarTags joins those.
func (g *grouper) taggedScalar(tag, next *tapeToken) (*tapeToken, bool) {
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
			g.fail(yamlerrors.NewSyntax("could not find merge key", next.RawToken()))

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
func (g *grouper) groupAnchorsWithScalarTags(in []*tapeToken) []*tapeToken {
	out := g.out(len(in))
	for _, tk := range in {
		if g.tagged != nil {
			if g.tagged.Line() == tk.Line() && tk.GroupType() == TokenGroupScalarTag {
				out = append(out, g.group2(TokenGroupAnchor, g.tagged, tk))
				g.tagged = nil

				continue
			}
			// The anchor names something else, or the empty node, and tk is
			// read as any other token would be.
			out = append(out, g.tagged)
			g.tagged = nil
		}

		if tk.GroupType() == TokenGroupAnchorName {
			g.tagged = tk

			continue
		}
		out = append(out, tk)
	}

	if g.ending && g.tagged != nil {
		// An anchor with nothing after it names the empty node. The parser
		// supplies that null; there is nothing to group here.
		out = append(out, g.tagged)
		g.tagged = nil
	}

	return out
}

// directiveState is what groupDirectives holds while it reads a '%' line: the
// '%' itself, the group it makes with its name, and what follows on that line.
type directiveState struct {
	head     *tapeToken // a '%', while its name and values are read
	name     *tapeToken // the '%' joined with its name
	values   []*tapeToken
	comments []*tapeToken
}

// explicitKey is what groupExplicitKeys holds between two tokens: the '?' and
// the body read so far, with the depths that say where the body ends.
type explicitKey struct {
	flowDepth int
	key       *tapeToken // a '?', while the body naming its key is read
	keyColumn int
	keyInFlow bool
	bodyDepth int
	body      []*tapeToken
}

// groupExplicitKeys joins a '?' with the body that names its key.
//
// The body is held until the token that ends it arrives: a token at or left of
// the '?' in block context, and the ':' or ',' or bracket that closes the entry
// in flow context. That is the widest window of the ten passes, and it is the
// key itself -- the parser is about to read it.
func (g *grouper) groupExplicitKeys(in []*tapeToken) []*tapeToken {
	out := g.out(len(in))
	{
		emit := func() bool {
			grouped, err := g.groupExplicitKeyBody(g.explicit.body)
			if err != nil {
				g.fail(err)

				return false
			}

			// A '?' with nothing after it opens an entry whose g.explicit.key is e-node,
			// which is what "? \n" and "?\n: v\n" are. The group holds the
			// indicator alone and the parser supplies the null.
			members := []*tapeToken{g.explicit.key}
			if len(grouped) == 0 {
				members = append(members, g.implicitNullKeyToken(g.explicit.key))
			}
			members = append(members, grouped...)

			g.explicit.key, g.explicit.body = nil, g.explicit.body[:0]

			out = append(out, g.group(TokenGroupMapKey, members))

			return true
		}

		for _, tk := range in {
			if g.explicit.key != nil {
				if !endsExplicitKeyBody(tk, g.explicit.keyColumn, g.explicit.keyInFlow, &g.explicit.bodyDepth) {
					g.explicit.body = append(g.explicit.body, tk)

					continue
				}
				if !emit() {
					return out
				}
				// The token that ended the g.explicit.body is not part of it, and is read
				// as any other token would be.
			}

			switch tk.Type() {
			case token.MappingStartType, token.SequenceStartType:
				g.explicit.flowDepth++
				out = append(out, tk)
			case token.MappingEndType, token.SequenceEndType:
				if g.explicit.flowDepth > 0 {
					g.explicit.flowDepth--
				}
				out = append(out, tk)
			case token.MappingKeyType:
				g.explicit.key, g.explicit.keyColumn, g.explicit.keyInFlow, g.explicit.bodyDepth = tk, tk.Column(), g.explicit.flowDepth > 0, 0
			default:
				out = append(out, tk)
			}
		}

		if g.ending && g.explicit.key != nil {
			emit()
		}
	}

	return out
}

// endsExplicitKeyBody reports whether tk stands past the body of the explicit
// key introduced by a '?' at keyColumn, and counts the flow collections opened
// inside that body.
//
// In block context the body is everything indented deeper than the '?' itself,
// and nothing else bounds it -- in particular a ':' on the same line does not.
// "? []: x" has the mapping {[]: x} for its key and no value at all, which is
// what the test suite records for it.
//
// A flow collection is not indentation-sensitive, so there the body runs to the
// punctuation that ends it: its ':', a ',', or the bracket closing the
// collection it sits in.
func endsExplicitKeyBody(tk *tapeToken, keyColumn int, inFlow bool, depth *int) bool {
	if !inFlow {
		return tk.Column() <= keyColumn
	}

	switch tk.Type() {
	case token.MappingStartType, token.SequenceStartType:
		*depth++
	case token.MappingEndType, token.SequenceEndType:
		if *depth == 0 {
			return true
		}
		*depth--
	case token.MappingValueType, token.CollectEntryType:
		if *depth == 0 {
			return true
		}
	}

	return false
}

// keyWindow holds the tokens a map key could still be made from. Everything
// before it has been handed on.
//
// The window is one token wide most of the time -- the scalar in front of a
// ':' -- and widens to hold a flow collection while one is open, because
// "[a, b]: v" keys on the whole collection.
type keyWindow struct {
	held []*tapeToken
	// openers holds the index in held of each flow collection still open,
	// outermost first, and seq says which of them are sequences. A pair written
	// directly inside a sequence is an implicit key and has to fit on one line
	// with its ':'; inside a mapping the same pair may span lines.
	openers []int
	seq     []bool
}

// keepFrom is where the window has to start for a ':' arriving next to find its
// key. Everything before it can be handed on.
func (w *keyWindow) keepFrom() int {
	if len(w.openers) > 0 {
		// A collection still open may yet close and stand as a key.
		return withKeyProperties(w.held, w.openers[0])
	}

	last := lastContentIndex(w.held)
	if last < 0 {
		return len(w.held)
	}
	if closesFlowCollection(w.held[last]) {
		start := flowCollectionStart(w.held[:last+1])
		if start < 0 {
			return last
		}

		return withKeyProperties(w.held, start)
	}

	return last
}

// release hands on the tokens that can no longer take part in a key.
// release hands on what the window no longer has to keep, appending it to out.
func (w *keyWindow) release(out []*tapeToken) []*tapeToken {
	keep := w.keepFrom()
	out = append(out, w.held[:keep]...)

	w.held = append(w.held[:0], w.held[keep:]...)
	for i := range w.openers {
		w.openers[i] -= keep
	}

	return out
}

// lastContentIndex is where the last token of the window that is not a comment
// stands. A comment may sit between a key and its ':' without parting them.
func lastContentIndex(held []*tapeToken) int {
	for i := len(held) - 1; i >= 0; i-- {
		if held[i].Type() != token.CommentType {
			return i
		}
	}

	return -1
}

// groupMapKeysByValue joins a key with the ':' that follows it.
//
// The key is held rather than handed on and rewritten where it stands, which is
// what the pass did while it read a slice: in a stream the token would be gone
// by the time its ':' arrived.
func (g *grouper) groupMapKeysByValue(in []*tapeToken) []*tapeToken {
	out := g.out(len(in))

	// The outer run keeps its window on the grouper, so it survives the end of
	// one run of tokens and is still holding when the next begins. A nested run
	// -- groupExplicitKeyBody grouping one explicit key's body -- reads that
	// body from end to end and takes a window of its own, or it would write
	// over what the run around it holds.
	w := &g.keys
	if g.nested > 0 {
		w = new(keyWindow)
	}

	for _, tk := range in {
		switch tk.Type() {
		case token.MappingStartType, token.SequenceStartType:
			w.openers = append(w.openers, len(w.held))
			w.seq = append(w.seq, tk.Type() == token.SequenceStartType)
			w.held = append(w.held, tk)
		case token.MappingEndType, token.SequenceEndType:
			if len(w.openers) > 0 {
				w.openers = w.openers[:len(w.openers)-1]
				w.seq = w.seq[:len(w.seq)-1]
			}
			w.held = append(w.held, tk)
		case token.MappingValueType:
			if !g.keyBefore(w, tk) {
				return out
			}
		default:
			w.held = append(w.held, tk)
		}

		if len(w.held) > g.heldHigh {
			g.heldHigh = len(w.held)
		}
		out = w.release(out)
	}

	// The window holds what a ':' arriving next would need. Between two runs
	// that ':' may still be coming, so the outer window empties only at the
	// end; a nested run always ends with its body.
	if !g.ending && g.nested == 0 {
		return out
	}

	out = append(out, w.held...)
	w.held = w.held[:0]

	return out
}

// keyBefore reads the key the ':' belongs to out of the window, and puts the
// group it makes back there. It reports false where the document is refused.
func (g *grouper) keyBefore(w *keyWindow, tk *tapeToken) bool {
	inFlow := len(w.openers) > 0
	last := lastContentIndex(w.held)

	if w.hasNoKey(last, tk, inFlow) {
		// The key is absent: ": value", "- :", "{ : }", "{a: 1, : 2}". YAML 1.2
		// allows it, and an absent key is the null node -- so there is nothing
		// to reject here, only a node to supply.
		w.held = append(w.held, g.group2(TokenGroupMapKey, g.implicitNullKeyToken(tk), tk))

		return true
	}

	key := w.held[last]
	if closesFlowCollection(key) {
		// The key is the flow collection that just closed, so it has to be
		// taken whole: "[a, b]: v" keys on the sequence, not on the ']' that
		// ends it.
		start := flowCollectionStart(w.held[:last+1])
		if start < 0 {
			g.fail(yamlerrors.NewSyntax("found an invalid key for this map", tk.RawToken()))

			return false
		}
		start = withKeyProperties(w.held, start)
		if w.held[start].Line() != key.Line() {
			// An implicit key has to be a single-line node, so a collection
			// spanning lines cannot be one.
			g.fail(yamlerrors.NewSyntax("map key definition includes an implicit line break", tk.RawToken()))

			return false
		}
		if inFlow && w.seq[len(w.seq)-1] && key.Line() != tk.Line() {
			// Directly inside a sequence the ':' is part of that one line too.
			// Inside a mapping it is separation like any other, and may follow
			// on the next line.
			g.fail(yamlerrors.NewSyntax("map key definition includes an implicit line break", tk.RawToken()))

			return false
		}

		keyTokens := append(append([]*tapeToken{}, w.held[start:]...), tk)
		w.held = append(w.held[:start], g.group(TokenGroupMapKey, keyTokens))

		return true
	}

	if isNotMapKeyType(key) {
		g.fail(yamlerrors.NewSyntax("found an invalid key for this map", tk.RawToken()))

		return false
	}

	// The key stays where it stands in the window and becomes the group, so
	// that the comments written between it and its ':' keep their place after
	// it.
	held := g.token()
	held.raw, held.Group, held.seq = key.raw, key.Group, key.seq
	key.Group = g.newGroup2(TokenGroupMapKey, held, tk)

	return true
}

// hasNoKey reports whether the ':' has no key in front of it.
//
// Three ways that happens. There is nothing before it at all; what is before it
// is punctuation that cannot be a key; or -- in block context only -- the
// candidate sits on an earlier line, and an implicit key must share its line
// with its ':'. A flow collection is not line-sensitive, so the last rule does
// not apply inside one, and an explicit "?" key is exempt everywhere: naming
// the key separately is precisely what "?" is for.
func (w *keyWindow) hasNoKey(last int, tk *tapeToken, inFlow bool) bool {
	if last < 0 {
		return true
	}

	candidate := w.held[last]
	if precedesAbsentKey(candidate) {
		return true
	}
	if inFlow || candidate.Group != nil {
		return false
	}

	return keyEndLine(candidate) != tk.Line()
}

// groupMapKeyValues joins a map key with the value written on its line.
//
// One token is held, the key, until the token after it says whether that is its
// value. A key whose value is on a later line keeps its own group, and the
// parser reads the value from the stream: "a:\n  b" is a key and a mapping, not
// a pair.
func (g *grouper) groupMapKeyValues(in []*tapeToken) []*tapeToken {
	out := g.out(len(in))
	{
		// As in groupMapKeysByValue: the outer run keeps what it holds on the
		// grouper so it survives the end of a run, and a nested one takes its
		// own.
		held := &g.keyed
		if g.nested > 0 {
			held = new(*tapeToken)
		}

		for _, tk := range in {
			if key := *held; key != nil {
				if pair := g.keyedValue(key, tk); pair != nil {
					out = append(out, pair)
					*held = nil

					continue
				}
				out = append(out, key)
				*held = nil
			}

			if tk.GroupType() == TokenGroupMapKey {
				*held = tk

				continue
			}
			out = append(out, tk)
		}

		if (g.ending || g.nested > 0) && *held != nil {
			out = append(out, *held)
			*held = nil
		}
	}

	return out
}

// keyedValue returns the group joining key with value, or nil where value is
// not the key's.
//
// A value has to stand on the key's line. An anchor name is not a value but
// what holds one, and a tag that has not been joined to a scalar is the same,
// so both leave the key on its own.
func (g *grouper) keyedValue(key, value *tapeToken) *tapeToken {
	if key.Line() != value.Line() || value.GroupType() == TokenGroupAnchorName {
		return nil
	}
	if value.Type() == token.TagType && value.GroupType() != TokenGroupScalarTag {
		return nil
	}
	if !isScalarType(value) && value.Type() != token.TagType {
		return nil
	}

	return g.group2(TokenGroupMapKeyValue, key, value)
}

// groupDirectives joins a '%' with its name and the values written after it on
// its line.
//
// The window is that line and the comment lines that may follow it: a directive
// has to be followed by the '---' that opens the document, and the comments in
// between belong to neither. They are the reason a perfectly ordinary
// "%YAML 1.2" with a note above the header was refused whenever comments were
// being parsed.
func (g *grouper) groupDirectives(in []*tapeToken) []*tapeToken {
	out := g.out(len(in))
	{
		for _, tk := range in {
			if g.directive.head != nil {
				if g.directive.name == nil {
					g.directive.name = g.group2(TokenGroupDirectiveName, g.directive.head, tk)

					continue
				}
				if tk.Line() == g.directive.head.Line() {
					g.directive.values = append(g.directive.values, tk)

					continue
				}
				if tk.Type() == token.CommentType {
					g.directive.comments = append(g.directive.comments, tk)

					continue
				}
				if tk.Type() != token.DocumentHeaderType {
					g.fail(yamlerrors.NewSyntax("unexpected directive value. document not started", g.directive.head.RawToken()))

					return out
				}

				head := g.directive.name
				if len(g.directive.values) != 0 {
					head = g.group(TokenGroupDirective, append([]*tapeToken{g.directive.name}, g.directive.values...))
				}
				out = append(out, head)
				out = append(out, g.directive.comments...)
				g.directive.head, g.directive.name, g.directive.values, g.directive.comments = nil, nil, nil, nil
				// The '---' is not part of the g.directive.head, and is read as any
				// other token would be.
			}

			if tk.Type() == token.DirectiveType {
				g.directive.head = tk

				continue
			}
			out = append(out, tk)
		}

		if !g.ending {
			return out
		}

		switch {
		case g.directive.head != nil && g.directive.name == nil:
			g.fail(yamlerrors.NewSyntax("undefined directive value", g.directive.head.RawToken()))
		case g.directive.head != nil:
			g.fail(yamlerrors.NewSyntax("unexpected directive value. document not started", g.directive.head.RawToken()))
		}
	}

	return out
}

func isScalarType(tk *tapeToken) bool {
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
func (g *grouper) groupExplicitKeyBody(body []*tapeToken) ([]*tapeToken, error) {
	// Called from inside groupExplicitKeys, which is reading one of the
	// grouper's two buffers and filling the other.
	g.nested++
	defer func() { g.nested-- }()

	grouped := g.groupMapKeysByValue(body)
	if g.err != nil {
		return nil, g.err
	}

	return g.groupMapKeyValues(grouped), nil
}

// keyEndLine reports the line on which a key token ends.
//
// A plain scalar may span several lines and its position records where it
// starts, so the line that matters for the implicit-key rule has to be
// computed. Whitespace on either side of the origin belongs to the neighboring
// tokens rather than to this one -- a trailing newline in particular would
// otherwise push the end line one past where the token really finishes.
func keyEndLine(tk *tapeToken) int {
	raw := tk.RawToken()
	if raw == nil {
		return tk.Line()
	}

	return tk.Line() + strings.Count(strings.Trim(raw.Origin, " \r\n"), "\n")
}

// closesFlowCollection reports whether tk ends a flow collection.
func closesFlowCollection(tk *tapeToken) bool {
	return tk.Type() == token.MappingEndType || tk.Type() == token.SequenceEndType
}

// flowCollectionStart finds where the flow collection ending at the last token
// of ret begins, so the whole of it can be taken as a mapping key. It reports
// -1 when the brackets do not balance.
func flowCollectionStart(ret []*tapeToken) int {
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
func withKeyProperties(ret []*tapeToken, start int) int {
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
func precedesAbsentKey(tk *tapeToken) bool {
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

// implicitNullKeyToken builds the null node standing in for an absent mapping
// key, positioned where the key would have been -- immediately before its ':'.
func (g *grouper) implicitNullKeyToken(colon *tapeToken) *tapeToken {
	pos := colon.RawToken().Position
	tk := token.New("null", "null", pos)
	tk.Type = token.ImplicitNullType

	wrapped := g.token()
	wrapped.raw = *tk

	return wrapped
}

func isNotMapKeyType(tk *tapeToken) bool {
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
func isFlowType(tk *tapeToken) bool {
	typ := tk.Type()
	return typ == token.MappingStartType ||
		typ == token.MappingEndType ||
		typ == token.SequenceStartType ||
		typ == token.SequenceEndType ||
		typ == token.SequenceEntryType ||
		typ == token.CollectEntryType
}
