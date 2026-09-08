// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec

import (
	"fmt"

	"github.com/go-openapi/go-yaml/ast"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
	"github.com/go-openapi/go-yaml/token"
)

// refuseDuplicateKeys reports the first key the parse recorded as repeated on
// the mapping.
//
// The parse records repeats and refuses nothing, so a document that repeats a
// key can still be read, rendered and linted; what to do about one is the
// load's. §3.2.1.1 makes it an error, so the load refuses it.
//
// parser.WithAllowDuplicateMapKey records none at all, and then the last entry
// written wins because that is what filling a map does.
//
// A parse under parser.WithJSONCompatible records one more pair: two keys that
// YAML tells apart and JSON does not, such as "1" and "\"1\"". Those are two
// keys rather than a repeat, so the complaint is ErrNotJSON.
func refuseDuplicateKeys(n ast.Node) error {
	m, ok := n.(*ast.MappingNode)
	if !ok || len(m.Duplicates) == 0 {
		return nil
	}

	d := m.Duplicates[0]
	if d.JSONNameOnly {
		return yamlerrors.NewNotJSON(
			fmt.Sprintf("two keys write the JSON member %q, first defined at [%d:%d]",
				d.Name, d.FirstAt.Line, d.FirstAt.Column),
			keyTokenAt(m, d),
		)
	}

	return yamlerrors.NewDuplicateKey(
		fmt.Sprintf("mapping key %q already defined at [%d:%d]", d.Name, d.FirstAt.Line, d.FirstAt.Column),
		keyTokenAt(m, d),
	)
}

// keyTokenAt returns the key the mapping wrote at pos for the complaint to
// point at, and the mapping's own token where the entry is not to hand.
//
// The parse keeps the position and not the token: a token held against a
// mapping outlives the entry that carried it, and a mapping of 5,000 keys would
// hold 5,000 tokens spread over the whole document.
func keyTokenAt(m *ast.MappingNode, d ast.DuplicateKey) *token.Token {
	pos := d.At

	for _, v := range m.Values {
		if v == nil || v.Key == nil {
			continue
		}
		if tk := v.Key.GetToken(); tk != nil && tk.Position.Line == pos.Line && tk.Position.Column == pos.Column {
			return tk
		}
	}

	// A walk hands the mapping over without gathering its entries, so the token
	// is not to hand. The position was recorded when the repeat was read, and
	// token.New fills in the rest: a token built as a struct literal carries no
	// spans, its EndLine reads 0, and printer.PrintErrorSource then draws a
	// window that closes before it opens and shows nothing.
	return token.New(d.Name, d.Name, pos)
}
