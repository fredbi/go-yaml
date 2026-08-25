// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser2

import (
	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/token"
)

func newMappingNode(ctx context, tk *token.Token, isFlow bool, values []*ast.MappingValueNode) (*ast.MappingNode, error) {
	node := ctx.arena.Mapping(tk, isFlow, values)
	node.SetPathNode(ctx.path)
	return node, nil
}

func newMappingValueNode(ctx context, colonTk, entryTk *Token, key ast.MapKeyNode, value ast.Node) (*ast.MappingValueNode, error) {
	node := ctx.arena.MappingValue(colonTk.RawToken(), key, value)
	node.SetPathNode(ctx.path)
	node.CollectEntry = entryTk.RawToken()
	// entryTk is the ',' that comes *before* this entry, so a comment hanging on
	// it was written about the entry before this one and is attached there.
	if _, explicit := key.(*ast.MappingKeyNode); explicit {
		// An explicit key's group ends on the key itself rather than on a ':',
		// so colonTk is the key and a comment on it is the key's own -- already
		// attached there. Carrying it over would write it twice, once on the
		// "?" line and once on the ':' line, and the document would gain a
		// comment on every cycle.
		return node, nil
	}
	if key.GetToken().Position.Line == value.GetToken().Position.Line {
		// originally key was commented, but now that null value has been added, value must be commented.
		if err := setLineComment(ctx, value, colonTk); err != nil {
			return nil, err
		}
	} else {
		if err := setLineComment(ctx, key, colonTk); err != nil {
			return nil, err
		}
	}
	return node, nil
}

func newMappingKeyNode(ctx context, tk *Token) (*ast.MappingKeyNode, error) {
	node := ast.MappingKey(tk.RawToken())
	node.SetPathNode(ctx.path)
	if err := setLineComment(ctx, node, tk); err != nil {
		return nil, err
	}
	return node, nil
}

func newAnchorNode(ctx context, tk *Token) (*ast.AnchorNode, error) {
	node := ast.Anchor(tk.RawToken())
	node.SetPathNode(ctx.path)
	if err := setLineComment(ctx, node, tk); err != nil {
		return nil, err
	}
	return node, nil
}

func newAliasNode(ctx context, tk *Token) (*ast.AliasNode, error) {
	node := ast.Alias(tk.RawToken())
	node.SetPathNode(ctx.path)
	if err := setLineComment(ctx, node, tk); err != nil {
		return nil, err
	}
	return node, nil
}

func newDirectiveNode(ctx context, tk *Token) (*ast.DirectiveNode, error) {
	node := ast.Directive(tk.RawToken())
	node.SetPathNode(ctx.path)
	if err := setLineComment(ctx, node, tk); err != nil {
		return nil, err
	}
	return node, nil
}

func newMergeKeyNode(ctx context, tk *Token) (*ast.MergeKeyNode, error) {
	node := ast.MergeKey(tk.RawToken())
	node.SetPathNode(ctx.path)
	if err := setLineComment(ctx, node, tk); err != nil {
		return nil, err
	}
	return node, nil
}

func newNullNode(ctx context, tk *Token) (*ast.NullNode, error) {
	node := ctx.arena.Null(tk.RawToken())
	node.SetPathNode(ctx.path)
	if err := setLineComment(ctx, node, tk); err != nil {
		return nil, err
	}
	return node, nil
}

func newBoolNode(ctx context, tk *Token) (*ast.BoolNode, error) {
	node := ctx.arena.Bool(tk.RawToken())
	node.SetPathNode(ctx.path)
	if err := setLineComment(ctx, node, tk); err != nil {
		return nil, err
	}
	return node, nil
}

func newIntegerNode(ctx context, tk *Token) (*ast.IntegerNode, error) {
	node := ctx.arena.Integer(tk.RawToken())
	node.SetPathNode(ctx.path)
	if err := setLineComment(ctx, node, tk); err != nil {
		return nil, err
	}
	return node, nil
}

func newFloatNode(ctx context, tk *Token) (*ast.FloatNode, error) {
	node := ctx.arena.Float(tk.RawToken())
	node.SetPathNode(ctx.path)
	if err := setLineComment(ctx, node, tk); err != nil {
		return nil, err
	}
	return node, nil
}

func newInfinityNode(ctx context, tk *Token) (*ast.InfinityNode, error) {
	node := ast.Infinity(tk.RawToken())
	node.SetPathNode(ctx.path)
	if err := setLineComment(ctx, node, tk); err != nil {
		return nil, err
	}
	return node, nil
}

func newNanNode(ctx context, tk *Token) (*ast.NanNode, error) {
	node := ast.Nan(tk.RawToken())
	node.SetPathNode(ctx.path)
	if err := setLineComment(ctx, node, tk); err != nil {
		return nil, err
	}
	return node, nil
}

func newStringNode(ctx context, tk *Token) (*ast.StringNode, error) {
	node := ctx.arena.String(tk.RawToken())
	node.SetPathNode(ctx.path)
	if err := setLineComment(ctx, node, tk); err != nil {
		return nil, err
	}
	return node, nil
}

func newLiteralNode(ctx context, tk *Token) (*ast.LiteralNode, error) {
	node := ast.Literal(tk.RawToken())
	node.SetPathNode(ctx.path)
	if err := setLineComment(ctx, node, tk); err != nil {
		return nil, err
	}
	return node, nil
}

func newTagNode(ctx context, tk *Token) (*ast.TagNode, error) {
	node := ast.Tag(tk.RawToken())
	node.SetPathNode(ctx.path)
	if err := setLineComment(ctx, node, tk); err != nil {
		return nil, err
	}
	return node, nil
}

func newSequenceNode(ctx context, tk *Token, isFlow bool) (*ast.SequenceNode, error) {
	node := ctx.arena.Sequence(tk.RawToken(), isFlow)
	node.SetPathNode(ctx.path)
	if isFlow {
		// tk is the '[' that opens the collection, so a comment on it was
		// written about the collection. A block sequence opens on the '-' of
		// its first entry, and a comment there is that entry's -- read as the
		// whole sequence's it came back twice, once at the head and once where
		// it was written.
		if err := setLineComment(ctx, node, tk); err != nil {
			return nil, err
		}
	}

	return node, nil
}

func newTagDefaultScalarValueNode(ctx context, tag *token.Token) (ast.ScalarNode, error) {
	pos := tag.Position
	pos.Column++

	var (
		tk   *Token
		node ast.ScalarNode
	)
	switch token.ReservedTagKeyword(tag.Value) {
	case token.IntegerTag:
		tk = &Token{Token: token.New("0", "0", pos)}
		n, err := newIntegerNode(ctx, tk)
		if err != nil {
			return nil, err
		}
		node = n
	case token.FloatTag:
		tk = &Token{Token: token.New("0", "0", pos)}
		n, err := newFloatNode(ctx, tk)
		if err != nil {
			return nil, err
		}
		node = n
	case token.StringTag, token.BinaryTag, token.TimestampTag:
		tk = &Token{Token: token.New("", "", pos)}
		n, err := newStringNode(ctx, tk)
		if err != nil {
			return nil, err
		}
		node = n
	case token.BooleanTag:
		tk = &Token{Token: token.New("false", "false", pos)}
		n, err := newBoolNode(ctx, tk)
		if err != nil {
			return nil, err
		}
		node = n
	case token.NullTag:
		tk = &Token{Token: token.New("null", "null", pos)}
		n, err := newNullNode(ctx, tk)
		if err != nil {
			return nil, err
		}
		node = n
	default:
		// A tag the core schema does not resolve -- the non-specific "!", or a
		// local tag -- leaves the empty node unresolved, which is null.
		tk = &Token{Token: token.New("null", "null", pos)}
		n, err := newNullNode(ctx, tk)
		if err != nil {
			return nil, err
		}
		node = n
	}
	ctx.insertToken(tk)
	ctx.goNext()
	return node, nil
}

func setLineComment(ctx context, node ast.Node, tk *Token) error {
	if tk == nil || tk.LineComment == nil {
		return nil
	}
	comment := ast.CommentGroup([]*token.Token{tk.LineComment})
	comment.SetPathNode(ctx.path)
	if err := node.SetComment(comment); err != nil {
		return err
	}
	return nil
}

func setHeadComment(cm *ast.CommentGroupNode, value ast.Node) error {
	if cm == nil {
		return nil
	}
	switch n := value.(type) {
	case *ast.MappingNode:
		if len(n.Values) != 0 && value.GetComment() == nil {
			cm.SetPathNode(n.Values[0].GetPathNode())
			return n.Values[0].SetComment(cm)
		}
	case *ast.MappingValueNode:
		cm.SetPathNode(n.GetPathNode())
		return n.SetComment(cm)
	}
	cm.SetPathNode(value.GetPathNode())
	return value.SetComment(cm)
}
