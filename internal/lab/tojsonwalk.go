// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package lab

import (
	"fmt"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/internal/lab/labparser"
)

// ToJSONWalk converts a YAML document to JSON from a walk of the parse.
//
// EXPERIMENT (2026-08-30). It holds no node and asks the arena to keep nothing.
// Everything it needs comes with each step: what collection it is in, how deep,
// which entry of that collection this is, and where the token stands. A
// separator goes before an entry whose index is not the first; a brace opens on
// Enter and closes on Leave.
//
// It reads mappings, sequences, scalars, anchors and aliases. An anchor writes
// its node as usual and keeps the text; an alias writes that text again. A tag
// or a merge key is refused rather than half-handled -- merging one mapping
// into another means reading back keys already written, which is what a
// converter that never looks back cannot do.
//
// Node paths are off: a converter never reads one, and building the trie is the
// largest thing a walk would otherwise hold -- 43% of the peak on the 3.7 MB
// golang_source, 48% on citm_catalog.
func ToJSONWalk(src []byte) ([]byte, error) {
	w := &jsonWalker{}

	file, err := labparser.New(labparser.OmitNodePaths()).Walk(src, w)
	if err != nil {
		return nil, err
	}
	if w.err != nil {
		return nil, w.err
	}
	if len(file.Docs) == 0 || w.out == nil {
		return []byte("null"), nil
	}

	return w.out, nil
}

// jsonWalker writes JSON as the walk hands each part of the document over.
//
// An anchor is written like any other node and its text kept: open records
// where in out the anchored node starts, and closing the anchor copies what was
// written there into named. An alias appends the copy. Anchors nest, so open is
// a stack.
type jsonWalker struct {
	out   []byte
	open  []anchorMark
	named map[string][]byte
	err   error
}

// anchorMark is one anchor still being written: its name, and where in out the
// node it names begins.
type anchorMark struct {
	name string
	at   int
}

func (w *jsonWalker) Enter(node ast.Node, at labparser.Step) bool {
	if w.err != nil {
		return false
	}

	w.separate(at)

	switch n := node.(type) {
	case *ast.MappingNode:
		w.out = append(w.out, '{')
	case *ast.SequenceNode:
		w.out = append(w.out, '[')
	case *ast.AnchorNode:
		if at.Key {
			// "{&n x}" anchors the key itself. Nothing inside it comes over --
			// a key is read without its parts going over on their own -- so the
			// text is written from the node here and kept under the name.
			w.anchorKey(n, at)

			return false
		}
		w.openAnchor(n)
	case *ast.AliasNode:
		w.writeAlias(n, at)
	case *ast.TagNode, *ast.MergeKeyNode:
		w.err = fmt.Errorf("%s is outside what this experiment reads", node.Type())

		return false
	default:
		w.scalar(node, at)
	}

	return true
}

func (w *jsonWalker) Leave(node ast.Node, _ labparser.Step) {
	switch node.(type) {
	case *ast.MappingNode:
		w.out = append(w.out, '}')
	case *ast.SequenceNode:
		w.out = append(w.out, ']')
	case *ast.AnchorNode:
		w.closeAnchor()
	}
}

// openAnchor records the name and where the node it names will start.
func (w *jsonWalker) openAnchor(node *ast.AnchorNode) {
	name := ""
	if node.Name != nil && node.Name.GetToken() != nil {
		name = node.Name.GetToken().Value
	}
	if name == "" {
		w.err = fmt.Errorf("anchor at %s has no name", &node.GetToken().Position)

		return
	}
	w.open = append(w.open, anchorMark{name: name, at: len(w.out)})
}

// anchorKey writes an anchor standing as a mapping key, and keeps its text.
//
// The key is a JSON string, and that string is what an alias naming the anchor
// writes later: "&n x" as a key makes "*n" elsewhere read "x".
func (w *jsonWalker) anchorKey(node *ast.AnchorNode, at labparser.Step) {
	name := ""
	if node.Name != nil && node.Name.GetToken() != nil {
		name = node.Name.GetToken().Value
	}
	if node.Value == nil {
		w.err = fmt.Errorf("anchor &%s at %s stands as a mapping key and names nothing",
			name, &node.GetToken().Position)

		return
	}

	from := len(w.out)
	w.scalar(node.Value, at)
	if name != "" {
		if w.named == nil {
			w.named = make(map[string][]byte)
		}
		w.named[name] = append([]byte(nil), w.out[from:]...)
	}
}

// closeAnchor keeps what the anchored node wrote.
//
// The text is copied: out grows as the rest of the document is written and a
// slice of it would not survive the next append.
func (w *jsonWalker) closeAnchor() {
	if len(w.open) == 0 {
		return
	}
	mark := w.open[len(w.open)-1]
	w.open = w.open[:len(w.open)-1]

	if w.named == nil {
		w.named = make(map[string][]byte)
	}
	w.named[mark.name] = append([]byte(nil), w.out[mark.at:]...)
}

// writeAlias writes again what the anchor of the same name wrote.
func (w *jsonWalker) writeAlias(node *ast.AliasNode, at labparser.Step) {
	name := ""
	if node.Value != nil && node.Value.GetToken() != nil {
		name = node.Value.GetToken().Value
	}
	text, ok := w.named[name]
	if !ok {
		w.err = fmt.Errorf("alias *%s at %s names no anchor written before it",
			name, &node.GetToken().Position)

		return
	}
	if at.Key {
		// A JSON key is a string, and what an anchor wrote is a value of
		// whatever shape the anchored node had.
		w.err = fmt.Errorf("alias *%s at %s stands as a mapping key, which this converter does not write",
			name, &node.GetToken().Position)

		return
	}
	w.out = append(w.out, text...)
}

// separate writes what goes between two entries of a collection.
//
// A mapping hands a key over and then its value, so the step says which this
// is: a comma before a key that is not the first, and a colon before a value.
func (w *jsonWalker) separate(at labparser.Step) {
	switch at.In {
	case labparser.KindMapping:
		switch {
		case at.Key && at.Index > 0:
			w.out = append(w.out, ',')
		case !at.Key:
			w.out = append(w.out, ':')
		}
	case labparser.KindSequence:
		if at.Index > 0 {
			w.out = append(w.out, ',')
		}
	}
}

// scalar writes one value, as a string where it stands as a mapping's key.
func (w *jsonWalker) scalar(node ast.Node, at labparser.Step) {
	value := scalar(node)
	if at.Key {
		w.out = appendString(w.out, fmt.Sprint(value))

		return
	}

	w.out = appendScalar(w.out, value)
}
