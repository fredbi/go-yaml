package parser

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-openapi/go-yaml/internal/errors"
	"github.com/go-openapi/go-yaml/token"
)

type TokenGroupType int

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
	Token       *token.Token
	Group       *TokenGroup
	LineComment *token.Token
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
		return t.Token.Position.Line
	}
	return t.Group.Line()
}

func (t *Token) Column() int {
	if t == nil {
		return 0
	}
	if t.Token != nil {
		return t.Token.Position.Column
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

type TokenGroup struct {
	Type   TokenGroupType
	Tokens []*Token
}

func (g *TokenGroup) First() *Token {
	if len(g.Tokens) == 0 {
		return nil
	}
	return g.Tokens[0]
}

func (g *TokenGroup) Last() *Token {
	if len(g.Tokens) == 0 {
		return nil
	}
	return g.Tokens[len(g.Tokens)-1]
}

func (g *TokenGroup) dump(ctx *groupTokenRenderContext) {
	num := ctx.num
	fmt.Fprint(os.Stdout, colorize(num, "("))
	ctx.num++
	for _, tk := range g.Tokens {
		tk.dump(ctx)
	}
	fmt.Fprint(os.Stdout, colorize(num, ")"))
}

func (g *TokenGroup) RawToken() *token.Token {
	if len(g.Tokens) == 0 {
		return nil
	}
	return g.Tokens[0].RawToken()
}

func (g *TokenGroup) Line() int {
	if len(g.Tokens) == 0 {
		return 0
	}
	return g.Tokens[0].Line()
}

func (g *TokenGroup) Column() int {
	if len(g.Tokens) == 0 {
		return 0
	}
	return g.Tokens[0].Column()
}

func (g *TokenGroup) TokenType() token.Type {
	if len(g.Tokens) == 0 {
		return 0
	}
	return g.Tokens[0].Type()
}

func CreateGroupedTokens(tokens token.Tokens) ([]*Token, error) {
	var err error
	tks := newTokens(tokens)
	tks = createLineCommentTokenGroups(tks)
	tks, err = createLiteralAndFoldedTokenGroups(tks)
	if err != nil {
		return nil, err
	}
	tks, err = createAnchorAndAliasTokenGroups(tks)
	if err != nil {
		return nil, err
	}
	tks, err = createScalarTagTokenGroups(tks)
	if err != nil {
		return nil, err
	}
	tks, err = createAnchorWithScalarTagTokenGroups(tks)
	if err != nil {
		return nil, err
	}
	tks, err = createMapKeyTokenGroups(tks)
	if err != nil {
		return nil, err
	}
	tks = createMapKeyValueTokenGroups(tks)
	tks, err = createDirectiveTokenGroups(tks)
	if err != nil {
		return nil, err
	}
	tks, err = createDocumentTokens(tks)
	if err != nil {
		return nil, err
	}
	return tks, nil
}

func newTokens(tks token.Tokens) []*Token {
	ret := make([]*Token, 0, len(tks))
	for _, tk := range tks {
		ret = append(ret, &Token{Token: tk})
	}
	return ret
}

func createLineCommentTokenGroups(tokens []*Token) []*Token {
	ret := make([]*Token, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		tk := tokens[i]
		switch tk.Type() {
		case token.CommentType:
			if i > 0 && tokens[i-1].Line() == tk.Line() {
				tokens[i-1].LineComment = tk.RawToken()
			} else {
				ret = append(ret, tk)
			}
		default:
			ret = append(ret, tk)
		}
	}
	return ret
}

func createLiteralAndFoldedTokenGroups(tokens []*Token) ([]*Token, error) {
	ret := make([]*Token, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		tk := tokens[i]
		switch tk.Type() {
		case token.LiteralType:
			tks := []*Token{tk}
			if i+1 < len(tokens) {
				tks = append(tks, tokens[i+1])
			}
			ret = append(ret, &Token{
				Group: &TokenGroup{
					Type:   TokenGroupLiteral,
					Tokens: tks,
				},
			})
			i++
		case token.FoldedType:
			tks := []*Token{tk}
			if i+1 < len(tokens) {
				tks = append(tks, tokens[i+1])
			}
			ret = append(ret, &Token{
				Group: &TokenGroup{
					Type:   TokenGroupFolded,
					Tokens: tks,
				},
			})
			i++
		default:
			ret = append(ret, tk)
		}
	}
	return ret, nil
}

func createAnchorAndAliasTokenGroups(tokens []*Token) ([]*Token, error) {
	ret := make([]*Token, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		tk := tokens[i]
		switch tk.Type() {
		case token.AnchorType:
			if i+1 >= len(tokens) {
				return nil, errors.ErrSyntax("undefined anchor name", tk.RawToken())
			}
			if i+2 >= len(tokens) {
				return nil, errors.ErrSyntax("undefined anchor value", tk.RawToken())
			}
			anchorName := &Token{
				Group: &TokenGroup{
					Type:   TokenGroupAnchorName,
					Tokens: []*Token{tk, tokens[i+1]},
				},
			}
			valueTk := tokens[i+2]
			if tk.Line() == valueTk.Line() && valueTk.Type() == token.SequenceEntryType {
				return nil, errors.ErrSyntax("sequence entries are not allowed after anchor on the same line", valueTk.RawToken())
			}
			if tk.Line() == valueTk.Line() && isScalarType(valueTk) {
				ret = append(ret, &Token{
					Group: &TokenGroup{
						Type:   TokenGroupAnchor,
						Tokens: []*Token{anchorName, valueTk},
					},
				})
				i++
			} else {
				ret = append(ret, anchorName)
			}
			i++
		case token.AliasType:
			if i+1 == len(tokens) {
				return nil, errors.ErrSyntax("undefined alias name", tk.RawToken())
			}
			ret = append(ret, &Token{
				Group: &TokenGroup{
					Type:   TokenGroupAlias,
					Tokens: []*Token{tk, tokens[i+1]},
				},
			})
			i++
		default:
			ret = append(ret, tk)
		}
	}
	return ret, nil
}

func createScalarTagTokenGroups(tokens []*Token) ([]*Token, error) {
	ret := make([]*Token, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		tk := tokens[i]
		if tk.Type() != token.TagType {
			ret = append(ret, tk)
			continue
		}
		tag := tk.RawToken()
		if strings.HasPrefix(tag.Value, "!!") {
			// secondary tag.
			switch token.ReservedTagKeyword(tag.Value) {
			case token.IntegerTag, token.FloatTag, token.StringTag, token.BinaryTag, token.TimestampTag, token.BooleanTag, token.NullTag:
				if len(tokens) <= i+1 {
					ret = append(ret, tk)
					continue
				}
				if tk.Line() != tokens[i+1].Line() {
					ret = append(ret, tk)
					continue
				}
				if tokens[i+1].GroupType() == TokenGroupAnchorName {
					ret = append(ret, tk)
					continue
				}
				if isScalarType(tokens[i+1]) {
					ret = append(ret, &Token{
						Group: &TokenGroup{
							Type:   TokenGroupScalarTag,
							Tokens: []*Token{tk, tokens[i+1]},
						},
					})
					i++
				} else {
					ret = append(ret, tk)
				}
			case token.MergeTag:
				if len(tokens) <= i+1 {
					ret = append(ret, tk)
					continue
				}
				if tk.Line() != tokens[i+1].Line() {
					ret = append(ret, tk)
					continue
				}
				if tokens[i+1].GroupType() == TokenGroupAnchorName {
					ret = append(ret, tk)
					continue
				}
				if tokens[i+1].Type() != token.MergeKeyType {
					return nil, errors.ErrSyntax("could not find merge key", tokens[i+1].RawToken())
				}
				ret = append(ret, &Token{
					Group: &TokenGroup{
						Type:   TokenGroupScalarTag,
						Tokens: []*Token{tk, tokens[i+1]},
					},
				})
				i++
			default:
				ret = append(ret, tk)
			}
		} else {
			if len(tokens) <= i+1 {
				ret = append(ret, tk)
				continue
			}
			if tk.Line() != tokens[i+1].Line() {
				ret = append(ret, tk)
				continue
			}
			if tokens[i+1].GroupType() == TokenGroupAnchorName {
				ret = append(ret, tk)
				continue
			}
			if isFlowType(tokens[i+1]) {
				ret = append(ret, tk)
				continue
			}
			ret = append(ret, &Token{
				Group: &TokenGroup{
					Type:   TokenGroupScalarTag,
					Tokens: []*Token{tk, tokens[i+1]},
				},
			})
			i++
		}
	}
	return ret, nil
}

func createAnchorWithScalarTagTokenGroups(tokens []*Token) ([]*Token, error) {
	ret := make([]*Token, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		tk := tokens[i]
		switch tk.GroupType() {
		case TokenGroupAnchorName:
			if i+1 >= len(tokens) {
				return nil, errors.ErrSyntax("undefined anchor value", tk.RawToken())
			}
			valueTk := tokens[i+1]
			if tk.Line() == valueTk.Line() && valueTk.GroupType() == TokenGroupScalarTag {
				ret = append(ret, &Token{
					Group: &TokenGroup{
						Type:   TokenGroupAnchor,
						Tokens: []*Token{tk, tokens[i+1]},
					},
				})
				i++
			} else {
				ret = append(ret, tk)
			}
		default:
			ret = append(ret, tk)
		}
	}
	return ret, nil
}

func createMapKeyTokenGroups(tokens []*Token) ([]*Token, error) {
	tks, err := createMapKeyByMappingKey(tokens)
	if err != nil {
		return nil, err
	}
	return createMapKeyByMappingValue(tks)
}

func createMapKeyByMappingKey(tokens []*Token) ([]*Token, error) {
	ret := make([]*Token, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		tk := tokens[i]
		switch tk.Type() {
		case token.MappingKeyType:
			if i+1 >= len(tokens) {
				return nil, errors.ErrSyntax("undefined map key", tk.RawToken())
			}
			ret = append(ret, &Token{
				Group: &TokenGroup{
					Type:   TokenGroupMapKey,
					Tokens: []*Token{tk, tokens[i+1]},
				},
			})
			i++
		default:
			ret = append(ret, tk)
		}
	}
	return ret, nil
}

func createMapKeyByMappingValue(tokens []*Token) ([]*Token, error) {
	ret := make([]*Token, 0, len(tokens))
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
		case token.MappingValueType:
			if hasNoKey(tokens, i, flowDepth > 0) {
				// The key is absent: ": value", "- :", "{ : }", "{a: 1, : 2}".
				// YAML 1.2 allows it, and an absent key is the null node -- so
				// there is nothing to reject here, only a node to supply.
				ret = append(ret, &Token{
					Group: &TokenGroup{
						Type:   TokenGroupMapKey,
						Tokens: []*Token{implicitNullKeyToken(tk), tk},
					},
				})

				continue
			}
			mapKeyTk := tokens[keyCandidateIndex(tokens, i)]
			if isNotMapKeyType(mapKeyTk) {
				return nil, errors.ErrSyntax("found an invalid key for this map", tk.RawToken())
			}
			newTk := &Token{Token: mapKeyTk.Token, Group: mapKeyTk.Group}
			mapKeyTk.Token = nil
			mapKeyTk.Group = &TokenGroup{
				Type:   TokenGroupMapKey,
				Tokens: []*Token{newTk, tk},
			}
		default:
			ret = append(ret, tk)
		}
	}
	return ret, nil
}

func createMapKeyValueTokenGroups(tokens []*Token) []*Token {
	ret := make([]*Token, 0, len(tokens))
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
				ret = append(ret, &Token{
					Group: &TokenGroup{
						Type:   TokenGroupMapKeyValue,
						Tokens: []*Token{tk, valueTk},
					},
				})
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

func createDirectiveTokenGroups(tokens []*Token) ([]*Token, error) {
	ret := make([]*Token, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		tk := tokens[i]
		switch tk.Type() {
		case token.DirectiveType:
			if i+1 >= len(tokens) {
				return nil, errors.ErrSyntax("undefined directive value", tk.RawToken())
			}
			directiveName := &Token{
				Group: &TokenGroup{
					Type:   TokenGroupDirectiveName,
					Tokens: []*Token{tk, tokens[i+1]},
				},
			}
			i++
			var valueTks []*Token
			for j := i + 1; j < len(tokens); j++ {
				if tokens[j].Line() != tk.Line() {
					break
				}
				valueTks = append(valueTks, tokens[j])
				i++
			}
			if i+1 >= len(tokens) || tokens[i+1].Type() != token.DocumentHeaderType {
				return nil, errors.ErrSyntax("unexpected directive value. document not started", tk.RawToken())
			}
			if len(valueTks) != 0 {
				ret = append(ret, &Token{
					Group: &TokenGroup{
						Type:   TokenGroupDirective,
						Tokens: append([]*Token{directiveName}, valueTks...),
					},
				})
			} else {
				ret = append(ret, directiveName)
			}
		default:
			ret = append(ret, tk)
		}
	}
	return ret, nil
}

func createDocumentTokens(tokens []*Token) ([]*Token, error) {
	var ret []*Token
	for i := 0; i < len(tokens); i++ {
		tk := tokens[i]
		switch tk.Type() {
		case token.DocumentHeaderType:
			if i != 0 {
				ret = append(ret, &Token{
					Group: &TokenGroup{Tokens: tokens[:i]},
				})
			}
			if i+1 == len(tokens) {
				// if current token is last token, add DocumentHeader only tokens to ret.
				return append(ret, &Token{
					Group: &TokenGroup{
						Type:   TokenGroupDocument,
						Tokens: []*Token{tk},
					},
				}), nil
			}
			if tokens[i+1].Type() == token.DocumentHeaderType {
				return append(ret, &Token{
					Group: &TokenGroup{
						Type:   TokenGroupDocument,
						Tokens: []*Token{tk},
					},
				}), nil
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
			tks, err := createDocumentTokens(tokens[i+1:])
			if err != nil {
				return nil, err
			}
			if len(tks) != 0 {
				tks[0].SetGroupType(TokenGroupDocument)
				tks[0].Group.Tokens = append([]*Token{tk}, tks[0].Group.Tokens...)
				return append(ret, tks...), nil
			}
			return append(ret, &Token{
				Group: &TokenGroup{
					Type:   TokenGroupDocument,
					Tokens: []*Token{tk},
				},
			}), nil
		case token.DocumentEndType:
			if i != 0 {
				ret = append(ret, &Token{
					Group: &TokenGroup{
						Type:   TokenGroupDocument,
						Tokens: tokens[0 : i+1],
					},
				})
			}
			if i+1 == len(tokens) {
				return ret, nil
			}
			if isScalarType(tokens[i+1]) {
				return nil, errors.ErrSyntax("unexpected end content", tokens[i+1].RawToken())
			}

			tks, err := createDocumentTokens(tokens[i+1:])
			if err != nil {
				return nil, err
			}
			return append(ret, tks...), nil
		}
	}
	return append(ret, &Token{
		Group: &TokenGroup{
			Type:   TokenGroupDocument,
			Tokens: tokens,
		},
	}), nil
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
func implicitNullKeyToken(colon *Token) *Token {
	pos := *(colon.RawToken().Position)
	tk := token.New("null", "null", &pos)
	tk.Type = token.ImplicitNullType

	return &Token{Token: tk}
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

func isFlowType(tk *Token) bool {
	typ := tk.Type()
	return typ == token.MappingStartType ||
		typ == token.MappingEndType ||
		typ == token.SequenceStartType ||
		typ == token.SequenceEntryType
}
