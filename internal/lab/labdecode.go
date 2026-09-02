// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package lab

import (
	"fmt"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/internal/lab/labparser"
)

// DecodeProgressive reads src into a Go value without keeping the tree.
//
// EXPERIMENT (2026-08-27). The decoder we ship parses a document into an
// ast.File, walks the whole tree into Go values, and drops the tree at the end,
// so the tree and the value are both live while the walk runs. Measured, the
// tree is 73% to 87% of what an Unmarshal holds at its peak.
//
// This converts each node as the parser finishes it and lets it go. A container
// is converted from children already converted, so what it holds is dropped as
// soon as the container has read it: only the frontier -- the nodes converted
// and not yet claimed by a parent -- is live at any moment, alongside the value
// being built.
//
// It reads what the benchmark workloads are: mappings, sequences and scalars.
// Anchors, aliases, tags and merge keys are refused rather than half-handled,
// because a wrong number would be worse than no number.
func DecodeProgressive(src []byte) (any, error) {
	d := &folder{done: map[ast.Node]any{}}

	f, err := labparser.ParseBytes(src, labparser.OnComplete(d.complete))
	if err != nil {
		return nil, err
	}
	if d.err != nil {
		return nil, d.err
	}
	if len(f.Docs) == 0 || f.Docs[0].Body == nil {
		return nil, nil
	}

	return d.claim(f.Docs[0].Body), nil
}

// folder turns nodes into Go values as they are finished.
//
// done holds what has been converted and not yet claimed by a parent. It is the
// frontier and nothing more: claim deletes as it reads, so a value lives in
// here from the moment its node is finished until the moment its parent takes
// it.
type folder struct {
	done map[ast.Node]any
	err  error
}

func (d *folder) complete(n ast.Node) {
	if d.err != nil {
		return
	}

	switch t := n.(type) {
	case *ast.MappingValueNode:
		// The entry is claimed by its mapping, so what it holds is recorded
		// against the entry and the key and value nodes are released here.
		d.done[t] = [2]any{d.keyOf(t.Key), d.claim(t.Value)}
	case *ast.MappingNode:
		m := make(map[string]any, len(t.Values))
		for _, e := range t.Values {
			pair, ok := d.claim(e).([2]any)
			if !ok {
				d.fail(fmt.Errorf("mapping entry was not folded: %T", e))

				return
			}
			k, _ := pair[0].(string)
			m[k] = pair[1]
		}
		t.Values = nil // the entries are converted; nothing needs them now
		d.done[t] = m
	case *ast.SequenceNode:
		s := make([]any, 0, len(t.Values))
		for _, v := range t.Values {
			s = append(s, d.claim(v))
		}
		t.Values = nil
		d.done[t] = s
	case *ast.AnchorNode, *ast.AliasNode, *ast.TagNode, *ast.MergeKeyNode:
		d.fail(fmt.Errorf("%s is outside what this experiment reads", n.Type()))
	default:
		d.done[n] = scalar(n)
	}
}

// claim takes a node's value and forgets it was ever here.
func (d *folder) claim(n ast.Node) any {
	if n == nil {
		return nil
	}
	v, ok := d.done[n]
	if !ok {
		// A scalar the parser built without reporting it -- a key, or a null
		// standing in for an absent value.
		return scalar(n)
	}
	delete(d.done, n)

	return v
}

func (d *folder) keyOf(n ast.Node) any {
	v := d.claim(n)
	if s, ok := v.(string); ok {
		return s
	}

	return fmt.Sprint(v)
}

func (d *folder) fail(err error) {
	if d.err == nil {
		d.err = err
	}
}

// scalar reads a scalar node's value. Every scalar shape answers GetValue, and
// a node that is not one carries nothing to read.
func scalar(n ast.Node) any {
	// A literal holds its folded and chomped text in the string node inside it;
	// its own GetValue answers the block as it was written, header and all.
	if l, ok := n.(*ast.LiteralNode); ok {
		return l.Value.GetValue()
	}
	if s, ok := n.(ast.ScalarNode); ok {
		return s.GetValue()
	}

	return nil
}
