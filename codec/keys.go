// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec

import (
	"fmt"

	"github.com/go-openapi/go-yaml/ast"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
	"github.com/go-openapi/go-yaml/token"
)

// keyName returns the text a mapping key addresses its entry by, and the type
// it resolved to.
//
// [token.KeyName] holds the rule, so the parser telling one key from another
// and this naming an entry give one answer for one document.
//
// A string is taken from the node rather than from its token: the node holds
// the text the quotes and escapes were read into, which is what the entry is
// addressed by.
func keyName(n ast.Node) (string, token.KeyKind) {
	switch t := n.(type) {
	case *ast.StringNode:
		return t.Value, token.KeyString
	case *ast.LiteralNode:
		if t.Value == nil {
			return "", token.KeyString
		}

		return t.Value.Value, token.KeyString
	case *ast.NullNode, *ast.BoolNode, *ast.IntegerNode, *ast.FloatNode,
		*ast.InfinityNode, *ast.NanNode:
		tk := n.GetToken()
		if tk == nil {
			return "", token.KeyOther
		}

		return token.KeyName(tk.Value, tk.Type)
	default:
		return "", token.KeyOther
	}
}

// refuseDuplicateKeys reports the first key the parse recorded as repeated on
// the mapping.
//
// The parse records repeats and refuses nothing, so a document that repeats a
// key can still be read, rendered and linted; what to do about one is the
// load's. §3.2.1.1 makes it an error, so the load refuses it.
//
// parser.WithAllowDuplicateMapKey records none at all, and then the last entry
// written wins because that is what filling a map does.
func refuseDuplicateKeys(n ast.Node) error {
	m, ok := n.(*ast.MappingNode)
	if !ok || len(m.Duplicates) == 0 {
		return nil
	}

	d := m.Duplicates[0]

	return yamlerrors.NewDuplicateKey(
		fmt.Sprintf("mapping key %q already defined at [%d:%d]", d.Name, d.FirstAt.Line, d.FirstAt.Column),
		keyTokenAt(m, d.At),
	)
}

// keyTokenAt returns the key the mapping wrote at pos for the complaint to
// point at, and the mapping's own token where the entry is not to hand.
//
// The parse keeps the position and not the token: a token held against a
// mapping outlives the entry that carried it, and a mapping of 5,000 keys would
// hold 5,000 tokens spread over the whole document.
func keyTokenAt(m *ast.MappingNode, pos token.Position) *token.Token {
	for _, v := range m.Values {
		if v == nil || v.Key == nil {
			continue
		}
		if tk := v.Key.GetToken(); tk != nil && tk.Position.Line == pos.Line && tk.Position.Column == pos.Column {
			return tk
		}
	}

	return m.GetToken()
}
