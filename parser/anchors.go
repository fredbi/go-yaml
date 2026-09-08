// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"slices"

	"github.com/go-openapi/go-yaml/ast"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
	"github.com/go-openapi/go-yaml/token"
)

// The parser keeps the anchors of the document it is reading and points every
// alias at the node its anchor names, so a consumer reads
// [ast.AliasNode.Target] rather than collecting anchors of its own. It does not
// substitute: the alias stays an alias in the tree, and expanding it is the
// consumer's to do.
//
// Three rules decide what an alias may name, and all three fall out of reading
// the document once, front to back:
//
//   - An alias names the anchor declared before it. §3.2.2.2: "an alias event
//     refers to the most recent event in the serialization having the specified
//     anchor". So a name declared later, or never, is
//     [yamlerrors.ErrUnknownAnchor], and a name declared twice resolves to
//     whichever declaration the alias stands after.
//   - An anchor belongs to the document it was written in. The table is emptied
//     at each document boundary, so an alias naming an earlier document's
//     anchor names nothing -- and the nodes the table pins are let go of there.
//   - An anchor names its node from where the node starts, not from where it
//     ends, so "&x [ *x ]" resolves and the tree it builds holds a cycle. The
//     parser reads that document because YAML's representation is a graph.
//     Whether a cycle can be held is the consumer's question and it is asked
//     later: the decoder refuses one, because a Go value built by walking has
//     nowhere to put it.

// openAnchor is an anchor whose node is being read: the name, and the node that
// will hold what the name stands for once it has been read.
type openAnchor struct {
	name string
	node *ast.AnchorNode
}

// cyclicAlias is an alias that named an anchor still being read. Its target is
// set once the document is done, which is the first moment the anchored node
// exists.
type cyclicAlias struct {
	alias  *ast.AliasNode
	anchor *ast.AnchorNode
	// tagged is the tag written before the anchor, where there is one. The node
	// the alias names carries both properties, so it is the tag node and not
	// what the anchor holds. See retagAnchor.
	tagged *ast.TagNode
}

// openAnchorName records that the node name stands for is being read.
//
// It is a stack and not a set because anchors nest -- "&x [&y 1]" -- and the
// names come off in the order they went on. Looking one up is a walk down it,
// over as many entries as there are anchors open at once, which is the
// document's nesting and not its length.
func (p *Parser) openAnchorName(name string, node *ast.AnchorNode) {
	p.openAnchors = append(p.openAnchors, openAnchor{name: name, node: node})
}

// keepAnchor enters the node an anchor names, and closes the name.
func (p *Parser) keepAnchor(name string, value ast.Node) {
	p.dropAnchorName()

	if name == "" || value == nil {
		// Nothing an alias can reach. The scanner refuses a '&' with no name
		// after it, so this is the guard and not the path.
		return
	}
	if p.anchors == nil {
		p.anchors = make(map[string]ast.Node, 4)
	}
	p.anchors[name] = value
	p.keepAnchorIdentity(name, value)
}

// anchorIdentity is what an anchor's node resolves to, in the two forms a key
// is told apart by.
//
// text and kind are what [Parser.mapKeyIdentity] reads off a scalar, and are
// empty for a collection. identity is [ast.KeyIdentity]'s reading of the node,
// which answers for both. An alias key is checked in whichever store its anchor
// belongs to, so "&a x: 1" and a later "*a" meet among the scalar keys and
// "&a [1]: 1" and its alias among the built ones.
type anchorIdentity struct {
	text     string
	kind     token.KeyKind
	identity string
}

// keepAnchorIdentity records what the anchored node resolves to, for an alias
// that later stands as a mapping key.
//
// The identity is taken here because this is the last moment the node is whole
// on a walk. Parser.keepsNothing holds the cells while the anchor is being read,
// so the node has its children now; the mapping around it rewinds past them
// once its entry closes, and [ast.AliasNode.Target] then points at a scrubbed
// cell -- "&a [a, b]" read back as "seq()".
//
// Two strings per anchor, and nothing is retained: the node itself goes.
func (p *Parser) keepAnchorIdentity(name string, value ast.Node) {
	if p.allowDuplicateMapKey {
		return
	}

	text, kind := p.mapKeyIdentity(value)
	identity := ast.KeyIdentity(value)
	if unnamedKey(text, kind) && ast.Unnamed(identity) {
		return
	}
	if p.anchorIdentities == nil {
		p.anchorIdentities = make(map[string]anchorIdentity, 4)
	}
	p.anchorIdentities[name] = anchorIdentity{text: text, kind: kind, identity: identity}
}

// dropAnchorName closes the innermost open name.
func (p *Parser) dropAnchorName() {
	if n := len(p.openAnchors); n > 0 {
		p.openAnchors = p.openAnchors[:n-1]
	}
}

// openAnchorNode returns the anchor of this name whose node is being read right
// now, which is what an alias inside that node names.
func (p *Parser) openAnchorNode(name string) (*ast.AnchorNode, bool) {
	// Innermost first: "&x [&x 1, *x]" names the inner one, which is the most
	// recent declaration and the one §3.2.2.2 asks for.
	for _, open := range slices.Backward(p.openAnchors) {
		if open.name == name {
			return open.node, true
		}
	}

	return nil, false
}

// resolveAlias points the alias at the node its name stands for.
func (p *Parser) resolveAlias(alias *ast.AliasNode, name string, tk *token.Token) error {
	if anchor, open := p.openAnchorNode(name); open {
		if p.jsonCompatible {
			// JSON is a tree written out in full, so it has no spelling for a
			// node that reaches back into itself, wherever the cycle closes.
			// Caught here rather than at the conversion, which read the alias
			// as naming an anchor it had not finished writing and reported it
			// as missing.
			return yamlerrors.NewNotJSON("a cycle cannot be written as JSON", tk)
		}

		// The alias stands inside what its own anchor names. The anchored node
		// is not built yet -- a sequence is built once its entries are read --
		// so the target is filled at the document's end, where it exists.
		p.cyclicAliases = append(p.cyclicAliases, cyclicAlias{alias: alias, anchor: anchor})

		return nil
	}
	if node, declared := p.anchors[name]; declared {
		alias.Target = node

		return nil
	}
	if node, declared := p.declaredAnchors[name]; declared {
		// An anchor [WithAnchors] published, which no document of this stream
		// wrote and an alias may still name.
		alias.Target = node

		return nil
	}

	return yamlerrors.NewUnknownAnchor(name, tk)
}

// retagAnchor points an anchor written after a tag at the tagged node.
//
// §6.9 lets a node's tag and anchor stand in either order and means the same by
// both. Written anchor first the tree is Anchor over Tag over the value, and the
// anchor names the tagged node. Written tag first it is Tag over Anchor over the
// value, and the anchor named the value with the tag stripped off it, so
// "a: !!int &a1 \"5\"" read 5 at a and "5" at "b: *a1" -- one node, read as a
// number where it stands and as a string through an alias to it.
//
// The tree keeps the order the document wrote, so it still renders as it was
// written. Only what the name stands for changes.
func (p *Parser) retagAnchor(tagged *ast.TagNode) {
	anchor, anchored := tagged.Value.(*ast.AnchorNode)
	if !anchored {
		return
	}

	name := anchorNameOf(anchor.Name)
	if name == "" {
		return
	}
	if p.anchors[name] == anchor.Value {
		// Still the entry this anchor made. A later "&a1" on another node has
		// replaced it, and that one is what the name means from there on.
		p.anchors[name] = tagged
	}

	// An alias inside the anchored node resolved before this tag was built, so
	// it holds the anchor rather than the node standing around it.
	for i := range p.cyclicAliases {
		if p.cyclicAliases[i].anchor == anchor {
			p.cyclicAliases[i].tagged = tagged
		}
	}
}

// takeAnchors returns what the document just read declared, and empties the
// table for the next one.
func (p *Parser) takeAnchors() map[string]ast.Node {
	for _, cyclic := range p.cyclicAliases {
		if cyclic.tagged != nil {
			cyclic.alias.Target = cyclic.tagged

			continue
		}
		cyclic.alias.Target = cyclic.anchor.Value
	}
	p.cyclicAliases = p.cyclicAliases[:0]

	anchors := p.anchors
	p.anchors = nil
	p.anchorIdentities = nil
	p.openAnchors = p.openAnchors[:0]

	return anchors
}
