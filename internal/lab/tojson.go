// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package lab

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/internal/lab/labparser"
)

// ToJSONProgressive converts a YAML document to JSON without keeping the tree.
//
// EXPERIMENT (2026-08-30). It stands for the shape most callers actually have:
// read a node, write something, move on. A converter never looks back, so it
// never needs the document it has already passed -- and codec.ToJSON gives it
// one anyway, along with a second whole copy of the document as Go values. That
// is what this measures against: codec.ToJSON unmarshals into an ordered map
// and marshals that, so tokens, tree and value all stand at once.
//
// Here a node is turned into its JSON text as the parser finishes it, and a
// container joins the text of its children and releases them. Only the frontier
// is live -- what has been written and not yet claimed by an enclosing
// container -- alongside the output being built.
//
// It reads what the benchmark workloads are: mappings, sequences and scalars.
// Anchors, aliases, tags and merge keys are refused rather than half-handled.
func ToJSONProgressive(src []byte) ([]byte, error) {
	w := &jsonFolder{done: map[ast.Node][]byte{}}

	file, err := labparser.ParseBytes(src, 0, labparser.OnComplete(w.complete))
	if err != nil {
		return nil, err
	}
	if w.err != nil {
		return nil, w.err
	}
	if len(file.Docs) == 0 || file.Docs[0].Body == nil {
		return []byte("null"), nil
	}

	return w.claim(file.Docs[0].Body), nil
}

// jsonFolder writes each node as JSON as the parser finishes it.
//
// done holds the text written for a node and not yet claimed by the container
// around it. claim deletes as it reads, so a node's text lives here from the
// moment the node is finished to the moment its container takes it.
type jsonFolder struct {
	done map[ast.Node][]byte
	err  error
}

func (w *jsonFolder) complete(n ast.Node) {
	if w.err != nil {
		return
	}

	switch t := n.(type) {
	case *ast.MappingValueNode:
		out := w.appendKey(nil, t.Key)
		out = append(out, ':')
		w.done[t] = append(out, w.claim(t.Value)...)
	case *ast.MappingNode:
		out := []byte{'{'}
		for i, entry := range t.Values {
			if i > 0 {
				out = append(out, ',')
			}
			out = append(out, w.claim(entry)...)
		}
		t.Values = nil // written; nothing needs the entries now
		w.done[t] = append(out, '}')
	case *ast.SequenceNode:
		out := []byte{'['}
		for i, value := range t.Values {
			if i > 0 {
				out = append(out, ',')
			}
			out = append(out, w.claim(value)...)
		}
		t.Values = nil
		w.done[t] = append(out, ']')
	case *ast.AnchorNode, *ast.AliasNode, *ast.TagNode, *ast.MergeKeyNode:
		w.fail(fmt.Errorf("%s is outside what this experiment reads", n.Type()))
	default:
		w.done[n] = appendScalar(nil, scalar(n))
	}
}

// claim takes what was written for a node and forgets the node was ever here.
func (w *jsonFolder) claim(n ast.Node) []byte {
	if n == nil {
		return []byte("null")
	}

	out, ok := w.done[n]
	if !ok {
		// A scalar the parser built without reporting it -- a key, or a null
		// standing in for an absent value.
		return appendScalar(nil, scalar(n))
	}
	delete(w.done, n)

	return out
}

// appendKey writes a mapping key, which JSON holds as a string whatever YAML
// read it as.
func (w *jsonFolder) appendKey(out []byte, n ast.Node) []byte {
	text := w.claim(n)
	if len(text) != 0 && text[0] == '"' {
		return append(out, text...)
	}

	return appendString(out, string(text))
}

func (w *jsonFolder) fail(err error) {
	if w.err == nil {
		w.err = err
	}
}

// appendScalar writes one YAML scalar as JSON.
func appendScalar(out []byte, v any) []byte {
	switch t := v.(type) {
	case nil:
		return append(out, "null"...)
	case string:
		return appendString(out, t)
	case bool:
		return strconv.AppendBool(out, t)
	case int:
		return strconv.AppendInt(out, int64(t), 10)
	case int64:
		return strconv.AppendInt(out, t, 10)
	case uint64:
		return strconv.AppendUint(out, t, 10)
	case float64:
		return strconv.AppendFloat(out, t, 'g', -1, 64)
	default:
		text, err := json.Marshal(t)
		if err != nil {
			return append(out, "null"...)
		}

		return append(out, text...)
	}
}

// appendString writes a JSON string. Anything needing an escape goes through
// encoding/json rather than being escaped here, so the rules are the standard
// library's and not a second set of them.
func appendString(out []byte, s string) []byte {
	if plainJSONString(s) {
		out = append(out, '"')
		out = append(out, s...)

		return append(out, '"')
	}

	text, err := json.Marshal(s)
	if err != nil {
		return append(out, `""`...)
	}

	return append(out, text...)
}

// plainJSONString reports whether s may be written between quotes as it stands.
func plainJSONString(s string) bool {
	for i := range len(s) {
		if c := s[i]; c < 0x20 || c > 0x7e || c == '"' || c == '\\' {
			return false
		}
	}

	return true
}
