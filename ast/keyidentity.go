// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"strings"

	"github.com/go-openapi/go-yaml/token"
)

// KeyIdentity writes what a mapping key resolves to, so that two keys the
// document spelled differently come out the same where 3.2.1.1 makes them one
// key.
//
// The text a document wrote is not the identity. "[a]", "[ a ]", "[a,]" and
// '["a"]' are four spellings of one sequence holding one string, and naming a
// key by its own characters reads them as four keys. Naming it by its first
// token is worse: every sequence key becomes "[", every block scalar key "|-",
// and each collides with every other.
//
// A scalar's identity is its type and that type's canonical spelling, which is
// [token.KeyName]'s rule -- "7" and "007" are one integer and so one key, where
// "1" and "1.0" are an integer and a float and so two. A collection's is its
// kind and the identities of what it holds, in order.
//
// It reads the built node, so it can only be asked once the node is built: a
// key is a run of tokens more often than not, and the parser has none of its
// children while it is still cutting them.
//
// The empty string is no identity at all, which [Unnamed] reports. An alias
// whose anchor the parse could not fill is the case that reaches it.
func KeyIdentity(n Node) string {
	var b strings.Builder
	if !writeKeyIdentity(&b, n, 0) {
		return ""
	}

	return b.String()
}

// Unnamed reports whether KeyIdentity gave up on a node.
func Unnamed(identity string) bool { return identity == "" }

// writeKeyIdentity appends n's identity to b, and reports whether it had one.
//
// depth bounds the descent: an alias may name a collection that holds the
// alias, and "&a [ *a ]" is a document the parser reads.
func writeKeyIdentity(b *strings.Builder, n Node, depth int) bool {
	if n == nil || depth > maxIdentityDepth {
		return false
	}

	switch nn := n.(type) {
	case *MappingKeyNode:
		return writeKeyIdentity(b, nn.Value, depth)
	case *AnchorNode:
		// The anchor names the node; it does not change what the node is.
		return writeKeyIdentity(b, nn.Value, depth)
	case *AliasNode:
		if nn.Target == nil {
			return false
		}

		return writeKeyIdentity(b, nn.Target, depth+1)
	case *TagNode:
		return writeTaggedIdentity(b, nn, depth)
	case *LiteralNode:
		// A literal or folded block scalar is a string whatever it spells, and
		// its own token is the header -- "|-" or ">-". Falling through to the
		// token named every block scalar key after its header, so two of them
		// collided however differently they read.
		if nn.Value == nil {
			writeScalarIdentity(b, "", token.KeyString)

			return true
		}
		writeScalarIdentity(b, nn.Value.Value, token.KeyString)

		return true
	case *SequenceNode:
		b.WriteString("seq(")
		for i, v := range nn.Values {
			if i > 0 {
				b.WriteByte(',')
			}
			if !writeKeyIdentity(b, v, depth+1) {
				return false
			}
		}
		b.WriteByte(')')

		return true
	case *MappingNode:
		return writeEntriesIdentity(b, nn.Values, depth)
	case *MappingValueNode:
		// A mapping written as one entry, which is how "{a: 1}" arrives where
		// the braces hold a single pair.
		return writeEntriesIdentity(b, []*MappingValueNode{nn}, depth)
	}

	tk := n.GetToken()
	if tk == nil {
		return false
	}
	name, kind := token.KeyName(tk.Value, tk.Type)
	writeScalarIdentity(b, name, kind)

	return true
}

// writeEntriesIdentity writes a mapping's identity from its entries.
func writeEntriesIdentity(b *strings.Builder, entries []*MappingValueNode, depth int) bool {
	b.WriteString("map(")
	for i, e := range entries {
		if i > 0 {
			b.WriteByte(',')
		}
		if e == nil || !writeKeyIdentity(b, e.Key, depth+1) {
			return false
		}
		b.WriteByte(':')
		if !writeKeyIdentity(b, e.Value, depth+1) {
			return false
		}
	}
	b.WriteByte(')')

	return true
}

// writeTaggedIdentity writes the identity of a node under a tag.
//
// A tag names the type, so it names the identity: "!!str 1" is the string and
// "1" is the integer, and the two are two keys. A tag the schema does not
// resolve leaves the node to speak for itself.
func writeTaggedIdentity(b *strings.Builder, n *TagNode, depth int) bool {
	res := n.Resolve()
	if res.Verdict != TagResolved {
		return writeKeyIdentity(b, n.Value, depth)
	}

	switch res.Tag {
	case token.StringTag:
		writeScalarIdentity(b, res.Text, token.KeyString)
	case token.NullTag:
		writeScalarIdentity(b, "null", token.KeyNull)
	case token.BooleanTag:
		name, kind := token.KeyName(res.Text, token.BoolType)
		writeScalarIdentity(b, name, kind)
	case token.IntegerTag:
		name, kind := token.KeyName(res.Text, token.IntegerType)
		writeScalarIdentity(b, name, kind)
	case token.FloatTag:
		name, kind := token.KeyName(res.Text, token.FloatType)
		writeScalarIdentity(b, name, kind)
	default:
		// A tag naming a kind -- !!seq, !!map, !!binary, !!timestamp -- or one
		// the application defined. What it tags is what it is.
		return writeKeyIdentity(b, n.Value, depth)
	}

	return true
}

// writeScalarIdentity writes a scalar's kind and canonical spelling.
//
// The kind is written too, so that the string "1" and the integer 1 do not
// share an identity: 3.2.1.1 makes them two keys.
func writeScalarIdentity(b *strings.Builder, name string, kind token.KeyKind) {
	b.WriteString(kind.String())
	b.WriteByte('/')
	b.WriteString(name)
}

// maxIdentityDepth bounds the descent, which follows aliases.
const maxIdentityDepth = 64
