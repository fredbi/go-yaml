// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"github.com/go-openapi/go-yaml/token"
)

// KeyName is the text a mapping key addresses an entry by, and the type it
// resolved to.
//
// [token.KeyName] holds the rule for a scalar's own spelling -- "7" and "007"
// are one integer written two ways, "1" and "1.0" are an integer and a float --
// and says why it sits in token rather than in any one caller. This is the
// other half: the walk from the node a document wrote down to the scalar that
// rule applies to.
//
// It looks through everything that stands in front of a key without being the
// key. An explicit "?" and an anchor name the node and say nothing about its
// type. A tag says what the type is, so it is resolved rather than stripped:
// "!!float 1" is the float 1.0 and is named "1.0", where stripping the tag
// would read the token "1" and name it after an integer. A block scalar is a
// string whatever it spells, and its own token is the "|-" header.
//
// The empty string under [token.KeyOther] means no name, which a caller has to
// answer for itself:
//
//   - an alias, because what it names depends on where the caller stands. A
//     tree reads [AliasNode.Target]; a parse still walking has to have kept the
//     anchored node's identity as it went.
//   - a sequence or a mapping, which has no scalar spelling at all. Compare
//     those with [KeyIdentity].
//
// Three packages named a key from a node before this: ast for [KeyIdentity],
// the parser for its duplicate check, and codec for the string a Go map or a
// JSON member is keyed by. Only the last left out the tag and the alias, so
// "!!float 1.0: x" came back keyed "1" -- a float in the integers' namespace,
// where an entry keyed "1" then displaces it.
func KeyName(n Node) (string, token.KeyKind) {
	return keyNameAt(n, 0)
}

// keyNameAt is [KeyName] with the descent bounded: a tag stands over a node
// that may carry another.
func keyNameAt(n Node, depth int) (string, token.KeyKind) {
	if n == nil || depth > maxKeyNameDepth {
		return "", token.KeyOther
	}

	switch nn := n.(type) {
	case *MappingKeyNode:
		return keyNameAt(nn.Value, depth+1)
	case *AnchorNode:
		return keyNameAt(nn.Value, depth+1)
	case *TagNode:
		return taggedKeyName(nn, depth)
	case *StringNode:
		// The node's own text, not the token's: a double-quoted key holds what
		// the escapes resolved to.
		return nn.Value, token.KeyString
	case *LiteralNode:
		if nn.Value == nil {
			return "", token.KeyString
		}

		return nn.Value.Value, token.KeyString
	case *AliasNode, *SequenceNode, *MappingNode, *MappingValueNode:
		return "", token.KeyOther
	}

	tk := n.GetToken()
	if tk == nil {
		return "", token.KeyOther
	}

	return token.KeyName(tk.Value, tk.Type)
}

// taggedKeyName names a key from the tag standing on it, for the tags that name
// one of the types a key is told apart by.
//
// A tag the schema does not resolve leaves the node to speak for itself, and so
// does one naming a kind -- !!seq, !!map, !!binary -- or one the application
// declared.
func taggedKeyName(n *TagNode, depth int) (string, token.KeyKind) {
	res := n.Resolve()
	if res.Verdict != TagResolved {
		return keyNameAt(n.Value, depth+1)
	}

	switch res.Tag {
	case token.StringTag:
		return res.Text, token.KeyString
	case token.NullTag:
		return "null", token.KeyNull
	case token.BooleanTag:
		return token.KeyName(res.Text, token.BoolType)
	case token.IntegerTag:
		return token.KeyName(res.Text, token.IntegerType)
	case token.FloatTag:
		return token.KeyName(res.Text, token.FloatType)
	default:
		return keyNameAt(n.Value, depth+1)
	}
}

// maxKeyNameDepth bounds the descent through the properties standing in front
// of a key.
const maxKeyNameDepth = 64
