// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package labparser

import (
	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/token"
)

// Kind says what sort of collection the walk is in, or has just entered.
type Kind uint8

const (
	// KindNone is the document's body, which stands in no collection.
	KindNone Kind = iota
	// KindMapping is a mapping, whose entries are keys with values.
	KindMapping
	// KindSequence is a sequence, whose entries are values.
	KindSequence
)

func (k Kind) String() string {
	switch k {
	case KindMapping:
		return "mapping"
	case KindSequence:
		return "sequence"
	default:
		return "none"
	}
}

// Step is where the walk stands when it hands something over.
//
// It is the whole of the context a caller needs to write a document out again:
// what it is inside, how deep, which entry of that, and where it stands in the
// source. A converter needs nothing else, and so has nothing to hold on to and
// nothing to ask the arena to keep.
type Step struct {
	// In is the collection around what is being handed over.
	In Kind
	// Depth counts the collections enclosing it. The document's body is 0.
	Depth int
	// Index is which entry of In this is, counted from 0. It is what tells a
	// writer whether a separator goes before this one.
	Index int
	// Key says this node is a mapping's key, and that its value comes next. A
	// writer needs it: a key and its value are two handovers of one entry, and
	// nothing about the node itself says which it is.
	Key bool
	// At is where the first token of what is handed over stands.
	At token.Position
}

// Visitor is handed each part of a document as the parse reaches it.
//
// Enter comes before anything the node holds and Leave after all of it, so a
// writer opens a collection on Enter and closes it on Leave. A scalar takes
// both, one after the other.
//
// What is handed over is only good until Leave returns: the parse may reclaim
// the tokens under it as soon as the walk moves on. Read what is needed while
// it is there.
type Visitor interface {
	// Enter is called on a node before anything it holds. Returning false
	// leaves what it holds unvisited, and Leave is not called for it.
	Enter(node ast.Node, at Step) bool
	// Leave is called once everything the node holds has been visited.
	Leave(node ast.Node, at Step)
}

// walkState is where a walk stands. It is nil where the parse was not asked to
// walk, and every hook below returns at once on that.
type walkState struct {
	visitor Visitor
	// in and index hold the collection at each depth, so a node knows what it
	// stands in and which entry of it it is.
	in    []Kind
	index []int
	// skip counts the depths below a node Enter refused, which are walked
	// without being handed over.
	skip int
	// quiet counts the parses standing inside a node already handed over. A
	// block scalar reads its content through parseToken, and the content is
	// part of the literal rather than a value of its own.
	quiet int
	// err is the first thing the walk refused.
	err error
}

// Walk reads the stream through, handing each node to v as the parse reaches it.
//
// It does not gather: a collection's entries are handed over one at a time and
// the collection keeps none of them, so what stands at once is the walk's own
// depth rather than the document.
//
// The tape is let go of as the descent reads past it, so what a node holds is
// good until Leave returns and no longer. The [ast.File] it returns holds the
// documents and not their bodies, which went to v: a body kept here would read
// whatever was written over it.
//
// ⚠️ Anchors are not handled. An alias names a subtree that has to outlive the
// tail, which wants Save, and nothing calls it yet.
func (p *Parser) Walk(v Visitor) (*ast.File, error) {
	p.walk = &walkState{visitor: v}
	defer func() { p.walk = nil }()

	// New pinned the tape, which is what a full scan wants. A walk keeps
	// nothing it is handed, so the pin goes and the tail moves as the descent
	// reads.
	p.tokens.Unpin()
	defer p.tokens.ReleaseAll()

	file, err := p.parse(p.newContext())
	if err != nil {
		return nil, err
	}
	if p.walk.err != nil {
		return nil, p.walk.err
	}

	return file, nil
}

// walking reports whether this parse is handing nodes over as it goes.
func (p *Parser) walking() bool { return p.walk != nil }

// openAnchor holds the tape and records where the anchor begins.
func (p *Parser) openAnchor(ctx context) {
	if p.walk == nil || p.tokens == nil {
		return
	}

	var from int32
	if tk := ctx.currentToken(); tk != nil {
		from = tk.Seq()
	}
	p.anchorFrom = append(p.anchorFrom, from)
	p.tokens.Pin()
}

// closeAnchor keeps the chunks the anchor covers and gives the hold back.
//
// The descent stands at or just past the last token of the anchored node, so
// saving to there takes one token more than the run holds at worst, which costs
// the chunk it falls in and never loses one that is wanted.
func (p *Parser) closeAnchor(ctx context) {
	if p.walk == nil || p.tokens == nil || len(p.anchorFrom) == 0 {
		return
	}

	from := p.anchorFrom[len(p.anchorFrom)-1]
	p.anchorFrom = p.anchorFrom[:len(p.anchorFrom)-1]

	to := p.tokens.Len()
	if tk := ctx.currentToken(); tk != nil {
		to = int(tk.Seq())
	}
	p.tokens.Save(int(from), to)
	p.tokens.Unpin()
}

// releaseDocument gives back what the anchors of a document saved.
//
// An alias names its anchor within one document, so what the anchors covered is
// finished with when the document is.
func (p *Parser) releaseDocument() {
	if p.walk == nil || p.tokens == nil {
		return
	}
	p.tokens.ReleaseAll()
}

// enter hands a node over before what it holds, and reports whether to go on
// into it.
func (p *Parser) enter(ctx context, node ast.Node, in Kind) bool {
	if p.walk == nil || node == nil {
		return true
	}
	if p.walk.skip > 0 {
		p.walk.skip++

		return true
	}

	if !p.walk.visitor.Enter(node, p.step(node)) {
		p.walk.skip = 1

		return false
	}

	p.walk.in = append(p.walk.in, in)
	p.walk.index = append(p.walk.index, 0)

	return true
}

// leave hands a node over once everything it holds has been.
func (p *Parser) leave(ctx context, node ast.Node) {
	if p.walk == nil || node == nil {
		return
	}
	if p.walk.skip > 0 {
		p.walk.skip--

		return
	}

	p.walk.in = p.walk.in[:len(p.walk.in)-1]
	p.walk.index = p.walk.index[:len(p.walk.index)-1]
	p.walk.visitor.Leave(node, p.step(node))
	p.count()
	p.readTo(ctx)
}

// handKey gives a mapping's key over, before its value is parsed.
func (p *Parser) handKey(ctx context, node ast.Node) {
	p.handAs(ctx, node, true)
}

// hand gives a node that holds nothing to the visitor, Enter then Leave.
func (p *Parser) hand(ctx context, node ast.Node) {
	p.handAs(ctx, node, false)
}

func (p *Parser) handAs(ctx context, node ast.Node, key bool) {
	if p.walk == nil || node == nil || p.walk.skip > 0 || p.walk.quiet > 0 {
		return
	}

	at := p.step(node)
	at.Key = key
	if p.walk.visitor.Enter(node, at) {
		p.walk.visitor.Leave(node, at)
	}
	p.count()
	p.readTo(ctx)
}

// count records that one more entry of the collection in hand has been walked.
func (p *Parser) count() {
	if n := len(p.walk.index); n > 0 {
		p.walk.index[n-1]++
	}
}

// step is where the walk stands, for a node about to be handed over.
func (p *Parser) step(node ast.Node) Step {
	at := Step{Depth: len(p.walk.in)}
	if n := len(p.walk.in); n > 0 {
		at.In, at.Index = p.walk.in[n-1], p.walk.index[n-1]
	}
	if tk := node.GetToken(); tk != nil {
		at.At = tk.Position
	}

	return at
}

// quiet stops what a node holds from being handed over on its own, for a node
// that reads its content back through the descent. It returns what undoes it.
func (p *Parser) quiet() func() {
	if p.walk == nil {
		return func() {}
	}
	p.walk.quiet++

	return func() { p.walk.quiet-- }
}

// readTo tells the arena how far the descent has read, so that it may fill
// again what stands behind that.
//
// Everything below the token the descent stands on has been read and handed
// over, and a walk keeps none of what it was handed. What the parse still needs
// from behind it is the column of each open level, which the context carries as
// a number rather than as a token.
func (p *Parser) readTo(ctx context) {
	if p.tokens == nil {
		return
	}
	if tk := ctx.currentToken(); tk != nil {
		p.tokens.SetTail(int(tk.Seq()))
	}
}

// saveHere keeps the chunks holding [from, to] without holding the tape first,
// for a run whose extent is already known.
func (p *Parser) saveHere(from, to int32) {
	if p.walk == nil || p.tokens == nil {
		return
	}
	p.tokens.Save(int(from), int(to))
}
