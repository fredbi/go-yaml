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
// It reads mappings, sequences and scalars. An anchor, an alias or a tag is
// refused rather than half-handled.
func ToJSONWalk(src []byte) ([]byte, error) {
	w := &jsonWalker{}

	file, err := labparser.New().Walk(src, w)
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
type jsonWalker struct {
	out []byte
	err error
}

func (w *jsonWalker) Enter(node ast.Node, at labparser.Step) bool {
	if w.err != nil {
		return false
	}

	w.separate(at)

	switch node.(type) {
	case *ast.MappingNode:
		w.out = append(w.out, '{')
	case *ast.SequenceNode:
		w.out = append(w.out, '[')
	case *ast.AnchorNode, *ast.AliasNode, *ast.TagNode, *ast.MergeKeyNode:
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
	}
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
