// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/go-openapi/go-yaml/ast"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
	"github.com/go-openapi/go-yaml/parser"
	"github.com/go-openapi/go-yaml/token"
)

// valueBuilder folds a document into Go values as the parse reaches each node,
// so that a document is read once rather than built into a tree and walked.
//
// It is codec.jsonWriter's walk with a different fold: the same Enter and Leave
// over the same nodes, keeping Go values where the converter keeps JSON text.
// Four things differ, and they are what a JSON writer has no use for -- a
// number too wide for a machine word stays a *big.Int or *big.Float, a
// "!!timestamp" becomes a time.Time, "!!binary" becomes []byte, and an infinity
// stays one where JSON has to refuse it.
type valueBuilder struct {
	// docs holds the value of each document body, in stream order.
	docs []any
	// stack is what is being filled, innermost last: a mapping, a sequence, or
	// a property standing around the node it names.
	stack []buildFrame
	// named holds what each anchor of the document named, for an alias to name
	// again. It is emptied at each document, since an anchor belongs to one.
	named map[string]any
	err   error
}

type frameKind uint8

const (
	// frameMapping and frameSequence are collections being filled.
	frameMapping frameKind = iota
	frameSequence
	// frameProperty is an anchor, a tag or a "?" standing around one node. It
	// takes the value that node builds and hands it on transformed.
	frameProperty
)

type buildFrame struct {
	kind frameKind
	at   parser.Step

	// mapping
	m       map[string]any
	key     string
	hasKey  bool
	merging bool

	// sequence
	seq []any

	// property
	node     ast.Node
	received any
	got      bool
}

// Enter is called before anything a node holds.
func (b *valueBuilder) Enter(node ast.Node, at parser.Step) bool {
	if b.err != nil {
		return false
	}

	if at.Depth == 0 && at.In == parser.KindNone {
		if _, isDirective := node.(*ast.DirectiveNode); isDirective {
			// A directive opens a document of its own and holds no value.
			return false
		}
		b.named = nil
	}

	if isMergeKey(node) {
		// "<<" names no key of its own: what it brings in is folded into the
		// mapping when the value below it arrives.
		if n := len(b.stack); n > 0 && b.stack[n-1].kind == frameMapping {
			b.stack[n-1].merging = true
		}

		return false
	}

	switch n := node.(type) {
	case *ast.MappingNode:
		b.stack = append(b.stack, buildFrame{kind: frameMapping, at: at, m: map[string]any{}})
	case *ast.SequenceNode:
		// An empty sequence is an empty slice and not a nil one, which is what
		// the tree-walking decoder gives and what a caller comparing against
		// "[]any{}" expects.
		b.stack = append(b.stack, buildFrame{kind: frameSequence, at: at, seq: []any{}})
	case *ast.AnchorNode, *ast.TagNode, *ast.MappingKeyNode:
		b.stack = append(b.stack, buildFrame{kind: frameProperty, at: at, node: node})
	case *ast.AliasNode:
		b.deliver(b.aliasValue(n), node, at)
	case *ast.CommentGroupNode:
		return false
	default:
		v, err := scalarValue(node)
		if err != nil {
			b.fail(err)

			return false
		}
		b.deliver(v, node, at)
	}

	return true
}

// Leave is called once everything a node holds has been.
func (b *valueBuilder) Leave(node ast.Node, at parser.Step) {
	if b.err != nil {
		return
	}

	switch node.(type) {
	case *ast.MappingNode, *ast.SequenceNode, *ast.AnchorNode, *ast.TagNode, *ast.MappingKeyNode:
	default:
		return
	}

	frame := b.stack[len(b.stack)-1]
	b.stack = b.stack[:len(b.stack)-1]

	switch frame.kind {
	case frameMapping:
		// The parser hangs the repeats on the mapping as it closes, so they are
		// read here rather than as it opened.
		if err := refuseDuplicateKeys(node); err != nil {
			b.fail(err)

			return
		}
		b.deliver(frame.m, node, frame.at)
	case frameSequence:
		b.deliver(frame.seq, node, frame.at)
	case frameProperty:
		v, err := b.closeProperty(frame)
		if err != nil {
			b.fail(err)

			return
		}
		b.deliver(v, node, frame.at)
	}
}

// closeProperty turns what a property's node built into what the property makes
// of it.
func (b *valueBuilder) closeProperty(frame buildFrame) (any, error) {
	value := frame.received
	if !frame.got {
		// A tag on a scalar hands nothing over -- parseScalarValue builds the
		// value without going through parseToken -- so it is read from the node
		// here, as jsonWriter.closeTag does. An anchor between a tag and its
		// scalar is the same shape. A property standing on nothing reads as the
		// empty node.
		v, err := propertyValue(frame.node)
		if err != nil {
			return nil, err
		}
		value = v
	}

	switch n := frame.node.(type) {
	case *ast.AnchorNode:
		if name := anchorName(n.Name); name != "" {
			if b.named == nil {
				b.named = map[string]any{}
			}
			b.named[name] = value
		}

		return value, nil
	case *ast.TagNode:
		return taggedWalkValue(n, value)
	default:
		// A "?" key stands around the node it addresses the entry by.
		return value, nil
	}
}

// propertyValue reads the node a property stands on, for the scalars the parse
// builds without handing over.
func propertyValue(n ast.Node) (any, error) {
	var inner ast.Node
	switch t := n.(type) {
	case *ast.AnchorNode:
		inner = t.Value
	case *ast.TagNode:
		inner = t.Value
	case *ast.MappingKeyNode:
		inner = t.Value
	}
	if inner == nil {
		return nil, nil
	}
	if _, scalar := inner.(ast.ScalarNode); !scalar {
		return nil, nil
	}

	return scalarValue(inner)
}

// aliasValue is what an alias names, which the anchor recorded as it closed.
func (b *valueBuilder) aliasValue(n *ast.AliasNode) any {
	name := anchorName(n.Value)
	if v, named := b.named[name]; named {
		return v
	}

	b.fail(yamlerrors.NewUnknownAnchor(name, n.GetToken()))

	return nil
}

// deliver puts a value where the node that built it belongs.
func (b *valueBuilder) deliver(v any, node ast.Node, at parser.Step) {
	if n := len(b.stack); n > 0 {
		top := &b.stack[n-1]
		switch top.kind {
		case frameProperty:
			top.received, top.got = v, true

			return
		case frameMapping:
			if at.Key {
				// Named from the node and not from the value, so that the type
				// spells it: mapKeyString writes an integer in decimal and a
				// float with the point that tells it from one.
				top.key, top.hasKey = mapKeyString(node, v), true

				return
			}
			if top.merging {
				b.merge(top, v, node)
				top.merging = false

				return
			}
			top.m[top.key] = v
			top.key, top.hasKey = "", false

			return
		case frameSequence:
			top.seq = append(top.seq, v)

			return
		}
	}

	b.docs = append(b.docs, v)
}

// merge folds what a "<<" names into the mapping holding it.
//
// The merged entries are defaults: a key the mapping writes itself wins,
// whichever side of the "<<" it stands on. Writing them without overwriting
// gives that either way -- a "<<" written first fills an empty mapping and the
// mapping's own keys replace what they repeat, and one written last finds those
// keys already there.
//
// A "<<" takes a mapping or a sequence of them, and an earlier mapping of the
// sequence wins over a later one.
func (b *valueBuilder) merge(top *buildFrame, v any, node ast.Node) {
	switch t := v.(type) {
	case map[string]any:
		for key, value := range t {
			if _, held := top.m[key]; !held {
				top.m[key] = value
			}
		}
	case []any:
		for _, one := range t {
			b.merge(top, one, node)
		}
	case nil:
	default:
		// "<<" takes a mapping or a sequence of them, and nothing else.
		b.fail(yamlerrors.NewUnexpectedNodeType(node.Type(), ast.MappingType, node.GetToken()))
	}
}

func (b *valueBuilder) fail(err error) {
	if b.err == nil {
		b.err = err
	}
}

// scalarValue is what a scalar node denotes.
func scalarValue(n ast.Node) (any, error) {
	switch t := n.(type) {
	case *ast.NullNode:
		return nil, nil
	case *ast.LiteralNode:
		if t.Value == nil {
			return "", nil
		}

		return t.Value.Value, nil
	case ast.ScalarNode:
		return t.GetValue(), nil
	default:
		return nil, yamlerrors.NewUnexpectedNodeType(n.Type(), ast.StringType, n.GetToken())
	}
}

// taggedWalkValue is what a tag makes of the value its node built.
func taggedWalkValue(n *ast.TagNode, value any) (any, error) {
	res := n.Resolve()

	switch res.Verdict {
	case ast.TagUnresolved:
		return value, nil
	case ast.TagKindMismatch:
		return nil, yamlerrors.NewSyntax(
			fmt.Sprintf("%s names a kind this node is not", res.Tag), n.GetToken())
	case ast.TagValueMismatch:
		if res.Lax {
			return res.Text, nil
		}

		return nil, yamlerrors.NewSyntax(
			fmt.Sprintf("cannot read %q as %s", res.Text, res.Tag), n.Value.GetToken())
	}

	if res.Empty {
		return tagZero(res.Tag)
	}

	// What each tag makes of the node is where this parts company with the JSON
	// converter: a number too wide for a machine word stays a *big.Int or
	// *big.Float, "!!binary" is bytes and "!!timestamp" a time.Time, none of
	// which JSON has a spelling for.
	switch res.Tag {
	case token.StringTag:
		return res.Text, nil
	case token.NullTag:
		return nil, nil
	case token.IntegerTag:
		return castToInteger(value), nil
	case token.FloatTag:
		return castToFloatValue(value), nil
	case token.BooleanTag:
		// Resolve has agreed there is a boolean to read, in whatever case the
		// document wrote it.
		b, _ := token.ParseBool(strings.ToLower(res.Text))

		return b, nil
	case token.BinaryTag:
		// Resolve has read the text as base64 already, so this cannot fail.
		return base64.StdEncoding.DecodeString(res.Text)
	case token.TimestampTag:
		t, _ := ast.ParseTimestamp(res.Text)

		return t, nil
	default:
		// A tag naming a kind -- !!seq, !!map, !!set, !!omap, !!merge -- takes
		// the node as it stands.
		return value, nil
	}
}

// WalkValue reads src by walking it, without building a tree. EXPERIMENT.
func WalkValue(src []byte) (any, error) {
	b := &valueBuilder{}
	if _, err := parser.New(parser.WithOmitNodePaths()).Walk(src, b); err != nil {
		return nil, err
	}
	if b.err != nil {
		return nil, b.err
	}
	if len(b.docs) == 0 {
		return nil, nil
	}

	return b.docs[0], nil
}
