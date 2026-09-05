// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/go-openapi/go-yaml/ast"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
	"github.com/go-openapi/go-yaml/parser"
	"github.com/go-openapi/go-yaml/token"
)

// ToJSON converts a YAML document to the JSON that holds the same values.
//
// The document is written as the parser finishes each node: a scalar becomes
// its JSON text, a collection joins the text of its children and forgets them.
// Only the frontier is live -- what has been written and not yet taken by an
// enclosing collection -- so a caller pays for the output and the tree, not for
// a second copy of the document as Go values.
//
// A stream of several documents converts its first, which is the one
// [Unmarshal] reads.
func ToJSON(src []byte) ([]byte, error) {
	w := jsonFolder{done: make(map[ast.Node][]byte)}

	file, err := parser.ParseBytes(src, parser.OmitNodePaths(), parser.OnComplete(w.complete))
	if err != nil {
		return nil, err
	}
	if w.err != nil {
		return nil, w.err
	}
	body, ok := firstDocumentBody(file)
	if !ok {
		return []byte("null"), nil
	}

	return w.claim(body), nil
}

// firstDocumentBody is the body of the first document of the stream that holds
// a value.
//
// A "%YAML" or "%TAG" line opens a document of its own, ahead of the one it
// applies to, so the directives are stepped over. An empty document is not: it
// holds no value and converts to null, which is what the stream says.
func firstDocumentBody(file *ast.File) (ast.Node, bool) {
	for _, doc := range file.Docs {
		if _, isDirective := doc.Body.(*ast.DirectiveNode); isDirective {
			continue
		}

		return doc.Body, doc.Body != nil
	}

	return nil, false
}

// jsonFolder writes each node as JSON as the parser finishes it.
//
// done holds the text written for a node and not yet taken by the collection
// around it. claim deletes as it reads, so a node's text lives here from the
// moment the node is finished to the moment its collection takes it.
//
// named and anchored record what an anchor stands for: the text its node wrote,
// which an alias writes again, and the node itself, which a merge key reads
// entries out of.
type jsonFolder struct {
	done     map[ast.Node][]byte
	named    map[string][]byte
	anchored map[string]ast.Node
	err      error
}

// complete writes n as JSON. The parser calls it in completion order, so every
// child of n has already been written and is waiting in done.
func (w *jsonFolder) complete(n ast.Node) {
	if w.err != nil {
		return
	}

	switch t := n.(type) {
	case *ast.MappingValueNode:
		w.done[t] = w.entry(nil, t)
	case *ast.MappingNode:
		w.done[t] = w.mapping(t)
	case *ast.SequenceNode:
		w.done[t] = w.sequence(t)
	case *ast.AnchorNode:
		w.done[t] = w.anchor(t)
	case *ast.AliasNode:
		w.done[t] = w.alias(t)
	case *ast.TagNode:
		w.done[t] = w.tagged(t)
	case *ast.DocumentNode:
		// An anchor belongs to the document it was written in: a stream is a
		// run of documents, each independent of the rest, and an alias naming
		// an anchor from an earlier one has nothing to name.
		clear(w.named)
		clear(w.anchored)
	case *ast.DirectiveNode, *ast.CommentNode, *ast.CommentGroupNode:
		// Not values. The document hands its body over to ToJSON directly.
	default:
		w.done[n] = w.render(n)
	}
}

// claim takes what was written for n and forgets n was ever here.
//
// A node the parser built without reporting it -- a key, or a null standing in
// for an absent value -- is written here instead.
func (w *jsonFolder) claim(n ast.Node) []byte {
	if n == nil {
		return []byte("null")
	}
	if out, ok := w.done[n]; ok {
		delete(w.done, n)

		return out
	}

	return w.render(n)
}

// entry writes one mapping entry, "key":value.
func (w *jsonFolder) entry(out []byte, t *ast.MappingValueNode) []byte {
	out = w.appendKey(out, t.Key)
	out = append(out, ':')

	return append(out, w.claim(t.Value)...)
}

// mapping writes a mapping, joining the entries it has already written.
func (w *jsonFolder) mapping(t *ast.MappingNode) []byte {
	if hasMergeKey(t) {
		return w.mergedMapping(t)
	}

	out := []byte{'{'}
	for i, entry := range t.Values {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, w.claim(entry)...)
	}

	return append(out, '}')
}

// sequence writes a sequence, joining the values it has already written.
func (w *jsonFolder) sequence(t *ast.SequenceNode) []byte {
	out := []byte{'['}
	for i, value := range t.Values {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, w.claim(value)...)
	}

	return append(out, ']')
}

// anchor writes the node it names and keeps the text under the name, so an
// alias can write it again and a merge key can read the node's entries.
func (w *jsonFolder) anchor(t *ast.AnchorNode) []byte {
	out := w.claim(t.Value)

	name := anchorName(t.Name)
	if name == "" {
		return out
	}
	if w.named == nil {
		w.named = make(map[string][]byte)
		w.anchored = make(map[string]ast.Node)
	}
	w.named[name] = out
	w.anchored[name] = t.Value

	return out
}

// alias writes again what the anchor it names wrote.
//
// An anchor is readable from the moment it closes, so an alias naming one that
// has not closed -- a forward reference, or a node aliasing itself -- names
// nothing and is refused, as it is when the document is read into Go values.
func (w *jsonFolder) alias(t *ast.AliasNode) []byte {
	name := anchorName(t.Value)
	if out, ok := w.named[name]; ok {
		return out
	}
	w.fail(yamlerrors.NewSyntax(fmt.Sprintf("could not find alias %q", name), t.Value.GetToken()))

	return []byte("null")
}

func (w *jsonFolder) fail(err error) {
	if w.err == nil {
		w.err = err
	}
}

// anchorName reads the name off an anchor or an alias.
func anchorName(n ast.Node) string {
	s, ok := n.(*ast.StringNode)
	if !ok {
		return ""
	}

	return s.Value
}

// appendKey writes a mapping key, which JSON holds as a string whatever YAML
// read it as.
func (w *jsonFolder) appendKey(out []byte, n ast.Node) []byte {
	if key, ok := w.scalarKeyText(n); ok {
		w.claim(n)

		return appendJSONString(out, key)
	}

	text := w.claim(n)
	if len(text) != 0 && text[0] == '"' {
		return append(out, text...)
	}

	return appendJSONString(out, string(text))
}

// scalarKeyText is a scalar key as the string a mapping holds it under.
//
// JSON keys are strings, so the key is the value written out: 4.0 and 4 address
// the same entry and are both "4". A collection used as a key has no such
// spelling and is written as its own JSON instead.
func (w *jsonFolder) scalarKeyText(n ast.Node) (string, bool) {
	n = w.keyNode(n)
	if !isScalarNode(n) {
		return "", false
	}

	switch t := jsonScalarOf(n).(type) {
	case nil:
		return "", false
	case string:
		return t, true
	default:
		return fmt.Sprint(t), true
	}
}

// keyNode is the node a key stands for: "? k" is written for k, and an alias
// key addresses what its anchor names.
func (w *jsonFolder) keyNode(n ast.Node) ast.Node {
	if k, ok := n.(*ast.MappingKeyNode); ok {
		n = k.Value
	}
	if alias, ok := n.(*ast.AliasNode); ok {
		if named := w.anchored[anchorName(alias.Value)]; named != nil {
			return named
		}
	}

	return n
}

// isScalarNode reports whether n is one of the nodes holding a single value.
//
// The ast.ScalarNode interface is wider than that -- an anchor and an alias
// answer GetValue with their own name -- so the types are named here.
func isScalarNode(n ast.Node) bool {
	switch n.(type) {
	case *ast.StringNode, *ast.IntegerNode, *ast.FloatNode, *ast.BoolNode,
		*ast.NullNode, *ast.InfinityNode, *ast.NanNode, *ast.LiteralNode:
		return true
	default:
		return false
	}
}

// render writes n from the tree, taking any child the parser already reported.
//
// complete goes through here, and so does claim for the nodes the parser builds
// without reporting -- so the two arms cannot drift: what a node converts to is
// written once, here.
func (w *jsonFolder) render(n ast.Node) []byte {
	switch t := n.(type) {
	case *ast.MappingValueNode:
		return w.entry(nil, t)
	case *ast.MappingNode:
		return w.mapping(t)
	case *ast.SequenceNode:
		return w.sequence(t)
	case *ast.AnchorNode:
		return w.anchor(t)
	case *ast.AliasNode:
		return w.alias(t)
	case *ast.TagNode:
		out := w.tagged(t)
		// "!!int &c 4" tags the value the anchor names. The anchor was written
		// before the tag was read, so what it stands for is corrected here.
		w.retagAnchor(t, out)

		return out
	case *ast.DocumentNode:
		return w.claim(t.Body)
	case *ast.MappingKeyNode:
		return w.claim(t.Value)
	case *ast.FloatNode:
		return appendJSONFloat(nil, t.GetValue())
	default:
		return appendJSONScalar(nil, jsonScalarOf(n))
	}
}

// hasMergeKey reports whether any entry of t is a "<<".
func hasMergeKey(t *ast.MappingNode) bool {
	for _, entry := range t.Values {
		if _, ok := entry.Key.(*ast.MergeKeyNode); ok {
			return true
		}
	}

	return false
}

// mergedMapping writes a mapping holding a "<<" entry.
//
// The merged keys go where the "<<" stands and the mapping's own win: a key
// written here overrides the one merged in, and the first merge to name a key
// beats a later one. JSON has no way to write a key twice, so the entries are
// gathered and deduplicated before any of them is written.
func (w *jsonFolder) mergedMapping(t *ast.MappingNode) []byte {
	local := make(map[string]struct{}, len(t.Values))
	for _, entry := range t.Values {
		if _, ok := entry.Key.(*ast.MergeKeyNode); ok {
			continue
		}
		local[string(w.keyText(entry.Key))] = struct{}{}
	}

	var (
		out  = []byte{'{'}
		seen = make(map[string]struct{}, len(t.Values))
	)
	write := func(key, value []byte) {
		if _, ok := seen[string(key)]; ok {
			return
		}
		seen[string(key)] = struct{}{}
		if len(out) > 1 {
			out = append(out, ',')
		}
		out = appendJSONString(out, string(key))
		out = append(out, ':')
		out = append(out, value...)
	}

	for _, entry := range t.Values {
		if _, ok := entry.Key.(*ast.MergeKeyNode); ok {
			w.eachMerged(entry.Value, func(key, value []byte) {
				if _, ok := local[string(key)]; ok {
					return
				}
				write(key, value)
			})

			continue
		}
		delete(w.done, entry)
		write(w.keyText(entry.Key), w.claim(entry.Value))
	}

	return append(out, '}')
}

// eachMerged calls do with every entry a "<<" value brings in.
//
// The value is an alias naming a mapping, a mapping written out, or a sequence
// of either -- earlier first, since an earlier merge wins.
func (w *jsonFolder) eachMerged(value ast.Node, do func(key, value []byte)) {
	switch t := w.resolveAlias(value).(type) {
	case *ast.MappingNode:
		for _, entry := range t.Values {
			if _, ok := entry.Key.(*ast.MergeKeyNode); ok {
				w.eachMerged(entry.Value, do)

				continue
			}
			do(w.keyText(entry.Key), w.render(entry.Value))
		}
	case *ast.SequenceNode:
		for _, elem := range t.Values {
			w.eachMerged(elem, do)
		}
	case *ast.MappingValueNode:
		do(w.keyText(t.Key), w.render(t.Value))
	case nil:
		w.fail(yamlerrors.NewSyntax("could not find the anchor a merge key names", value.GetToken()))
	default:
		w.fail(yamlerrors.NewUnexpectedNodeType(t.Type(), ast.MappingType, t.GetToken()))
	}
}

// resolveAlias follows an alias to the node its anchor names.
func (w *jsonFolder) resolveAlias(n ast.Node) ast.Node {
	alias, ok := n.(*ast.AliasNode)
	if !ok {
		return n
	}

	return w.anchored[anchorName(alias.Value)]
}

// keyText is a mapping key as the string JSON holds it, unquoted.
func (w *jsonFolder) keyText(n ast.Node) []byte {
	if key, ok := w.scalarKeyText(n); ok {
		w.claim(n)

		return []byte(key)
	}

	return w.claim(n)
}

// tagged writes a node carrying a tag.
//
// The eight tags that name a scalar type are read off the text the scalar was
// written with, so "!!str 1" is the string "1" and "!!int \"3\"" the number 3.
// Every other tag -- !!seq, !!map, !!set, !!omap, !!timestamp, !!merge and any
// the document defines itself -- writes the value it stands on.
func (w *jsonFolder) tagged(t *ast.TagNode) []byte {
	if t.Start == nil {
		return w.claim(t.Value)
	}
	if t.Directive != nil {
		// A "%TAG" line gives the handle a prefix of the document's own, so
		// "!!int" under one names the document's type and not YAML's.
		return w.claim(t.Value)
	}

	text, isScalar := taggedText(t.Value)
	if !isScalar {
		return w.claim(t.Value)
	}

	switch token.ReservedTagKeyword(t.Start.Value) {
	case token.StringTag:
		w.claim(t.Value)

		return appendJSONString(nil, text)
	case token.IntegerTag:
		w.claim(t.Value)

		return strconv.AppendInt(nil, taggedInteger(text), 10)
	case token.FloatTag:
		w.claim(t.Value)
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			f = 0
		}

		return appendJSONFloat(nil, f)
	case token.BooleanTag:
		w.claim(t.Value)
		b, ok := token.ParseBool(text)
		if !ok {
			// The tag says boolean whatever the text is, so a spelling neither
			// schema resolves is tried once more in lower case: "!!bool Yes"
			// and "!!bool YES" are the same request.
			b, ok = token.ParseBool(strings.ToLower(text))
		}
		if !ok {
			w.fail(yamlerrors.NewSyntax(fmt.Sprintf("cannot convert %q to boolean", text), t.Value.GetToken()))

			return []byte("false")
		}

		return strconv.AppendBool(nil, b)
	case token.NullTag:
		w.claim(t.Value)

		return []byte("null")
	case token.BinaryTag:
		w.claim(t.Value)

		return appendJSONBinary(nil, text)
	default:
		return w.claim(t.Value)
	}
}

// retagAnchor writes down what an anchor standing under a tag now stands for.
func (w *jsonFolder) retagAnchor(t *ast.TagNode, out []byte) {
	anchor, ok := t.Value.(*ast.AnchorNode)
	if !ok {
		return
	}
	if name := anchorName(anchor.Name); name != "" {
		w.named[name] = out
	}
}

// taggedInteger reads the whole number a "!!int" stands on. A text that is not
// a number at all counts as zero, and one written as a float keeps its whole
// part: "!!int 3.7" is 3.
func taggedInteger(text string) int64 {
	if n, err := strconv.ParseInt(text, 0, 64); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(text, 64); err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) {
		return int64(f)
	}

	return 0
}

// taggedText is the text a tagged scalar was written with, and whether the tag
// stands on a scalar at all.
//
// An anchor between the tag and the scalar is stepped over: "!!int &c 4" tags
// the 4. A collection, an alias, or a tag on nothing is not a scalar and keeps
// whatever it converted to.
func taggedText(n ast.Node) (string, bool) {
	if a, ok := n.(*ast.AnchorNode); ok {
		return taggedText(a.Value)
	}
	if l, ok := n.(*ast.LiteralNode); ok {
		if l.Value == nil {
			return "", false
		}

		return l.Value.Value, true
	}

	if _, isNull := n.(*ast.NullNode); isNull {
		// "!!str" on its own tags the empty string, not the word "null".
		return "", true
	}
	if !isScalarNode(n) {
		return "", false
	}

	tk := n.GetToken()
	if tk == nil {
		return "", false
	}

	return tk.Value, true
}

// jsonScalarOf is the Go value a scalar node holds.
func jsonScalarOf(n ast.Node) any {
	// A literal holds its folded and chomped text in the string node inside it;
	// its own GetValue answers the block as it was written, header and all.
	if l, ok := n.(*ast.LiteralNode); ok {
		if l.Value == nil {
			return nil
		}

		return l.Value.GetValue()
	}
	if s, ok := n.(ast.ScalarNode); ok {
		return s.GetValue()
	}

	return nil
}

// maxFloatBits is the widest integer a float64 can hold the magnitude of.
const maxFloatBits = 1024

// appendJSONScalar writes one YAML scalar as JSON.
//
// A number too wide for a machine word arrives as a *big.Int or a *big.Float
// and is written as the number it is -- up to the point where no JSON reader
// can hold it. Past 1e308 it goes out quoted, because encoding/json refuses to
// read a larger number into any Go type it has, and a string at least keeps the
// digits. Infinity and NaN have no JSON spelling at all and are written as
// null, which is what encoding/json refuses to write.
func appendJSONScalar(out []byte, v any) []byte {
	switch t := v.(type) {
	case nil:
		return append(out, "null"...)
	case string:
		return appendJSONString(out, t)
	case bool:
		return strconv.AppendBool(out, t)
	case int:
		return strconv.AppendInt(out, int64(t), 10)
	case int64:
		return strconv.AppendInt(out, t, 10)
	case uint64:
		return strconv.AppendUint(out, t, 10)
	case float64:
		if math.IsInf(t, 0) || math.IsNaN(t) {
			return append(out, "null"...)
		}

		return strconv.AppendFloat(out, t, 'g', -1, 64)
	case *big.Int:
		if t.BitLen() > maxFloatBits {
			return appendJSONString(out, t.String())
		}

		return append(out, t.String()...)
	case *big.Float:
		if t.IsInf() {
			return append(out, "null"...)
		}
		if f, _ := t.Float64(); math.IsInf(f, 0) {
			return appendJSONString(out, t.Text('g', -1))
		}

		return t.Append(out, 'g', -1)
	case []byte:
		return appendJSONBytes(out, t)
	default:
		text, err := json.Marshal(t)
		if err != nil {
			return append(out, "null"...)
		}

		return append(out, text...)
	}
}

// appendJSONFloat writes a value YAML read as a float.
//
// JSON has one number type, so 1.0 and 1 are the same value -- but a document
// that wrote a float and converts back to YAML should still hold one, and a
// bare "1" reads as an integer. The fractional part is kept for that.
func appendJSONFloat(out []byte, v any) []byte {
	at := len(out)
	out = appendJSONScalar(out, v)
	for _, c := range out[at:] {
		switch c {
		case '.', 'e', 'E', 'n': // n for the null an infinity writes
			return out
		}
	}

	return append(out, ".0"...)
}

// appendJSONBinary writes the bytes a "!!binary" scalar holds.
func appendJSONBinary(out []byte, text string) []byte {
	raw, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return appendJSONString(out, text)
	}

	return appendJSONBytes(out, raw)
}

// appendJSONBytes writes a byte slice the way the value encoder does: a
// sequence of the numbers, not a base64 string.
func appendJSONBytes(out []byte, raw []byte) []byte {
	out = append(out, '[')
	for i, b := range raw {
		if i > 0 {
			out = append(out, ',')
		}
		out = strconv.AppendUint(out, uint64(b), 10)
	}

	return append(out, ']')
}

// appendJSONString writes a JSON string. Anything needing an escape goes
// through encoding/json rather than being escaped here, so the rules are the
// standard library's and not a second set of them.
func appendJSONString(out []byte, s string) []byte {
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
