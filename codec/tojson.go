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
// The document is written as the parse reaches each node, into one buffer: a
// scalar becomes its JSON text where it stands, a collection writes its
// brackets around what it holds. Nothing is kept but the output and the text of
// the anchors an alias may still name, so the parse hands its tokens and its
// nodes back as it goes and a document of any size is read from a handful of
// them.
//
// A stream of several documents converts its first, which is the one
// [Unmarshal] reads. The rest are still read, so a stream whose later documents
// cannot be converted is refused rather than half-answered.
func ToJSON(src []byte) ([]byte, error) {
	// The JSON runs from half the source to a little under it on the workload
	// corpus -- 0.51x on golang_source, 0.93x on twitter_status -- so the
	// source's length is one allocation that holds all of it. Growing from
	// nothing cost more than the text itself: appendJSONString was a quarter of
	// what the conversion allocated, almost all of it doubling.
	w := &jsonWriter{out: make([]byte, 0, len(src))}

	if _, err := parser.New(parser.OmitNodePaths()).Walk(src, w); err != nil {
		return nil, err
	}
	if w.err != nil {
		return nil, w.err
	}
	if w.documents == 0 {
		return []byte("null"), nil
	}

	return w.out[:w.firstEnd], nil
}

// jsonWriter writes JSON as the walk hands each part of the document over.
//
// Everything it holds is bounded by the document's shape rather than its size:
// the anchors named so far, the collections still open, and the output. A
// mapping records where it began only so that a merge key can be answered at
// the end of it, which is the one thing a converter cannot write as it goes.
type jsonWriter struct {
	out []byte
	// documents counts the document bodies seen, and firstEnd is where the
	// first one ended in out. Later documents are converted and thrown away.
	documents int
	firstEnd  int
	// named holds what each anchor of the document wrote, for an alias to write
	// again. It is emptied at each document, since an alias names an anchor of
	// its own document.
	named map[string][]byte
	// open is the anchors being written, innermost last. Anchors nest.
	open []anchorMark
	// maps is the mappings being written, innermost last.
	maps []mapFrame
	// keys is where each explicit key being written begins in out. A "?" key
	// may hold a collection, which is written as JSON and then held as the
	// string JSON addresses the entry by.
	keys []int
	// tags is the tags being written. A tag opens before its value is parsed,
	// so what it stands on is read when it closes.
	tags []tagMark
	err  error
}

// anchorMark is one anchor still being written: its name, and where in out the
// node it names begins.
type anchorMark struct {
	name string
	at   int
	// key says the anchor stands as a mapping key, so what it wrote becomes
	// the string JSON addresses the entry by -- and that string is what an
	// alias naming it writes later.
	key bool
}

// tagMark is one tag being written: where what it types begins in out, and
// whether the tag stands as a mapping key.
type tagMark struct {
	at  int
	key bool
}

// mapFrame is one mapping being written.
type mapFrame struct {
	// at is where its '{' stands in out, and entries how many of its own it has
	// written -- which is not the step's index, since a merge key writes none.
	at      int
	entries int
	// valueAt is where the value of the entry being written begins, and -1
	// where the entry has none yet.
	valueAt int
	// merged holds what each "<<" of this mapping names, in the order they were
	// written. Empty for the mappings that have none, which is nearly all.
	merged [][]byte
	// mergeValue says the next value handed over belongs to a "<<" and is to be
	// collected rather than written. mergeSeq says that value is a sequence of
	// aliases and its depth, so its own brackets are not written either.
	mergeValue bool
	mergeSeq   int
	// mergedInto says this mapping is the value of a "<<" written out rather
	// than named, so what it writes goes to the mapping around it rather than
	// into the output. It is -1 for every other mapping.
	mergedInto int
}

func (w *jsonWriter) Enter(node ast.Node, at parser.Step) bool {
	if w.err != nil {
		return false
	}
	if at.Depth == 0 && at.In == parser.KindNone {
		if _, isDirective := node.(*ast.DirectiveNode); isDirective {
			// A "%YAML" or "%TAG" line opens a document of its own, ahead of
			// the one it applies to. It holds no value.
			return false
		}
		w.openDocument()
	}

	if frame := w.frame(); frame != nil && frame.mergeValue {
		return w.collectMerge(node, at)
	}

	if _, merge := node.(*ast.MergeKeyNode); merge {
		// "<<" names no key of its own: what it brings in is written at the end
		// of the mapping, where the keys the mapping writes itself are known.
		// Nothing goes over here, not even the comma an entry would take.
		if frame := w.frame(); frame != nil {
			frame.mergeValue = true
			frame.mergeSeq = -1
		}

		return false
	}

	w.separate(at)

	switch n := node.(type) {
	case *ast.MappingKeyNode:
		// "? k" addresses the entry by whatever k writes. What that is comes
		// over inside this node; closing it turns what was written into the
		// string a JSON key has to be.
		w.keys = append(w.keys, len(w.out))

		return true
	case *ast.MappingNode:
		w.openCollectionKey(at)
		w.maps = append(w.maps, mapFrame{at: len(w.out), valueAt: -1, mergedInto: -1})
		w.out = append(w.out, '{')
	case *ast.SequenceNode:
		w.openCollectionKey(at)
		w.out = append(w.out, '[')
	case *ast.AnchorNode:
		w.openAnchor(n, at.Key)
	case *ast.AliasNode:
		w.writeAlias(n, at)
	case *ast.TagNode:
		// A tag opens before the node it types is parsed, so TagNode.Value is
		// nil here and what the tag stands on is written when it closes.
		w.tags = append(w.tags, tagMark{at: len(w.out), key: at.Key})
	default:
		w.scalar(node, at)
	}

	return true
}

func (w *jsonWriter) Leave(node ast.Node, at parser.Step) {
	if w.err != nil {
		return
	}

	switch n := node.(type) {
	case *ast.MappingKeyNode:
		w.closeKey(parser.Step{Key: true})
	case *ast.TagNode:
		w.closeTag(n)
	case *ast.MappingNode:
		w.closeMapping()
		w.closeKey(at)
	case *ast.SequenceNode:
		if frame := w.frame(); frame != nil && frame.mergeSeq == at.Depth {
			frame.mergeSeq, frame.mergeValue = -1, false

			return
		}
		w.out = append(w.out, ']')
		w.closeKey(at)
	case *ast.AnchorNode:
		w.closeAnchor(n)
	}

	if at.Depth == 0 && at.In == parser.KindNone && w.documents == 1 {
		w.firstEnd = len(w.out)
	}
}

// openCollectionKey records where a collection standing as a mapping key
// begins, so that closeKey can hold it to the string JSON keys are.
func (w *jsonWriter) openCollectionKey(at parser.Step) {
	if at.Key {
		w.keys = append(w.keys, len(w.out))
	}
}

// closeKey turns what a key wrote into the string it addresses the entry by.
//
// A key may be written as any JSON at all -- "? [a, b]" and "[a, b]: v" both
// address the entry by a sequence -- and JSON holds a key as a string, so what
// was written is read back as one.
func (w *jsonWriter) closeKey(at parser.Step) {
	if !at.Key {
		return
	}
	from := w.keys[len(w.keys)-1]
	w.keys = w.keys[:len(w.keys)-1]

	text := unquoted(w.out[from:])
	w.out = appendJSONString(w.out[:from], text)
}

// openDocument starts a document of the stream.
//
// An anchor belongs to the document it was written in, so what the last one
// named is forgotten here. The first document's text is what ToJSON returns;
// the rest are written after it and cut off, so that an alias naming nothing is
// still refused wherever it stands.
func (w *jsonWriter) openDocument() {
	w.documents++
	clear(w.named)
	w.maps = w.maps[:0]
	w.open = w.open[:0]
}

// frame is the mapping being written, or nil outside one.
func (w *jsonWriter) frame() *mapFrame {
	if len(w.maps) == 0 {
		return nil
	}

	return &w.maps[len(w.maps)-1]
}

// separate writes what goes between two entries of a collection, and keeps the
// count the next one needs.
//
// A mapping hands a key over and then its value, so the step says which this
// is: a comma before a key that is not the first written, and a colon before a
// value. The count is the mapping's own rather than the step's index, since a
// "<<" takes an index and writes nothing.
//
// It runs before anything is written and before any frame is pushed, so the
// mapping it reads is the one the node stands in rather than one the node is
// about to open.
func (w *jsonWriter) separate(at parser.Step) {
	switch at.In {
	case parser.KindMapping:
		frame := w.frame()
		if frame == nil {
			return
		}
		if at.Key {
			if frame.entries > 0 {
				w.out = append(w.out, ',')
			}
			frame.entries++
			frame.valueAt = -1

			return
		}
		if frame.valueAt >= 0 {
			// A second value for one key. The parse can hand two over -- "&!"
			// reads as a tag on nothing and an anchor, both entries of the
			// mapping -- and the last of them is the one the document is read
			// into Go values as.
			w.out = w.out[:frame.valueAt]

			return
		}
		w.out = append(w.out, ':')
		frame.valueAt = len(w.out)
	case parser.KindSequence:
		if at.Index > 0 {
			w.out = append(w.out, ',')
		}
	}
}

// scalar writes one value, as a string where it stands as a mapping's key.
func (w *jsonWriter) scalar(node ast.Node, at parser.Step) {
	if at.Key {
		w.out = appendJSONString(w.out, keyText(node))

		return
	}

	w.out = appendJSONScalar(w.out, jsonScalarOf(node))
}

// keyText is a scalar key as the string a mapping holds it under. JSON keys are
// strings, so 4.0 and 4 address the same entry and are both "4".
func keyText(node ast.Node) string {
	switch t := jsonScalarOf(node).(type) {
	case nil:
		// A key left empty addresses the entry by the word JSON writes for it,
		// which is what the value converter held it under.
		return "null"
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

// closeTag writes what a tag stands on, once the walk has read it.
//
// A tag on a collection is written by the collection itself, which went over
// between this tag's Enter and Leave; there is nothing left to do but hold the
// text to a string where the tag stands as a mapping key. A tag on a scalar
// writes nothing of its own -- parseScalarValue builds the value without
// handing it over -- so the scalar is written here, from the node.
//
// The eight tags that name a scalar type are read off the text the scalar was
// written with, so "!!str 1" is the string "1" and "!!int \"3\"" the number 3.
// Every other tag -- !!seq, !!map, !!set, !!omap, !!timestamp, !!merge and any
// the document defines itself -- writes the value as it stands.
func (w *jsonWriter) closeTag(t *ast.TagNode) {
	mark := w.tags[len(w.tags)-1]
	w.tags = w.tags[:len(w.tags)-1]

	switch resolved, ok := w.taggedValue(t); {
	case ok:
		// The tag names a scalar type, so it says what the value is whatever
		// the value wrote for itself: "!!str" on an empty node is "", not null.
		w.out = append(w.out[:mark.at], resolved...)
	case len(w.out) == mark.at:
		// A tag on a scalar writes nothing of its own -- parseScalarValue
		// builds the value without handing it over -- so it is written here.
		w.out = appendJSONScalar(w.out, jsonScalarOf(t.Value))
	}

	if mark.key {
		w.out = appendJSONString(w.out[:mark.at], unquoted(w.out[mark.at:]))
	}
}

// taggedValue is the JSON a tagged scalar is written as, and whether the tag
// names a scalar type at all.
//
// A "%TAG" line gives the handle a prefix of the document's own, so "!!int"
// under one names the document's type and not YAML's, and the value stands as
// it is written.
func (w *jsonWriter) taggedValue(t *ast.TagNode) ([]byte, bool) {
	text, isScalar := taggedText(t.Value)
	if t.Start == nil || t.Directive != nil || !isScalar {
		return nil, false
	}

	var written []byte
	switch token.ReservedTagKeyword(t.Start.Value) {
	case token.StringTag:
		written = appendJSONString(nil, text)
	case token.IntegerTag:
		written = strconv.AppendInt(nil, taggedInteger(text), 10)
	case token.FloatTag:
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			f = 0
		}
		written = appendJSONFloat(nil, f)
	case token.BooleanTag:
		b, ok := token.ParseBool(text)
		if !ok {
			// The tag says boolean whatever the text is, so a spelling neither
			// schema resolves is tried once more in lower case: "!!bool Yes"
			// and "!!bool YES" are the same request.
			b, ok = token.ParseBool(strings.ToLower(text))
		}
		if !ok {
			w.fail(yamlerrors.NewSyntax(fmt.Sprintf("cannot convert %q to boolean", text), t.Value.GetToken()))

			return []byte("false"), true
		}
		written = strconv.AppendBool(nil, b)
	case token.NullTag:
		written = []byte("null")
	case token.BinaryTag:
		written = appendJSONBinary(nil, text)
	default:
		return nil, false
	}

	return written, true
}

// openAnchor records the name and where the node it names will start.
func (w *jsonWriter) openAnchor(node *ast.AnchorNode, key bool) {
	// An anchor whose name the scan could not read -- "&\"\"" and the like --
	// names nothing an alias can reach, so nothing is remembered under it. The
	// node it stands on is written as it would be without the anchor.
	w.open = append(w.open, anchorMark{name: anchorName(node.Name), at: len(w.out), key: key})
}

// closeAnchor keeps what the anchored node wrote.
//
// An anchor standing as a mapping key holds whatever JSON its node wrote, and a
// JSON key is a string: "&n x" as a key addresses the entry by "x", and that is
// what "*n" writes later.
func (w *jsonWriter) closeAnchor(node *ast.AnchorNode) {
	if len(w.open) == 0 {
		return
	}
	mark := w.open[len(w.open)-1]
	w.open = w.open[:len(w.open)-1]

	if len(w.out) == mark.at {
		// A key written as "{&a 21}" is read without its parts going over on
		// their own, so the anchor stands around a node the walk never handed
		// over. It is written here, from the node.
		w.out = appendJSONScalar(w.out, jsonScalarOf(node.Value))
	}

	// What the anchor stands for is the value its node wrote, not the string a
	// key is held to: "&a FALSE" as a key addresses the entry by "false" and
	// "*a" as a value elsewhere is still the boolean.
	w.remember(mark.name, w.out[mark.at:])

	if mark.key {
		w.out = appendJSONString(w.out[:mark.at], unquoted(w.out[mark.at:]))
	}
}

// remember keeps a copy of what an anchor wrote. The text is copied because out
// grows as the rest of the document is written and a slice of it would not
// survive the next append.
func (w *jsonWriter) remember(name string, text []byte) {
	if name == "" {
		return
	}
	if w.named == nil {
		w.named = make(map[string][]byte)
	}
	w.named[name] = append([]byte(nil), text...)
}

// writeAlias writes again what the anchor of the same name wrote.
//
// An anchor is readable from the moment it closes, so an alias naming one that
// has not closed -- a forward reference, or a node aliasing itself -- names
// nothing and is refused, as it is when the document is read into Go values.
func (w *jsonWriter) writeAlias(node *ast.AliasNode, at parser.Step) {
	name := anchorName(node.Value)
	text, ok := w.named[name]
	if !ok {
		w.fail(yamlerrors.NewSyntax(fmt.Sprintf("could not find alias %q", name), node.GetToken()))

		return
	}
	if at.Key {
		// A JSON key is a string whatever the anchored node was.
		w.out = appendJSONString(w.out, unquoted(text))

		return
	}
	w.out = append(w.out, text...)
}

// unquoted is the text of a JSON string, or the JSON itself where it is not
// one. "null" is the word, not the empty string a JSON null reads as.
func unquoted(text []byte) string {
	if len(text) == 0 || text[0] != '"' {
		return string(text)
	}
	var s string
	if err := json.Unmarshal(text, &s); err != nil {
		return string(text)
	}

	return s
}

// anchorName reads the name off an anchor or an alias.
func anchorName(n ast.Node) string {
	if n == nil || n.GetToken() == nil {
		return ""
	}

	return n.GetToken().Value
}

func (w *jsonWriter) fail(err error) {
	if w.err == nil {
		w.err = err
	}
}

// collectMerge takes what a "<<" names instead of writing it.
//
// The value is an alias naming a mapping, or a sequence of them -- earlier
// first, since an earlier merge wins. Nothing is written here: the entries go in
// at the end of the mapping, where the keys it writes itself are known.
func (w *jsonWriter) collectMerge(node ast.Node, at parser.Step) bool {
	frame := w.frame()

	switch n := node.(type) {
	case *ast.SequenceNode:
		frame.mergeSeq = at.Depth

		return true
	case *ast.AliasNode:
		name := anchorName(n.Value)
		text, ok := w.named[name]
		if !ok {
			w.fail(yamlerrors.NewSyntax(fmt.Sprintf("could not find alias %q", name), n.GetToken()))

			return false
		}
		if len(text) == 0 || text[0] != '{' {
			// "<<" takes a mapping, or a sequence of them. An anchor naming a
			// scalar or a sequence brings in no entries and is refused, as it
			// is when the document is read into Go values.
			w.fail(yamlerrors.NewUnexpectedNodeType(n.Type(), ast.MappingType, n.GetToken()))

			return false
		}
		frame.merged = append(frame.merged, text)
		if frame.mergeSeq < 0 {
			frame.mergeValue = false
		}

		return false
	case *ast.MappingNode:
		// "<<: {a: 1}" merges a mapping written out. It is not read here --
		// nothing has written it yet -- so the walk goes into it and what it
		// writes is taken at its own Leave.
		frame.mergeValue = false
		w.separate(at)
		w.maps = append(w.maps, mapFrame{at: len(w.out), valueAt: -1, mergedInto: frame.at})
		w.out = append(w.out, '{')

		return true
	default:
		w.fail(yamlerrors.NewUnexpectedNodeType(node.Type(), ast.MappingType, node.GetToken()))

		return false
	}
}

// closeMapping writes what the mapping's "<<" entries bring in, and closes it.
//
// The merged keys go in at the end, and the mapping's own win: a key written
// here overrides the one merged in, and the first merge to name a key beats a
// later one. JSON has no way to write a key twice, so what the mapping already
// holds is read back out of what was written before anything is added.
func (w *jsonWriter) closeMapping() {
	frame := w.maps[len(w.maps)-1]
	w.maps = w.maps[:len(w.maps)-1]

	if frame.mergedInto >= 0 {
		// "<<: {a: 1}" merges a mapping written out. It had to be walked to be
		// read, so it was written where it stood; it comes back out of the
		// output and goes in with the rest of what the "<<" brings.
		w.out = append(w.out, '}')
		text := append([]byte(nil), w.out[frame.at:]...)
		w.out = w.out[:frame.at]
		if into := w.frame(); into != nil {
			into.merged = append(into.merged, text)
		}

		return
	}

	if len(frame.merged) == 0 {
		w.out = append(w.out, '}')

		return
	}

	held := make(map[string]struct{}, frame.entries)
	for _, pair := range jsonPairs(w.out[frame.at:]) {
		held[pair.key] = struct{}{}
	}
	for _, text := range frame.merged {
		for _, pair := range jsonPairs(text) {
			if _, ok := held[pair.key]; ok {
				continue
			}
			held[pair.key] = struct{}{}
			if frame.entries > 0 {
				w.out = append(w.out, ',')
			}
			w.out = append(w.out, text[pair.from:pair.to]...)
			frame.entries++
		}
	}
	w.out = append(w.out, '}')
}

// jsonPair is one entry of a JSON object: its key, and where the whole
// "key":value stands in the object's text.
type jsonPair struct {
	key      string
	from, to int
}

// jsonPairs reads the entries of a JSON object back out of the text.
//
// The text is what this converter wrote, so the shapes are the ones it writes
// and nothing else: a value is a string, a number, a literal, an object or an
// array, and a string escapes with a backslash. Anything that does not open
// with '{' has no entries.
func jsonPairs(text []byte) []jsonPair {
	if len(text) == 0 || text[0] != '{' {
		return nil
	}

	var (
		pairs []jsonPair
		i     = 1
	)
	for i < len(text) && text[i] != '}' {
		if text[i] == ',' {
			i++

			continue
		}
		start := i
		key, next, ok := jsonString(text, i)
		if !ok {
			return pairs
		}
		i = next
		if i >= len(text) || text[i] != ':' {
			return pairs
		}
		i = jsonValueEnd(text, i+1)
		pairs = append(pairs, jsonPair{key: key, from: start, to: i})
	}

	return pairs
}

// jsonString reads the string at i and returns it, and where it ends.
func jsonString(text []byte, i int) (string, int, bool) {
	if i >= len(text) || text[i] != '"' {
		return "", i, false
	}
	end := jsonValueEnd(text, i)
	var s string
	if err := json.Unmarshal(text[i:end], &s); err != nil {
		return "", i, false
	}

	return s, end, true
}

// jsonValueEnd returns where the value starting at i ends.
func jsonValueEnd(text []byte, i int) int {
	depth := 0
	for i < len(text) {
		switch text[i] {
		case '"':
			for i++; i < len(text); i++ {
				if text[i] == '\\' {
					i++

					continue
				}
				if text[i] == '"' {
					break
				}
			}
			i++
			if depth == 0 {
				return i
			}

			continue
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth <= 0 {
				return i + 1
			}
		case ',':
			if depth == 0 {
				return i
			}
		}
		i++
		if depth == 0 && i < len(text) && (text[i] == ',' || text[i] == '}' || text[i] == ']') {
			return i
		}
	}

	return i
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
