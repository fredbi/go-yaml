// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec

import (
	"fmt"

	"github.com/go-openapi/go-yaml/ast"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
	"github.com/go-openapi/go-yaml/parser"
	"github.com/go-openapi/go-yaml/token"
)

// jsonTokener hands a document over as JSON tokens as the walk reaches each
// part of it.
//
// What it holds is bounded by the document's shape rather than its length: the
// collections still open, the keys of each of those, and what a "<<" brings in.
// The keys are held because a mapping's own key beats one a merge brings in and
// the merge is answered when the mapping closes, which is the one thing a
// converter cannot hand over as it goes.
type jsonTokener struct {
	state *JSONTokens
	yield func(JSONToken) bool

	// stopped says the range body asked to stop, or the conversion failed.
	stopped bool
	// count is how many tokens have gone over, against JSONTokens.budget, and
	// handed how many of those reached the range body rather than a collected
	// run.
	count  int
	handed int

	// firstDoc is which document of the stream to convert. A "%YAML" or "%TAG"
	// line is a document of its own in the File's Docs, ahead of the one it
	// applies to, so the document to convert is the one past the directives.
	// ended says the walk stands outside it.
	firstDoc int
	ended    bool

	// maps are the mappings open, innermost last.
	maps []tokenMapFrame
	// tags are the tags open, innermost last.
	tags []tokenTagMark
	// buffer is where tokens go while a "<<" value is read, and nil otherwise.
	buffer *[]JSONToken
	// expanding names the anchors being written out again, so an alias that
	// reaches back into its own anchor is refused rather than followed.
	expanding []string
	// bufDepth is the depth of the mapping a "<<" wrote out, whose tokens are
	// being collected, and bufOwner which mapping they will be merged into.
	bufDepth int
	bufOwner int
	// keys are the wrappers a mapping key opened with, innermost last. A "?",
	// an anchor and a tag are all handed over before what they stand on is
	// parsed, so a key is named when its wrapper closes.
	keys []tokenKeyMark
	// peek indexes the tag in tags whose first token decides whether it holds a
	// scalar or a collection, and is -1 where there is none.
	peek int
	// suppress counts the wrappers whose content does not go over on its own:
	// a key, which is one token whatever it holds, and a tag naming a scalar
	// type, which says what its node is worth whatever the node wrote.
	suppress int
}

// tokenMapFrame is one mapping being handed over.
type tokenMapFrame struct {
	// keys are the keys the mapping has written itself, for the merge to be
	// answered against when it closes.
	keys []string
	// merged holds what each "<<" of this mapping brings in, earliest first.
	merged [][]JSONToken
	// mergeValue says the next node handed over belongs to a "<<" and is to be
	// collected rather than handed on. mergeSeq holds the depth of a sequence
	// of merge sources, and -1 where there is none.
	mergeValue bool
	mergeSeq   int
}

// tokenKeyMark is one mapping key open: the wrapper the walk handed over, and
// the depth it stands at.
//
// ⛔ The depth and not the pointer is what closes it. The parse hands its cells
// out again behind the descent, so a node built inside this key may be the same
// pointer, and matching on that would close the key early. A wrapper's Leave
// reports the depth its Enter did, and key wrappers nest strictly, so the depth
// names exactly one of them.
type tokenKeyMark struct {
	node  ast.Node
	depth int
}

// tokenTagMark is one tag open: how many tokens had gone over when it opened,
// and whether it stands as a mapping key.
type tokenTagMark struct {
	at int
	// key says the tag stands as a mapping key, so what it resolves to names
	// the entry rather than filling it.
	key bool
	// suppressed says openTag held back what the tag stands on.
	suppressed bool
	// peeking says the tag names a kind rather than a scalar type, so what it
	// stands on is held back only until the first token says which it is: a
	// collection writes itself and the hold is dropped, a scalar is kept in
	// held and replaced where the tag resolves.
	peeking bool
	held    JSONToken
	heldSet bool
}

func (t *jsonTokener) Enter(node ast.Node, at parser.Step) bool {
	if t.stopped {
		return false
	}

	if _, isDirective := node.(*ast.DirectiveNode); isDirective && at.Depth == 0 && at.In == parser.KindNone {
		// A "%YAML" or "%TAG" line opens a document of its own, ahead of the
		// one it applies to. It holds no value, so the document to convert is
		// the next one along -- guarded on nothing having gone over yet, since
		// a directive arriving after that is not opening it.
		if at.Document == t.firstDoc && t.handed == 0 {
			t.firstDoc = at.Document + 1
		}

		return false
	}

	// A later document of the stream is still read, so that a stream this
	// converter cannot read is refused rather than half-answered, and none of
	// it goes over.
	t.ended = at.Document != t.firstDoc
	if t.ended {
		return true
	}

	if frame := t.frame(); frame != nil && frame.mergeValue {
		return t.collectMerge(node, at)
	}

	if isMergeKey(node) {
		// "<<" names no key of its own: what it brings in goes over at the end
		// of the mapping, where the keys the mapping writes itself are known.
		if frame := t.frame(); frame != nil {
			frame.mergeValue = true
			frame.mergeSeq = -1
		}

		return false
	}

	if at.Key {
		return t.enterKey(node, at)
	}

	switch n := node.(type) {
	case *ast.MappingNode:
		t.open(JSONObjectStart, at.At)
		t.maps = append(t.maps, tokenMapFrame{mergeSeq: -1})
	case *ast.SequenceNode:
		t.open(JSONArrayStart, at.At)
	case *ast.AnchorNode:
		// An anchor stands around the node it names, which goes over on its
		// own. Nothing is recorded: an alias reads ast.AliasNode.Target.
	case *ast.AliasNode:
		t.emitAlias(n, at)

		return false
	case *ast.TagNode:
		t.openTag(n, false)
	default:
		t.emitScalarNode(node, at.At)
	}

	return !t.stopped
}

func (t *jsonTokener) Leave(node ast.Node, at parser.Step) {
	if t.stopped || at.Document != t.firstDoc {
		return
	}

	if t.closeKey(node, at) {
		return
	}

	switch n := node.(type) {
	case *ast.MappingNode:
		if err := refuseDuplicateKeys(n); err != nil {
			t.fail(err)

			return
		}
		t.closeMapping(at)
	case *ast.SequenceNode:
		if frame := t.frame(); frame != nil && frame.mergeSeq == at.Depth {
			frame.mergeSeq, frame.mergeValue = -1, false

			return
		}
		t.close(JSONArrayEnd, at.At)
	case *ast.TagNode:
		t.closeTag(n, at)
	}

}

// frame is the mapping being handed over, and nil outside one.
func (t *jsonTokener) frame() *tokenMapFrame {
	if len(t.maps) == 0 {
		return nil
	}

	return &t.maps[len(t.maps)-1]
}

// emit hands one token over, and records where the conversion now stands.
func (t *jsonTokener) emit(tok JSONToken) {
	if t.stopped {
		return
	}
	if t.peek >= 0 {
		switch tok.Kind {
		case JSONObjectStart, JSONArrayStart:
			// The tags naming a kind stand on a collection, which writes
			// itself. Nothing is held back from here on -- and a chain of them,
			// "! !set" over a mapping, all stand on the same collection.
			t.releasePeeked()
		default:
			if !t.tags[t.peek].heldSet {
				t.tags[t.peek].held, t.tags[t.peek].heldSet = tok, true
			}

			return
		}
	}
	if t.suppress > 0 {
		return
	}
	t.count++
	if t.state.budget > 0 && t.count > t.state.budget {
		t.fail(yamlerrors.NewNotJSON(
			fmt.Sprintf("the document hands over more than %d JSON tokens", t.state.budget), nil))

		return
	}

	if t.buffer != nil {
		*t.buffer = append(*t.buffer, tok)

		return
	}

	t.handed++
	t.step(tok)
	if !t.yield(tok) {
		t.stopped = true
	}
}

// releasePeeked stops the tags naming a kind holding back what they stand on,
// innermost first, once a collection has said that is what they stand on.
func (t *jsonTokener) releasePeeked() {
	for i := len(t.tags) - 1; i >= 0; i-- {
		if !t.tags[i].peeking || !t.tags[i].suppressed {
			break
		}
		t.tags[i].suppressed, t.tags[i].peeking = false, false
		t.suppress--
	}
	t.peek = -1
}

// step moves the path and the depth onto the token about to go over.
//
// A closing token reports the depth it returns to, which is what the JSON lexer
// this feeds does: it pops the collection before handing the closer over.
func (t *jsonTokener) step(tok JSONToken) {
	s := t.state

	switch tok.Kind {
	case JSONObjectEnd, JSONArrayEnd:
		s.depth--
		if len(s.frames) > 0 {
			s.frames = s.frames[:len(s.frames)-1]
		}

		return
	case JSONKey:
		if n := len(s.frames); n > 0 {
			s.frames[n-1].key, s.frames[n-1].named = tok.Value, true
		}

		return
	}

	// Everything else fills a slot of the collection around it, a nested
	// collection as much as a scalar, so a sequence counts one more element.
	if n := len(s.frames); n > 0 && s.frames[n-1].array {
		if s.frames[n-1].named {
			s.frames[n-1].index++
		}
		s.frames[n-1].named = true
	}

	if tok.Kind == JSONObjectStart || tok.Kind == JSONArrayStart {
		s.depth++
		s.frames = append(s.frames, jsonPathFrame{array: tok.Kind == JSONArrayStart})
	}
}

// open hands a collection's opening token over.
func (t *jsonTokener) open(kind JSONTokenKind, at token.Position) {
	t.emit(JSONToken{Kind: kind, At: at})
}

// close hands a collection's closing token over.
func (t *jsonTokener) close(kind JSONTokenKind, at token.Position) {
	t.emit(JSONToken{Kind: kind, At: at})
}

// fail records the first thing the conversion refused and stops it.
func (t *jsonTokener) fail(err error) {
	if t.state.err == nil {
		t.state.err = err
	}
	t.stopped = true
}
