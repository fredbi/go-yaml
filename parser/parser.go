// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"errors"
	"fmt"
	"iter"
	"os"
	"strings"

	"github.com/go-openapi/go-yaml/ast"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
	"github.com/go-openapi/go-yaml/parser/scanner"
	"github.com/go-openapi/go-yaml/token"
)

type Mode uint

const (
	ParseComments Mode = 1 << iota // parse comments and add them to AST
)

// ParseBytes reads src and returns the file it describes.
func ParseBytes(src []byte, mode Mode, opts ...Option) (*ast.File, error) {
	text := string(src)

	var s scanner.Scanner
	s.Init(text)

	p, err := New(s.Tokens(), mode, opts...)
	if scanErr := s.Err(); scanErr != nil {
		// The scanner stopped first, and says why. New only knows that the
		// token it was handed was an invalid one.
		err = scanErr
	}
	if err != nil {
		return nil, yamlerrors.WithSource(asSyntaxError(err), yamlerrors.Source{Text: text, FirstLine: 1})
	}

	f, err := p.Parse()
	if err != nil {
		// An error drawn under the document needs the document. Parse reads a
		// token stream and has none, so it is told here, where the text is.
		return nil, yamlerrors.WithSource(err, yamlerrors.Source{Text: text, FirstLine: 1})
	}

	return f, nil
}

// ParseFile reads the file named filename and returns the file it describes.
func ParseFile(filename string, mode Mode, opts ...Option) (*ast.File, error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	f, err := ParseBytes(src, mode, opts...)
	if err != nil {
		return nil, err
	}
	f.Name = filename

	return f, nil
}

// asSyntaxError reports a scanning failure the way a parsing one is reported,
// so a caller sees one kind of error whichever stage refused the document.
func asSyntaxError(err error) error {
	var invalid *scanner.InvalidTokenError
	if errors.As(err, &invalid) {
		return yamlerrors.NewSyntax(invalid.Message, invalid.Token)
	}

	return err
}

type YAMLVersion string

const (
	YAML10 YAMLVersion = "1.0"
	YAML11 YAMLVersion = "1.1"
	YAML12 YAMLVersion = "1.2"
	YAML13 YAMLVersion = "1.3"
)

var yamlVersionMap = map[string]YAMLVersion{
	"1.0": YAML10,
	"1.1": YAML11,
	"1.2": YAML12,
	"1.3": YAML13,
}

type Parser struct {
	tokens []*Token
	raw    rawTokens
	// entries holds the entries of every mapping open at this point in the
	// descent, innermost run last. parseMap takes its run off the end once the
	// mapping is built.
	entries []*ast.MappingValueNode
	// lineComments holds the comment closing a token's line, against that
	// token. It is nil where the mode did not ask for comments.
	lineComments          map[*Token]*token.Token
	yamlVersion           YAMLVersion
	allowDuplicateMapKey  bool
	omitNodePaths         bool
	secondaryTagDirective *ast.DirectiveNode
	tagHandles            map[string]struct{}

	// keyStack holds the keys of every mapping open at this point in the
	// descent, innermost last, and keyIndex addresses them. Both are reused
	// for the whole parse: a mapping pushes its keys on the way in and drops
	// them on the way out, so the two grow once to the deepest, widest point
	// of the document and allocate nothing after that.
	keyStack []mapKeyRef
	// keyIndex records where each of those keys was first written. It keeps the
	// position and not the node it came from: a node holds the token it was
	// built from, and a token kept here outlives the entry that carried it, so
	// every key of every open mapping would stay reachable until that mapping
	// closed. A mapping of 5,000 keys held 5,000 tokens spread over the whole
	// document; it now holds 5,000 positions of 16 bytes and no token at all.
	keyIndex map[mapKeyRef]token.Position

	// seqEntries holds the entries of every sequence open at this point in the
	// descent, innermost last. A sequence fills its slices from its own run
	// when it closes, each at the length it ends up with, rather than growing
	// three of them an entry at a time. That growth was 96-99% of everything
	// runtime.growslice copied during a parse -- 1,385K of 1,389K on
	// canada_geometry, which is deep sequences and nothing else.
	seqEntries []pendingEntry

	// pathSlab hands out path trie nodes in blocks, so a document of N keys
	// costs N/pathSlabSize allocations rather than N.
	pathSlab []ast.PathNode
	// refs holds one token reference per depth of the descent. They are held by
	// pointer, so growing the slice leaves the ones in hand where they are.
	refs []*tokenRef
}

// tokenRefAt returns the reference for a group read at depth, positioned at the
// start of tokens.
//
// The parse is depth first, so one group at most is being read at each depth at
// any moment: the reference for a depth is set again for the next group read
// there rather than another being taken. A document nested N deep costs N
// references however many groups it holds.
func (p *Parser) tokenRefAt(depth int32, g *TokenGroup) *tokenRef {
	for int(depth) >= len(p.refs) {
		p.refs = append(p.refs, new(tokenRef))
	}

	ref := p.refs[depth]
	ref.tokens, ref.idx = g.Members(&ref.pair), 0
	ref.pull, ref.drained = nil, false

	return ref
}

// pathSlabSize is how many trie steps one allocation covers. A document of N
// keys then costs N/pathSlabSize allocations rather than N.
const pathSlabSize = 512

// newPathNode returns the next unused step of the path trie, or nil when
// [OmitNodePaths] has turned path recording off.
func (p *Parser) newPathNode() *ast.PathNode {
	if p.omitNodePaths {
		return nil
	}
	if len(p.pathSlab) == 0 {
		p.pathSlab = make([]ast.PathNode, pathSlabSize)
	}
	n := &p.pathSlab[0]
	p.pathSlab = p.pathSlab[1:]

	return n
}

// mapKeyRef addresses one key of one mapping: base is where that mapping's
// keys start in keyStack, and text is the key as mapKeyText reads it.
type mapKeyRef struct {
	base int
	text string
}

// recordMapKey records that the mapping starting at base uses text as a key,
// written at pos.
//
// It returns where text was first written, and whether the mapping had already
// used it.
func (p *Parser) recordMapKey(base int, text string, pos token.Position) (token.Position, bool) {
	ref := mapKeyRef{base: base, text: text}
	if prev, defined := p.keyIndex[ref]; defined {
		return prev, true
	}
	p.keyIndex[ref] = pos
	p.keyStack = append(p.keyStack, ref)

	return token.Position{}, false
}

// closeMapping drops the keys of the mapping that started at base.
//
// Mappings close in the order they open, so the keys of the one closing are
// always those above its base.
func (p *Parser) closeMapping(base int) {
	for _, ref := range p.keyStack[base:] {
		delete(p.keyIndex, ref)
	}
	p.keyStack = p.keyStack[:base]
}

// New returns a parser reading the tokens seq hands over.
//
// seq is read to its end here, because grouping looks both ways along the
// stream: what a ':' belongs to is not settled until the tokens after it have
// been read. The tokens are held as values in blocks of their own, so what the
// scanner hands over is copied and the scanner keeps none of it.
//
// Where mode does not carry ParseComments, comment tokens are dropped as they
// arrive and never reach the grouping at all.
func New(seq iter.Seq[token.Token], mode Mode, opts ...Option) (*Parser, error) {
	keepComments := mode&ParseComments != 0

	var raw rawTokens
	for tk := range seq {
		if !keepComments && tk.Type == token.CommentType {
			continue
		}
		if tk.Type == token.InvalidType {
			// A stream handed in rather than scanned here carries no reason:
			// the scanner reports one, and Scanner.Err returns it.
			held := raw.add(tk)

			return nil, yamlerrors.NewSyntax("found an invalid token", held)
		}
		raw.add(tk)
	}

	tks, lineComments, err := createGroupedTokens(&raw)
	if err != nil {
		return nil, err
	}

	p := &Parser{
		tokens:       tks,
		raw:          raw,
		lineComments: lineComments,
		keyIndex:     make(map[mapKeyRef]token.Position),
	}
	for _, opt := range opts {
		opt(p)
	}

	return p, nil
}

// Parse reads the stream through and returns the file it describes.
//
// Call it once per parser. A comment is handed to the node that keeps it as the
// tree is built, and a second call would find none left to hand over.
func (p *Parser) Parse() (*ast.File, error) {
	return p.parse(p.newContext())
}

func (p *Parser) parse(ctx context) (*ast.File, error) {
	file := &ast.File{Docs: []*ast.DocumentNode{}}
	for _, token := range p.tokens {
		doc, err := p.parseDocument(ctx, token.Group)
		if err != nil {
			return nil, err
		}
		file.Docs = append(file.Docs, doc)
	}
	return file, nil
}

func (p *Parser) parseDocument(ctx context, docGroup *TokenGroup) (*ast.DocumentNode, error) {
	if docGroup.Len() == 0 {
		return ast.Document(docGroup.RawToken(), nil), nil
	}

	var (
		docPair [2]*Token
		tokens  = docGroup.Members(&docPair)
		start   *token.Token
		end     *token.Token
	)
	if docGroup.First().Type() == token.DocumentHeaderType {
		start = docGroup.First().RawToken()
		tokens = tokens[1:]
	}
	if docGroup.Last().Type() == token.DocumentEndType {
		end = docGroup.Last().RawToken()
		tokens = tokens[:len(tokens)-1]
		defer func() {
			// clear yaml version value if DocumentEnd token (...) is specified.
			p.yamlVersion = ""
		}()
	}

	if len(tokens) == 0 {
		return ast.Document(docGroup.RawToken(), nil), nil
	}

	body, err := p.parseDocumentBody(ctx.withGroup(p, newTokenGroup(TokenGroupDocumentBody, tokens)))
	if err != nil {
		return nil, err
	}

	// A TAG directive defines a handle for the one document that follows it,
	// and this was that document: what it declared goes out of scope here.
	// Carrying the definitions on let a later document use a handle it never
	// declared. A document holding only the directives themselves does not end
	// their scope -- it is what opens it.
	if _, directives := body.(*ast.DirectiveNode); !directives {
		p.clearTagDirectives()
	}
	node := ast.Document(start, body)
	node.End = end
	return node, nil
}

func (p *Parser) parseDocumentBody(ctx context) (ast.Node, error) {
	node, err := p.parseToken(ctx, ctx.currentToken())
	if err != nil {
		return nil, err
	}
	// Comments may trail what the document holds -- between a directive and the
	// '---' below it, most often. They are not a second value.
	if comment := p.parseFootComment(ctx, 1); comment != nil {
		if err := setHeadComment(comment, node); err != nil {
			return nil, err
		}
	}
	if ctx.next() {
		return nil, yamlerrors.NewSyntax("value is not allowed in this context", ctx.currentToken().RawToken())
	}
	return node, nil
}

func (p *Parser) parseToken(ctx context, tk *Token) (ast.Node, error) {
	switch tk.GroupType() {
	case TokenGroupMapKey, TokenGroupMapKeyValue:
		return p.parseMap(ctx)
	case TokenGroupDirective:
		node, err := p.parseDirective(ctx.withGroup(p, tk.Group), tk.Group)
		if err != nil {
			return nil, err
		}
		ctx.goNext()
		return node, nil
	case TokenGroupDirectiveName:
		node, err := p.parseDirectiveName(ctx.withGroup(p, tk.Group))
		if err != nil {
			return nil, err
		}
		ctx.goNext()
		return node, nil
	case TokenGroupAnchor:
		node, err := p.parseAnchor(ctx.withGroup(p, tk.Group), tk.Group)
		if err != nil {
			return nil, err
		}
		ctx.goNext()
		return node, nil
	case TokenGroupAnchorName:
		anchor, err := p.parseAnchorName(ctx.withGroup(p, tk.Group))
		if err != nil {
			return nil, err
		}
		ctx.goNext()
		value, err := p.parseAnchorValue(ctx, anchor)
		if err != nil {
			return nil, err
		}
		anchor.Value = value
		return anchor, nil
	case TokenGroupAlias:
		node, err := p.parseAlias(ctx.withGroup(p, tk.Group))
		if err != nil {
			return nil, err
		}
		ctx.goNext()
		return node, nil
	case TokenGroupLiteral, TokenGroupFolded:
		node, err := p.parseLiteral(ctx.withGroup(p, tk.Group))
		if err != nil {
			return nil, err
		}
		ctx.goNext()
		return node, nil
	case TokenGroupScalarTag:
		node, err := p.parseTag(ctx.withGroup(p, tk.Group))
		if err != nil {
			return nil, err
		}
		ctx.goNext()
		return node, nil
	}
	switch tk.Type() {
	case token.CommentType:
		return p.parseComment(ctx)
	case token.TagType:
		return p.parseTag(ctx)
	case token.MappingStartType:
		return p.parseFlowMap(ctx.withFlow(true))
	case token.SequenceStartType:
		return p.parseFlowSequence(ctx.withFlowSequence())
	case token.SequenceEntryType:
		return p.parseSequence(ctx)
	case token.SequenceEndType:
		// SequenceEndType is always validated in parseFlowSequence.
		// Therefore, if this is found in other cases, it is treated as a syntax error.
		return nil, yamlerrors.NewSyntax("could not find '[' character corresponding to ']'", tk.RawToken())
	case token.MappingEndType:
		// MappingEndType is always validated in parseFlowMap.
		// Therefore, if this is found in other cases, it is treated as a syntax error.
		return nil, yamlerrors.NewSyntax("could not find '{' character corresponding to '}'", tk.RawToken())
	case token.MappingValueType:
		return nil, yamlerrors.NewSyntax("found an invalid key for this map", tk.RawToken())
	}
	node, err := p.parseScalarValue(ctx, tk)
	if err != nil {
		return nil, err
	}
	ctx.goNext()
	return node, nil
}

func (p *Parser) parseScalarValue(ctx context, tk *Token) (ast.ScalarNode, error) {
	if tk.Group != nil {
		switch tk.GroupType() {
		case TokenGroupAnchor:
			return p.parseAnchor(ctx.withGroup(p, tk.Group), tk.Group)
		case TokenGroupAnchorName:
			anchor, err := p.parseAnchorName(ctx.withGroup(p, tk.Group))
			if err != nil {
				return nil, err
			}
			ctx.goNext()
			value, err := p.parseAnchorValue(ctx, anchor)
			if err != nil {
				return nil, err
			}
			anchor.Value = value
			return anchor, nil
		case TokenGroupAlias:
			return p.parseAlias(ctx.withGroup(p, tk.Group))
		case TokenGroupLiteral, TokenGroupFolded:
			return p.parseLiteral(ctx.withGroup(p, tk.Group))
		case TokenGroupScalarTag:
			return p.parseTag(ctx.withGroup(p, tk.Group))
		default:
			return nil, yamlerrors.NewSyntax("unexpected scalar value", tk.RawToken())
		}
	}
	switch tk.Type() {
	case token.MergeKeyType:
		return newMergeKeyNode(ctx, tk)
	case token.NullType, token.ImplicitNullType:
		return newNullNode(ctx, tk)
	case token.BoolType:
		return newBoolNode(ctx, tk)
	case token.IntegerType, token.BinaryIntegerType, token.OctetIntegerType, token.HexIntegerType:
		return newIntegerNode(ctx, tk)
	case token.FloatType:
		return newFloatNode(ctx, tk)
	case token.InfinityType:
		return newInfinityNode(ctx, tk)
	case token.NanType:
		return newNanNode(ctx, tk)
	case token.StringType, token.SingleQuoteType, token.DoubleQuoteType:
		return newStringNode(ctx, tk)
	case token.TagType:
		// this case applies when it is a scalar tag and its value does not exist.
		// Examples of cases where the value does not exist include cases like `key: !!str,` or `!!str : value`.
		return p.parseScalarTag(ctx)
	}
	return nil, yamlerrors.NewSyntax("unexpected scalar value type", tk.RawToken())
}

// attachTrailingComment gives a comment written after a ',' to the entry the
// ',' follows, which is the entry it was written about: in "[ a, # note" the
// note sits on a's line and is a remark on a.
//
// The scanner hangs such a comment on the ',' itself rather than leaving it in
// the stream, so the loop reading the collection never sees it. Left there it
// reached a sequence entry node that nothing renders, or -- in a mapping -- the
// entry after the comma, one place further on than it was written.
func attachTrailingComment(ctx context, entryTk *Token, values []ast.Node) error {
	if entryTk == nil || len(values) == 0 || ctx.lineComment(entryTk) == nil {
		return nil
	}

	target := values[len(values)-1]
	if entry, ok := target.(*ast.MappingValueNode); ok && entry.Value != nil {
		// On the entry itself it would read as a comment introducing it.
		target = entry.Value
	}
	if target.GetComment() != nil {
		return nil
	}
	comment := ast.CommentGroup([]*token.Token{ctx.takeLineComment(entryTk)})
	comment.SetPathNode(ctx.path)

	return target.SetComment(comment)
}

func (p *Parser) parseFlowMap(ctx context) (*ast.MappingNode, error) {
	base := len(p.keyStack)
	defer p.closeMapping(base)
	ctx = ctx.withMapping(base)

	node, err := newMappingNode(ctx, ctx.currentToken().RawToken(), true, nil)
	if err != nil {
		return nil, err
	}
	ctx.goNext() // skip MappingStart token

	isFirst := true
	for ctx.next() {
		// As in a flow sequence: a comment may precede the ',' as well as
		// follow it.
		headComment := p.parseHeadComment(ctx)
		if ctx.isTokenNotFound() {
			break
		}

		tk := ctx.currentToken()
		if tk.Type() == token.MappingEndType {
			node.End = tk.RawToken()
			node.FootComment = headComment
			break
		}

		var entryTk *Token
		if tk.Type() == token.CollectEntryType {
			entryTk = tk
			entered := make([]ast.Node, 0, len(node.Values))
			for _, value := range node.Values {
				entered = append(entered, value)
			}
			if err := attachTrailingComment(ctx, entryTk, entered); err != nil {
				return nil, err
			}
			ctx.goNext()
			if next := p.parseHeadComment(ctx); next != nil {
				headComment = mergeComments(headComment, next)
			}
		} else if !isFirst {
			return nil, yamlerrors.NewSyntax("',' or '}' must be specified", tk.RawToken())
		}

		if tk := ctx.currentToken(); tk.Type() == token.MappingEndType {
			// this case is here: "{ elem, }".
			// In this case, ignore the last element and break mapping parsing.
			node.End = tk.RawToken()
			break
		}

		mapKeyTk := ctx.currentToken()
		entered := len(node.Values)
		switch mapKeyTk.GroupType() {
		case TokenGroupMapKeyValue:
			value, err := p.parseMapKeyValue(ctx.withGroup(p, mapKeyTk.Group), mapKeyTk.Group, entryTk)
			if err != nil {
				return nil, err
			}
			node.Values = append(node.Values, value)
			ctx.goNext()
		case TokenGroupMapKey:
			key, err := p.parseMapKey(ctx.withGroup(p, mapKeyTk.Group), mapKeyTk.Group)
			if err != nil {
				return nil, err
			}
			ctx := p.valueContext(ctx, key)
			colonTk := mapKeyTk.Group.Last()
			if p.isFlowMapDelim(ctx.nextToken()) {
				value, err := newNullNode(ctx, ctx.insertNullToken(colonTk))
				if err != nil {
					return nil, err
				}
				mapValue, err := newMappingValueNode(ctx, colonTk, entryTk, key, value)
				if err != nil {
					return nil, err
				}
				node.Values = append(node.Values, mapValue)
				ctx.goNext()
			} else {
				ctx.goNext()
				if ctx.isTokenNotFound() {
					return nil, yamlerrors.NewSyntax("could not find map value", colonTk.RawToken())
				}
				value, err := p.parseToken(ctx, ctx.currentToken())
				if err != nil {
					return nil, err
				}
				mapValue, err := newMappingValueNode(ctx, colonTk, entryTk, key, value)
				if err != nil {
					return nil, err
				}
				node.Values = append(node.Values, mapValue)
			}
		default:
			if !p.isFlowMapDelim(ctx.nextToken()) {
				errTk := mapKeyTk
				if errTk == nil {
					errTk = tk
				}
				return nil, yamlerrors.NewSyntax("could not find flow map content", errTk.RawToken())
			}
			key, err := p.parseScalarValue(ctx, mapKeyTk)
			if err != nil {
				return nil, err
			}
			value, err := newNullNode(ctx, ctx.insertNullToken(mapKeyTk))
			if err != nil {
				return nil, err
			}
			mapValue, err := newMappingValueNode(ctx, mapKeyTk, entryTk, key, value)
			if err != nil {
				return nil, err
			}
			node.Values = append(node.Values, mapValue)
			if ctx.currentToken() == mapKeyTk {
				// A plain scalar key is still the current token, so skip it. A
				// key that is a property group -- the "&a" of "{&a}" -- was
				// read by parseScalarValue, which already moved past it, and
				// advancing again would step over the '}'.
				ctx.goNext()
			}
		}
		if headComment != nil && len(node.Values) > entered {
			// The comment introduced this entry, so it belongs above it.
			if err := node.Values[entered].SetComment(headComment); err != nil {
				return nil, err
			}
		}
		isFirst = false
	}
	if node.End == nil {
		return nil, yamlerrors.NewSyntax("could not find flow mapping end token '}'", node.Start)
	}

	// set line comment if exists. e.g.) } # comment
	if err := setLineComment(ctx, node, ctx.currentToken()); err != nil {
		return nil, err
	}
	ctx.goNext() // skip mapping end token.
	return node, nil
}

func (p *Parser) isFlowMapDelim(tk *Token) bool {
	return tk.Type() == token.MappingEndType || tk.Type() == token.CollectEntryType
}

// parseMapEntry parses exactly ONE "key: value" pair at keyTk.
//
// Extracted from parseMap so sibling entries can be accumulated in a loop. parseMap used to
// recurse once per sibling, building a whole MappingNode at every level and discarding it to
// keep only .Values -- which made a mapping of N keys cost N recursions and slice
// concatenations summing to O(N^2).
func (p *Parser) parseMapEntry(ctx context, keyTk *Token) (*ast.MappingValueNode, error) {
	if keyTk.Group == nil {
		return nil, yamlerrors.NewSyntax("unexpected map key", keyTk.RawToken())
	}
	if keyTk.GroupType() == TokenGroupMapKeyValue {
		node, err := p.parseMapKeyValue(ctx.withGroup(p, keyTk.Group), keyTk.Group, nil)
		if err != nil {
			return nil, err
		}
		ctx.goNext()
		if err := p.validateMapKeyValueNextToken(ctx, keyTk, ctx.currentToken()); err != nil {
			return nil, err
		}

		return node, nil
	}

	key, err := p.parseMapKey(ctx.withGroup(p, keyTk.Group), keyTk.Group)
	if err != nil {
		return nil, err
	}
	ctx.goNext()

	valueTk := ctx.currentToken()
	if keyTk.Line() == valueTk.Line() && valueTk.Type() == token.SequenceEntryType {
		return nil, yamlerrors.NewSyntax("block sequence entries are not allowed in this context", valueTk.RawToken())
	}
	childCtx := p.valueContext(ctx, key)
	value, err := p.parseMapValue(childCtx, key, keyTk.Group.Last())
	if err != nil {
		return nil, err
	}

	return newMappingValueNode(childCtx, keyTk.Group.Last(), nil, key, value)
}

func (p *Parser) parseMap(ctx context) (*ast.MappingNode, error) {
	base := len(p.keyStack)
	defer p.closeMapping(base)
	ctx = ctx.withMapping(base)

	// The entries are gathered on a stack the parser reuses for every mapping,
	// so a mapping's Values is allocated once, at its own length, rather than
	// grown an entry at a time. entryBase is where this mapping's run starts.
	entryBase := len(p.entries)
	defer func() { p.entries = p.entries[:entryBase] }()

	keyTk := ctx.currentToken()
	keyValueNode, err := p.parseMapEntry(ctx, keyTk)
	if err != nil {
		return nil, err
	}
	p.entries = append(p.entries, keyValueNode)

	var tk *Token
	if ctx.isComment() {
		tk = ctx.nextNotCommentToken()
	} else {
		tk = ctx.currentToken()
	}
	for tk.Column() == keyTk.Column() {
		typ := tk.Type()
		if ctx.isFlow && typ == token.SequenceEndType {
			// [
			// key: value
			// ] <=
			break
		}
		if !p.isMapToken(tk) {
			return nil, yamlerrors.NewSyntax("non-map value is specified", tk.RawToken())
		}
		cm := p.parseHeadComment(ctx)
		if typ == token.MappingEndType {
			// a: {
			//  b: c
			// } <=
			ctx.goNext()
			break
		}
		entry, err := p.parseMapEntry(ctx, ctx.currentToken())
		if err != nil {
			return nil, err
		}
		if err := setHeadComment(cm, entry); err != nil {
			return nil, err
		}
		p.entries = append(p.entries, entry)
		if ctx.isComment() {
			tk = ctx.nextNotCommentToken()
		} else {
			tk = ctx.currentToken()
		}
	}
	mapNode, err := newMappingNode(ctx, keyValueNode.GetToken(), false, p.entries[entryBase:])
	if err != nil {
		return nil, err
	}

	if ctx.isComment() {
		if keyTk.Column() <= ctx.currentToken().Column() {
			// If the comment is in the same or deeper column as the last element column in map value,
			// treat it as a footer comment for the last element.
			//
			// It attaches to the last ENTRY rather than to the mapping: when sibling entries were
			// parsed by recursion, the innermost call always held exactly one value and so took
			// that branch. Parsing them in a loop puts every value in one node, so the choice has
			// to be made explicitly to keep the attribution identical.
			last := mapNode.Values[len(mapNode.Values)-1]
			last.FootComment = p.parseFootComment(ctx, keyTk.Column())
			last.FootComment.SetPathNode(last.Key.GetPathNode())
		}
	}
	return mapNode, nil
}

func (p *Parser) validateMapKeyValueNextToken(ctx context, keyTk, tk *Token) error {
	if tk == nil {
		return nil
	}
	if tk.Column() <= keyTk.Column() {
		return nil
	}
	if ctx.isComment() {
		return nil
	}
	if ctx.isFlow && (tk.Type() == token.CollectEntryType || tk.Type() == token.SequenceEndType) {
		return nil
	}
	// a: b
	//  c <= this token is invalid.
	return yamlerrors.NewSyntax("value is not allowed in this context. map key-value is pre-defined", tk.RawToken())
}

func (p *Parser) isMapToken(tk *Token) bool {
	if tk.Group == nil {
		return tk.Type() == token.MappingStartType || tk.Type() == token.MappingEndType
	}
	g := tk.Group
	return g.Type == TokenGroupMapKey || g.Type == TokenGroupMapKeyValue
}

func (p *Parser) parseMapKeyValue(ctx context, g *TokenGroup, entryTk *Token) (*ast.MappingValueNode, error) {
	if g.Type != TokenGroupMapKeyValue {
		return nil, yamlerrors.NewSyntax("unexpected map key-value pair", g.RawToken())
	}
	if g.First().Group == nil {
		return nil, yamlerrors.NewSyntax("unexpected map key", g.RawToken())
	}
	keyGroup := g.First().Group
	key, err := p.parseMapKey(ctx.withGroup(p, keyGroup), keyGroup)
	if err != nil {
		return nil, err
	}

	c := p.valueContext(ctx, key)
	value, err := p.parseToken(c, g.Last())
	if err != nil {
		return nil, err
	}
	return newMappingValueNode(c, keyGroup.Last(), entryTk, key, value)
}

// parseMapKeyValueNode parses the key part of a map-key group.
//
// A key is usually a single scalar token, and that path is kept: it is every
// ordinary document. A key spanning more tokens is a flow collection used as a
// key, which has to be parsed as a node like any other.
func (p *Parser) parseMapKeyValueNode(ctx context, g *TokenGroup) (ast.Node, error) {
	if g.Len() <= 2 {
		return p.parseScalarValue(ctx, g.First())
	}

	return p.parseToken(ctx, g.First())
}

func (p *Parser) parseMapKey(ctx context, g *TokenGroup) (ast.MapKeyNode, error) {
	if g.Type != TokenGroupMapKey {
		return nil, yamlerrors.NewSyntax("unexpected map key", g.RawToken())
	}
	if g.First().Type() == token.MappingKeyType {
		mapKeyTk := g.First()
		if mapKeyTk.Group != nil {
			ctx = ctx.withGroup(p, mapKeyTk.Group)
		}
		key, err := newMappingKeyNode(ctx, mapKeyTk)
		if err != nil {
			return nil, err
		}
		ctx.goNext() // skip mapping key token
		if ctx.isTokenNotFound() {
			return nil, yamlerrors.NewSyntax("could not find value for mapping key", mapKeyTk.RawToken())
		}

		value, err := p.parseToken(ctx, ctx.currentToken())
		if err != nil {
			return nil, err
		}
		scalar, ok := value.(ast.MapKeyNode)
		if !ok {
			return nil, yamlerrors.NewSyntax("cannot use this node as a map key", value.GetToken())
		}
		key.Value = scalar
		if _, isScalar := value.(ast.ScalarNode); !isScalar {
			// A collection used as a key has no path: neither YAMLPath nor
			// JSON Pointer has syntax that reaches one, so it stays out of the
			// path map rather than being given an invented address.
			return key, nil
		}
		keyText := p.mapKeyText(scalar)
		key.SetPathNode(ctx.withChild(p, keyText).path)
		if err := p.validateMapKey(ctx, key, keyText, g.Last()); err != nil {
			return nil, err
		}

		return key, nil
	}
	if g.Last().Type() != token.MappingValueType {
		return nil, yamlerrors.NewSyntax("expected map key-value delimiter ':'", g.Last().RawToken())
	}

	scalar, err := p.parseMapKeyValueNode(ctx, g)
	if err != nil {
		return nil, err
	}
	key, ok := scalar.(ast.MapKeyNode)
	if !ok {
		return nil, yamlerrors.NewSyntax("cannot take map-key node", scalar.GetToken())
	}
	keyText := p.mapKeyText(key)
	key.SetPathNode(ctx.withChild(p, keyText).path)
	if err := p.validateMapKey(ctx, key, keyText, g.Last()); err != nil {
		return nil, err
	}

	return key, nil
}

// validateMapKey checks key against the rules a mapping key is held to, and
// records it among the keys of the mapping being parsed.
//
// keyText is the key as mapKeyText reads it. Two entries of one mapping repeat
// a key when their texts are equal, so the check needs the text and not the
// path built from it.
func (p *Parser) validateMapKey(ctx context, key ast.MapKeyNode, keyText string, colonTk *Token) error {
	tk := key.GetToken()
	if !p.allowDuplicateMapKey {
		if pos, defined := p.recordMapKey(ctx.keyBase, keyText, tk.Position); defined {
			return yamlerrors.NewSyntax(
				fmt.Sprintf("mapping key %q already defined at [%d:%d]", tk.Value, pos.Line, pos.Column),
				tk,
			)
		}
	}
	origin := p.removeLeftWhiteSpace(tk.Origin)
	if ctx.isFlow {
		// A pair written inside a flow sequence is an implicit key: it has to
		// fit on one line, and its ':' has to be on that line with it.
		//
		// A flow mapping's key is under neither restriction. It may span lines,
		// and a line break before the ':' is ordinary separation, so
		// "{foo\n: bar}" is as legal as "{foo: bar}".
		if ctx.inFlowSequence && isScalarKeyToken(tk) {
			origin = p.removeRightWhiteSpace(origin)
			if int(tk.Position.Line)+p.newLineCharacterNum(origin) != colonTk.Line() {
				return yamlerrors.NewSyntax("map key definition includes an implicit line break", tk)
			}
		}
		return nil
	}
	if tk.Type != token.StringType && tk.Type != token.SingleQuoteType && tk.Type != token.DoubleQuoteType {
		return nil
	}
	if p.existsNewLineCharacter(origin) {
		return yamlerrors.NewSyntax("unexpected key name", tk)
	}
	return nil
}

// isScalarKeyToken reports whether tk is a scalar written where a key goes,
// quoted or not.
// isScalarKeyToken reports whether a key's token sits where the entry begins,
// so that the column of what follows can be measured against it.
//
// A plain or quoted key does. So does the implicit null standing for a key that
// was never written: implicitNullKeyToken copies the ':' position, and the ':'
// is where the entry begins. Without it ":\n1\n" read as {null: 1}, where the
// same document with the key written out, "k:\n1\n", is refused.
func isScalarKeyToken(tk *token.Token) bool {
	switch tk.Type {
	case token.StringType, token.SingleQuoteType, token.DoubleQuoteType, token.ImplicitNullType:
		return true
	default:
		return false
	}
}

// carriesProperty reports whether a token is an anchor or a tag: a property
// naming the node that follows it rather than a node of its own.
func carriesProperty(tk *Token) bool {
	return tk.GroupType() == TokenGroupAnchorName || tk.Type() == token.TagType
}

func (p *Parser) removeLeftWhiteSpace(src string) string {
	// CR or LF or CRLF
	return strings.TrimLeftFunc(src, func(r rune) bool {
		return r == ' ' || r == '\r' || r == '\n'
	})
}

func (p *Parser) removeRightWhiteSpace(src string) string {
	// CR or LF or CRLF
	return strings.TrimRightFunc(src, func(r rune) bool {
		return r == ' ' || r == '\r' || r == '\n'
	})
}

func (p *Parser) existsNewLineCharacter(src string) bool {
	return p.newLineCharacterNum(src) > 0
}

func (p *Parser) newLineCharacterNum(src string) int {
	var num int
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '\r':
			if len(src) > i+1 && src[i+1] == '\n' {
				i++
			}
			num++
		case '\n':
			num++
		}
	}
	return num
}

// valueContext returns the context for the value of key.
//
// parseMapKey has already built the key's path and stored it on the key node,
// so read it back rather than build the same string a second time. A flow
// collection used as a key is the one case with no path: it is given one here,
// the same way it was before.
func (p *Parser) valueContext(ctx context, key ast.MapKeyNode) context {
	if path := key.GetPathNode(); path != nil {
		return ctx.withPath(path)
	}
	return ctx.withChild(p, p.mapKeyText(key))
}

func (p *Parser) mapKeyText(n ast.Node) string {
	if n == nil {
		return ""
	}
	switch nn := n.(type) {
	case *ast.MappingKeyNode:
		return p.mapKeyText(nn.Value)
	case *ast.TagNode:
		return p.mapKeyText(nn.Value)
	case *ast.AnchorNode:
		return p.mapKeyText(nn.Value)
	case *ast.AliasNode:
		return ""
	}
	return n.GetToken().Value
}

func (p *Parser) parseMapValue(ctx context, key ast.MapKeyNode, colonTk *Token) (ast.Node, error) {
	tk := ctx.currentToken()
	if tk == nil {
		return newNullNode(ctx, ctx.addNullValueToken(colonTk))
	}

	if ctx.isComment() {
		tk = ctx.nextNotCommentToken()
	}
	keyCol := int(key.GetToken().Position.Column)
	keyLine := int(key.GetToken().Position.Line)

	if tk.Column() != keyCol && tk.Line() == keyLine && (tk.GroupType() == TokenGroupMapKey || tk.GroupType() == TokenGroupMapKeyValue) {
		// a: b:
		//    ^
		//
		// a: b: c
		//    ^
		return nil, yamlerrors.NewSyntax("mapping value is not allowed in this context", tk.RawToken())
	}

	if tk.Column() == keyCol && p.isMapToken(tk) {
		// in this case,
		// ----
		// key: <value does not defined>
		// next
		return newNullNode(ctx, ctx.insertNullToken(colonTk))
	}

	if ctx.isFlow && closesFlowEntry(tk) {
		// "[a:]", "[:]" and "[a, :]" -- the punctuation belongs to the
		// collection the pair is written in, so the pair's value is e-node.
		return newNullNode(ctx, ctx.insertNullToken(colonTk))
	}

	if next := ctx.nextNotCommentToken(); tk.Line() == keyLine && carriesProperty(tk) &&
		next != nil && next.Column() <= keyCol && !p.isMapToken(next) &&
		next.Type() != token.SequenceEntryType && next.Type() != token.DocumentHeaderType &&
		next.Type() != token.DocumentEndType {
		// key: &anchor
		// next
		// ^
		//
		// The property stands on the key's line, so the node it names is a
		// block node and has to be indented past the key like any other value.
		// Level with the key a token can only open the next entry, and this one
		// opens nothing -- so the property names nothing and the token belongs
		// nowhere. Read as the property's node it made "k: &a\n1" the mapping
		// {k: 1}, which no other implementation reads at all.
		return nil, yamlerrors.NewSyntax("value is not indented past its key", next.RawToken())
	}

	if next := ctx.nextNotCommentToken(); tk.Line() == keyLine && tk.GroupType() == TokenGroupAnchorName &&
		next.Column() == keyCol && p.isMapToken(next) {
		// in this case,
		// ----
		// key: &anchor
		// next
		//
		// A comment may stand between the two. It belongs to the entry below
		// and says nothing about where this one ends, so what follows the
		// anchor is looked for past it.
		group := newTokenGroup(TokenGroupAnchor, []*Token{tk, ctx.createImplicitNullToken(tk)})
		anchor, err := p.parseAnchor(ctx.withGroup(p, group), group)
		if err != nil {
			return nil, err
		}
		ctx.goNext()
		return anchor, nil
	}

	if tk.Column() <= keyCol && tk.GroupType() == TokenGroupAnchorName {
		// key: <value does not defined>
		// &anchor
		return nil, yamlerrors.NewSyntax("anchor is not allowed in this context", tk.RawToken())
	}
	if tk.Column() <= keyCol && tk.Type() == token.TagType {
		// key: <value does not defined>
		// !!tag
		return nil, yamlerrors.NewSyntax("tag is not allowed in this context", tk.RawToken())
	}

	if tk.Column() < keyCol {
		// in this case,
		// ----
		//   key: <value does not defined>
		// next
		return newNullNode(ctx, ctx.insertNullToken(colonTk))
	}

	if isScalarKeyToken(key.GetToken()) && tk.Column() == keyCol && tk.Line() != keyLine &&
		tk.Type() != token.SequenceEntryType {
		// a:
		// b
		// ^
		//
		// An entry's value is written further in than its key. Level with the
		// key, a token can only open the next entry -- another key, handled
		// above, or the '-' of a block sequence, which by convention sits at
		// its own key's column. Anything else has nowhere to belong, and
		// reading it as the value made "a:\nb" the mapping {a: b} where every
		// other implementation refuses the document.
		//
		// Only a plain or quoted key is measured this way. Where the key
		// carries a property or is written after a '?', its first token is the
		// property or the '?' rather than the key itself, and the column that
		// token sits at says nothing about where the entry begins.
		return nil, yamlerrors.NewSyntax("value is not indented past its key", tk.RawToken())
	}

	if tk.Line() == keyLine && tk.GroupType() == TokenGroupAnchorName &&
		ctx.nextNotCommentToken().Column() < keyCol {
		// in this case,
		// ----
		//   key: &anchor
		// next
		group := newTokenGroup(TokenGroupAnchor, []*Token{tk, ctx.createImplicitNullToken(tk)})
		anchor, err := p.parseAnchor(ctx.withGroup(p, group), group)
		if err != nil {
			return nil, err
		}
		ctx.goNext()
		return anchor, nil
	}

	value, err := p.parseToken(ctx, ctx.currentToken())
	if err != nil {
		return nil, err
	}
	if err := p.validateAnchorValueInMapOrSeq(value, keyCol); err != nil {
		return nil, err
	}
	return value, nil
}

func (p *Parser) validateAnchorValueInMapOrSeq(value ast.Node, col int) error {
	anchor, ok := value.(*ast.AnchorNode)
	if !ok {
		return nil
	}
	tag, ok := anchor.Value.(*ast.TagNode)
	if !ok {
		return nil
	}
	anchorTk := anchor.GetToken()
	tagTk := tag.GetToken()

	if anchorTk.Position.Line == tagTk.Position.Line {
		// key:
		//   &anchor !!tag
		//
		// - &anchor !!tag
		return nil
	}

	if int(tagTk.Position.Column) <= col {
		// key: &anchor
		// !!tag
		//
		// - &anchor
		// !!tag
		return yamlerrors.NewSyntax("tag is not allowed in this context", tagTk)
	}
	return nil
}

func (p *Parser) parseAnchor(ctx context, g *TokenGroup) (*ast.AnchorNode, error) {
	anchorNameGroup := g.First().Group
	anchor, err := p.parseAnchorName(ctx.withGroup(p, anchorNameGroup))
	if err != nil {
		return nil, err
	}
	ctx.goNext()
	value, err := p.parseAnchorValue(ctx, anchor)
	if err != nil {
		return nil, err
	}
	anchor.Value = value
	return anchor, nil
}

// endsValue reports whether a token closes what precedes it rather than
// starting something new: the ':' of a mapping entry, or the ',' and brackets
// that punctuate a flow collection.
// closesFlowEntry reports whether a token ends the entry it follows inside a
// flow collection, rather than standing for a node of its own.
func closesFlowEntry(tk *Token) bool {
	switch tk.Type() {
	case token.CollectEntryType, token.MappingEndType, token.SequenceEndType:
		return true
	default:
		return false
	}
}

func endsValue(tk *Token) bool {
	switch tk.Type() {
	case token.MappingValueType, token.CollectEntryType, token.MappingEndType, token.SequenceEndType:
		return true
	default:
		return false
	}
}

// startsEntry reports whether a token opens the next entry of the mapping
// around it, rather than continuing what came before.
func startsEntry(tk *Token) bool {
	switch tk.GroupType() {
	case TokenGroupMapKey, TokenGroupMapKeyValue:
		return true
	}

	// A '-' cannot be a scalar's value, so it opens the next entry of the
	// sequence around it rather than continuing this one.
	return tk.Type() == token.SequenceEntryType
}

// parseAnchorValue reads what an anchor names.
//
// An anchor with nothing after it names the empty node: "a: &x" is a valid
// document, and *x resolves to null. Refusing it made an anchor the one thing
// that could not be attached to an absent value.
func (p *Parser) parseAnchorValue(ctx context, anchor *ast.AnchorNode) (ast.Node, error) {
	if ctx.isTokenNotFound() || endsValue(ctx.currentToken()) {
		// Built rather than inserted: there is no token here to stand for the
		// null, and putting one in the stream would leave it to be read again.
		return newNullNode(ctx, ctx.createImplicitNullToken(&Token{Token: anchor.GetToken()}))
	}

	value, err := p.parseToken(ctx, ctx.currentToken())
	if err != nil {
		return nil, err
	}
	if _, ok := value.(*ast.AnchorNode); ok {
		return nil, yamlerrors.NewSyntax("anchors cannot be used consecutively", value.GetToken())
	}

	return value, nil
}

func (p *Parser) parseAnchorName(ctx context) (*ast.AnchorNode, error) {
	anchor, err := newAnchorNode(ctx, ctx.currentToken())
	if err != nil {
		return nil, err
	}
	ctx.goNext()
	if ctx.isTokenNotFound() {
		return nil, yamlerrors.NewSyntax("could not find anchor value", anchor.GetToken())
	}

	anchorName, err := p.parseScalarValue(ctx, ctx.currentToken())
	if err != nil {
		return nil, err
	}
	if anchorName == nil {
		return nil, yamlerrors.NewSyntax("unexpected anchor. anchor name is not scalar value", ctx.currentToken().RawToken())
	}
	anchor.Name = anchorName
	return anchor, nil
}

func (p *Parser) parseAlias(ctx context) (*ast.AliasNode, error) {
	alias, err := newAliasNode(ctx, ctx.currentToken())
	if err != nil {
		return nil, err
	}
	ctx.goNext()
	if ctx.isTokenNotFound() {
		return nil, yamlerrors.NewSyntax("could not find alias value", alias.GetToken())
	}

	aliasName, err := p.parseScalarValue(ctx, ctx.currentToken())
	if err != nil {
		return nil, err
	}
	if aliasName == nil {
		return nil, yamlerrors.NewSyntax("unexpected alias. alias name is not scalar value", ctx.currentToken().RawToken())
	}
	alias.Value = aliasName
	return alias, nil
}

func (p *Parser) parseLiteral(ctx context) (*ast.LiteralNode, error) {
	node, err := newLiteralNode(ctx, ctx.currentToken())
	if err != nil {
		return nil, err
	}
	ctx.goNext() // skip literal/folded token

	tk := ctx.currentToken()
	if tk == nil {
		value, err := newStringNode(ctx, &Token{Token: token.New("", "", node.Start.Position)})
		if err != nil {
			return nil, err
		}
		node.Value = value
		return node, nil
	}
	value, err := p.parseToken(ctx, tk)
	if err != nil {
		return nil, err
	}
	str, ok := value.(*ast.StringNode)
	if !ok {
		return nil, yamlerrors.NewSyntax("unexpected token. required string token", value.GetToken())
	}
	node.Value = str
	return node, nil
}

func (p *Parser) parseScalarTag(ctx context) (*ast.TagNode, error) {
	tag, err := p.parseTag(ctx)
	if err != nil {
		return nil, err
	}
	if tag.Value == nil {
		return nil, yamlerrors.NewSyntax("specified not scalar tag", tag.GetToken())
	}
	if _, ok := tag.Value.(ast.ScalarNode); !ok {
		return nil, yamlerrors.NewSyntax("specified not scalar tag", tag.GetToken())
	}
	return tag, nil
}

func (p *Parser) parseTag(ctx context) (*ast.TagNode, error) {
	tagTk := ctx.currentToken()
	tagRawTk := tagTk.RawToken()
	if handle, named := namedTagHandle(tagRawTk.Value); named {
		if _, declared := p.tagHandles[handle]; !declared {
			return nil, yamlerrors.NewSyntax(
				fmt.Sprintf("tag handle %s is not defined by a TAG directive", handle), tagRawTk)
		}
	}
	node, err := newTagNode(ctx, tagTk)
	if err != nil {
		return nil, err
	}
	ctx.goNext()

	comment := p.parseHeadComment(ctx)

	var tagValue ast.Node
	if p.secondaryTagDirective != nil {
		valueTk := ctx.currentToken()
		if valueTk == nil {
			// A secondary tag directive with nothing left to tag. Produce the
			// same implicit null parseTagValue produces for the primary case,
			// rather than building a node out of a token that is not there.
			value, err := newNullNode(ctx, ctx.createImplicitNullToken(&Token{Token: tagRawTk}))
			if err != nil {
				return nil, err
			}
			tagValue = value
		} else {
			value, err := newStringNode(ctx, valueTk)
			if err != nil {
				return nil, err
			}
			tagValue = value
		}
		node.Directive = p.secondaryTagDirective
	} else {
		value, err := p.parseTagValue(ctx, tagRawTk, ctx.currentToken())
		if err != nil {
			return nil, err
		}
		tagValue = value
	}
	if err := setHeadComment(comment, tagValue); err != nil {
		return nil, err
	}
	node.Value = tagValue
	return node, nil
}

func (p *Parser) clearTagDirectives() {
	p.tagHandles = nil
	p.secondaryTagDirective = nil
}

// namedTagHandle returns the handle a tag shorthand uses, and whether that
// handle is one a TAG directive has to define.
//
// The primary "!" and secondary "!!" handles are always available, and a
// verbatim "!<...>" tag uses none: only "!name!" has to be declared.
func namedTagHandle(value string) (string, bool) {
	if !strings.HasPrefix(value, "!") || strings.HasPrefix(value, "!<") {
		return "", false
	}

	name, _, found := strings.Cut(value[1:], "!")
	if !found || name == "" {
		return "", false
	}

	return "!" + name + "!", true
}

func (p *Parser) parseTagValue(ctx context, tagRawTk *token.Token, tk *Token) (ast.Node, error) {
	if tk == nil {
		return newNullNode(ctx, ctx.createImplicitNullToken(&Token{Token: tagRawTk}))
	}
	switch token.ReservedTagKeyword(tagRawTk.Value) {
	case token.MappingTag, token.SetTag:
		if !p.isMapToken(tk) {
			return nil, yamlerrors.NewSyntax("could not find map", tk.RawToken())
		}
		if tk.Type() == token.MappingStartType {
			return p.parseFlowMap(ctx.withFlow(true))
		}
		return p.parseMap(ctx)
	case token.IntegerTag, token.FloatTag, token.StringTag, token.BinaryTag, token.TimestampTag, token.BooleanTag, token.NullTag:
		if tk.GroupType() == TokenGroupLiteral || tk.GroupType() == TokenGroupFolded {
			return p.parseLiteral(ctx.withGroup(p, tk.Group))
		}
		if endsValue(tk) || startsEntry(tk) {
			// Nothing here is the tag's value: either punctuation closes what
			// the tag was written in, or the next entry of the enclosing
			// mapping has begun. The tag is on the empty node.
			return newTagDefaultScalarValueNode(ctx, tagRawTk)
		}
		scalar, err := p.parseScalarValue(ctx, tk)
		if err != nil {
			return nil, err
		}
		ctx.goNext()
		return scalar, nil
	case token.SequenceTag, token.OrderedMapTag:
		if tk.Type() == token.SequenceStartType {
			return p.parseFlowSequence(ctx.withFlowSequence())
		}
		return p.parseSequence(ctx)
	}
	if endsValue(tk) {
		// A tag the core schema does not resolve -- the non-specific "!", or a
		// local tag -- with punctuation after it that closes what the tag was
		// written in. The tag stands on the empty node: "[!]", "[a, !]",
		// "{a: !}". The case above says the same for the resolved tags, where
		// the empty node takes the tag's own default rather than null.
		return newTagDefaultScalarValueNode(ctx, tagRawTk)
	}

	return p.parseToken(ctx, tk)
}

func (p *Parser) parseFlowSequence(ctx context) (*ast.SequenceNode, error) {
	node, err := newSequenceNode(ctx, ctx.currentToken(), true)
	if err != nil {
		return nil, err
	}
	ctx.goNext() // skip SequenceStart token

	isFirst := true
	for ctx.next() {
		// A comment may sit anywhere separation may, including before the ','
		// that follows an element. Collect it so it can be carried, and let the
		// structural token after it decide what happens next.
		headComment := p.parseHeadComment(ctx)
		if ctx.isTokenNotFound() {
			break
		}

		tk := ctx.currentToken()
		if tk.Type() == token.SequenceEndType {
			node.End = tk.RawToken()
			node.FootComment = headComment
			break
		}

		var entryTk *Token
		if tk.Type() == token.CollectEntryType {
			if isFirst {
				return nil, yamlerrors.NewSyntax("expected sequence element, but found ','", tk.RawToken())
			}
			entryTk = tk
			if err := attachTrailingComment(ctx, entryTk, node.Values); err != nil {
				return nil, err
			}
			ctx.goNext()
			if next := p.parseHeadComment(ctx); next != nil {
				headComment = mergeComments(headComment, next)
			}
		} else if !isFirst {
			return nil, yamlerrors.NewSyntax("',' or ']' must be specified", tk.RawToken())
		}

		if tk := ctx.currentToken(); tk.Type() == token.SequenceEndType {
			// this case is here: "[ elem, ]".
			// In this case, ignore the last element and break sequence parsing.
			node.End = tk.RawToken()
			break
		}

		if ctx.isTokenNotFound() {
			break
		}

		ctx := ctx.withIndex(p, uint(len(node.Values)))
		value, err := p.parseToken(ctx, ctx.currentToken())
		if err != nil {
			return nil, err
		}
		node.Values = append(node.Values, value)
		if headComment != nil {
			node.ValueHeadComments = growHeadComments(node.ValueHeadComments, len(node.Values))
			node.ValueHeadComments[len(node.Values)-1] = headComment
		}
		seqEntry := ctx.arena.SequenceEntry(entryTk.RawToken(), value, headComment)
		if err := setLineComment(ctx, seqEntry, entryTk); err != nil {
			return nil, err
		}
		seqEntry.SetPathNode(ctx.path)
		node.Entries = append(node.Entries, seqEntry)

		isFirst = false
	}
	if node.End == nil {
		return nil, yamlerrors.NewSyntax("sequence end token ']' not found", node.Start)
	}

	// set line comment if exists. e.g.) ] # comment
	if err := setLineComment(ctx, node, ctx.currentToken()); err != nil {
		return nil, err
	}
	ctx.goNext() // skip sequence end token.
	return node, nil
}

// pendingEntry is one entry of a sequence being parsed, held until the sequence
// closes and its slices can be sized at once.
type pendingEntry struct {
	value       ast.Node
	entry       *ast.SequenceEntryNode
	headComment *ast.CommentGroupNode
}

// fillSequence gives node the entries it was built from, each slice allocated
// once at the length it ends up with.
//
// ValueHeadComments is left empty where no entry carried a head comment, which
// is every sequence of a document written without them. Readers already meet a
// short one -- a flow sequence only ever grew it as far as its last commented
// entry.
func fillSequence(node *ast.SequenceNode, entries []pendingEntry) {
	if len(entries) == 0 {
		return
	}

	node.Values = make([]ast.Node, len(entries))
	node.Entries = make([]*ast.SequenceEntryNode, len(entries))

	var commented bool
	for i, held := range entries {
		node.Values[i] = held.value
		node.Entries[i] = held.entry
		commented = commented || held.headComment != nil
	}
	if !commented {
		return
	}

	node.ValueHeadComments = make([]*ast.CommentGroupNode, len(entries))
	for i, held := range entries {
		node.ValueHeadComments[i] = held.headComment
	}
}

func (p *Parser) parseSequence(ctx context) (*ast.SequenceNode, error) {
	seqTk := ctx.currentToken()
	seqNode, err := newSequenceNode(ctx, seqTk, false)
	if err != nil {
		return nil, err
	}

	// The entries are gathered on a stack the parser reuses for every sequence,
	// so this one's slices are allocated at its own length rather than grown an
	// entry at a time. base is where this sequence's run starts.
	base := len(p.seqEntries)
	defer func() { p.seqEntries = p.seqEntries[:base] }()

	tk := seqTk
	for tk.Type() == token.SequenceEntryType && tk.Column() == seqTk.Column() {
		seqTk := tk
		headComment := p.parseHeadComment(ctx)
		ctx.goNext() // skip sequence entry token

		ctx := ctx.withIndex(p, uint(len(p.seqEntries)-base))
		value, err := p.parseSequenceValue(ctx, seqTk)
		if err != nil {
			return nil, err
		}
		seqEntry := ctx.arena.SequenceEntry(seqTk.RawToken(), value, headComment)
		if err := setLineComment(ctx, seqEntry, seqTk); err != nil {
			return nil, err
		}
		seqEntry.SetPathNode(ctx.path)
		p.seqEntries = append(p.seqEntries, pendingEntry{
			value:       value,
			entry:       seqEntry,
			headComment: headComment,
		})

		if ctx.isComment() {
			tk = ctx.nextNotCommentToken()
		} else {
			tk = ctx.currentToken()
		}
	}
	fillSequence(seqNode, p.seqEntries[base:])

	if ctx.isComment() {
		if seqTk.Column() <= ctx.currentToken().Column() {
			// If the comment is in the same or deeper column as the last element column in sequence value,
			// treat it as a footer comment for the last element.
			seqNode.FootComment = p.parseFootComment(ctx, seqTk.Column())
			if len(seqNode.Values) != 0 {
				seqNode.FootComment.SetPathNode(seqNode.Values[len(seqNode.Values)-1].GetPathNode())
			}
		}
	}
	return seqNode, nil
}

func (p *Parser) parseSequenceValue(ctx context, seqTk *Token) (ast.Node, error) {
	tk := ctx.currentToken()
	if tk == nil {
		return newNullNode(ctx, ctx.addNullValueToken(seqTk))
	}

	if ctx.isComment() {
		tk = ctx.nextNotCommentToken()
	}
	seqCol := seqTk.Column()
	seqLine := seqTk.Line()

	if tk.Column() == seqCol && tk.Type() == token.SequenceEntryType {
		// in this case,
		// ----
		// - <value does not defined>
		// -
		return newNullNode(ctx, ctx.insertNullToken(seqTk))
	}

	if next := ctx.nextNotCommentToken(); tk.Line() == seqLine && tk.GroupType() == TokenGroupAnchorName &&
		next.Column() <= seqCol {
		// in this case,
		// ----
		// - &anchor
		// -
		//
		// Whatever an anchor at the end of an entry's line names has to be
		// written inside that entry, which means further in than its '-'. A
		// token back at that column or before it belongs to something the
		// entry is part of, so the anchor names the empty node.
		//
		// A comment may stand between the two. It belongs to what comes after
		// and says nothing about where this entry ends, so what follows the
		// anchor is looked for past it.
		group := newTokenGroup(TokenGroupAnchor, []*Token{tk, ctx.createImplicitNullToken(tk)})
		anchor, err := p.parseAnchor(ctx.withGroup(p, group), group)
		if err != nil {
			return nil, err
		}
		ctx.goNext()
		return anchor, nil
	}

	if tk.Column() <= seqCol && tk.GroupType() == TokenGroupAnchorName {
		// - <value does not defined>
		// &anchor
		return nil, yamlerrors.NewSyntax("anchor is not allowed in this sequence context", tk.RawToken())
	}
	if tk.Column() <= seqCol && tk.Type() == token.TagType {
		// - <value does not defined>
		// !!tag
		return nil, yamlerrors.NewSyntax("tag is not allowed in this sequence context", tk.RawToken())
	}

	if tk.Column() < seqCol || (tk.Column() == seqCol && tk.Line() != seqLine) {
		// in this case,
		// ----
		//   - <value does not defined>
		// next
		return newNullNode(ctx, ctx.insertNullToken(seqTk))
	}

	if tk.Line() == seqLine && tk.GroupType() == TokenGroupAnchorName &&
		ctx.nextNotCommentToken().Column() < seqCol {
		// in this case,
		// ----
		//   - &anchor
		// next
		group := newTokenGroup(TokenGroupAnchor, []*Token{tk, ctx.createImplicitNullToken(tk)})
		anchor, err := p.parseAnchor(ctx.withGroup(p, group), group)
		if err != nil {
			return nil, err
		}
		ctx.goNext()
		return anchor, nil
	}

	value, err := p.parseToken(ctx, ctx.currentToken())
	if err != nil {
		return nil, err
	}
	if err := p.validateAnchorValueInMapOrSeq(value, seqCol); err != nil {
		return nil, err
	}
	return value, nil
}

func (p *Parser) parseDirective(ctx context, g *TokenGroup) (*ast.DirectiveNode, error) {
	directiveNameGroup := g.First().Group
	directive, err := p.parseDirectiveName(ctx.withGroup(p, directiveNameGroup))
	if err != nil {
		return nil, err
	}

	switch directive.Name.String() {
	case "YAML":
		if g.Len() != 2 {
			return nil, yamlerrors.NewSyntax("unexpected format YAML directive", g.First().RawToken())
		}
		valueTk := g.At(1)
		valueRawTk := valueTk.RawToken()
		value := valueRawTk.Value
		ver, exists := yamlVersionMap[value]
		if !exists {
			return nil, yamlerrors.NewSyntax(fmt.Sprintf("unknown YAML version %q", value), valueRawTk)
		}
		if p.yamlVersion != "" {
			return nil, yamlerrors.NewSyntax("YAML version has already been specified", valueRawTk)
		}
		p.yamlVersion = ver
		versionNode, err := newStringNode(ctx, valueTk)
		if err != nil {
			return nil, err
		}
		directive.Values = append(directive.Values, versionNode)
	case "TAG":
		if g.Len() != 3 {
			return nil, yamlerrors.NewSyntax("unexpected format TAG directive", g.First().RawToken())
		}
		tagKey, err := newStringNode(ctx, g.At(1))
		if err != nil {
			return nil, err
		}
		if tagKey.Value == "!!" {
			p.secondaryTagDirective = directive
		}
		if p.tagHandles == nil {
			p.tagHandles = make(map[string]struct{})
		}
		p.tagHandles[tagKey.Value] = struct{}{}
		tagValue, err := newStringNode(ctx, g.At(2))
		if err != nil {
			return nil, err
		}
		directive.Values = append(directive.Values, tagKey, tagValue)
	default:
		if g.Len() > 1 {
			for i := 1; i < g.Len(); i++ {
				tk := g.At(i)
				value, err := newStringNode(ctx, tk)
				if err != nil {
					return nil, err
				}
				directive.Values = append(directive.Values, value)
			}
		}
	}
	return directive, nil
}

func (p *Parser) parseDirectiveName(ctx context) (*ast.DirectiveNode, error) {
	directive, err := newDirectiveNode(ctx, ctx.currentToken())
	if err != nil {
		return nil, err
	}
	ctx.goNext()
	if ctx.isTokenNotFound() {
		return nil, yamlerrors.NewSyntax("could not find directive value", directive.GetToken())
	}

	directiveName, err := p.parseScalarValue(ctx, ctx.currentToken())
	if err != nil {
		return nil, err
	}
	if directiveName == nil {
		return nil, yamlerrors.NewSyntax("unexpected directive. directive name is not scalar value", ctx.currentToken().RawToken())
	}
	directive.Name = directiveName
	return directive, nil
}

func (p *Parser) parseComment(ctx context) (ast.Node, error) {
	cm := p.parseHeadComment(ctx)
	if ctx.isTokenNotFound() {
		return cm, nil
	}
	node, err := p.parseToken(ctx, ctx.currentToken())
	if err != nil {
		return nil, err
	}
	if err := setHeadComment(cm, node); err != nil {
		return nil, err
	}
	return node, nil
}

// mergeComments joins two comment groups, either of which may be absent.
func mergeComments(a, b *ast.CommentGroupNode) *ast.CommentGroupNode {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	}

	return ast.CommentGroup(append(commentTokens(a), commentTokens(b)...))
}

func commentTokens(n *ast.CommentGroupNode) []*token.Token {
	tks := make([]*token.Token, 0, len(n.Comments))
	for _, c := range n.Comments {
		tks = append(tks, c.Token)
	}

	return tks
}

// growHeadComments returns the slice sized to hold a comment for every value so
// far, keeping what is already in it.
func growHeadComments(comments []*ast.CommentGroupNode, size int) []*ast.CommentGroupNode {
	for len(comments) < size {
		comments = append(comments, nil)
	}

	return comments
}

func (p *Parser) parseHeadComment(ctx context) *ast.CommentGroupNode {
	tks := []*token.Token{}
	for ctx.isComment() {
		tks = append(tks, ctx.currentToken().RawToken())
		ctx.goNext()
	}
	if len(tks) == 0 {
		return nil
	}
	return ast.CommentGroup(tks)
}

func (p *Parser) parseFootComment(ctx context, col int) *ast.CommentGroupNode {
	tks := []*token.Token{}
	for ctx.isComment() && col <= ctx.currentToken().Column() {
		tks = append(tks, ctx.currentToken().RawToken())
		ctx.goNext()
	}
	if len(tks) == 0 {
		return nil
	}
	return ast.CommentGroup(tks)
}
