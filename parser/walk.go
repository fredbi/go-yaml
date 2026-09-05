// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser

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
	// KindAnchor is an anchor, which stands around the one node it names.
	KindAnchor
	// KindTag is a tag, which stands around the one node it types.
	KindTag
)

func (k Kind) String() string {
	switch k {
	case KindMapping:
		return "mapping"
	case KindSequence:
		return "sequence"
	case KindAnchor:
		return "anchor"
	case KindTag:
		return "tag"
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

// Walk reads src through, handing each node to v as the parse reaches it.
//
// ⚠️ The contract is not settled and may change without a deprecation. Three
// questions are open: [ast.Walk] never reaches SequenceEntryNode, FootComment
// or ValueHeadComments; parseFootComment writes into an entry the parse had
// already finished, so the last entry of a block is not handed over until the
// next non-comment token settles it; and where comments belong in the tree is
// still being decided. Use [Parser.Parse] where you need a stable API.
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
// An anchor goes over before the node it names and closes after it, with
// [KindAnchor] as the step's In, so a caller that records what a node writes
// has the anchor open while the node is written. parseAnchorName pins the tape
// at the '&' and parseAnchorValue saves the chunks the node covered, so an
// alias may still read them; releaseDocument gives them back when the document
// ends.
func (p *Parser) Walk(src []byte, v Visitor) (*ast.File, error) {
	p.walk = &walkState{visitor: v}
	defer func() { p.walk = nil }()

	p.begin(src)

	// begin pinned the tape, which is what a full scan wants. A walk keeps
	// nothing it is handed, so the pin goes and the tail moves as the descent
	// reads.
	p.tokens.Unpin()
	defer p.tokens.ReleaseAll()

	file, err := p.parse(p.newContext())
	if err != nil {
		return nil, drawUnder(src, err)
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
	if p.walk.quiet > 0 {
		// The node reads its content back through the descent, so nothing
		// inside it goes over on its own. skip unwinds this in leave.
		p.walk.skip = 1

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
// The tail follows the outermost run and not the token in hand. A descent
// standing deep in a document still holds the tokens of every level it is
// inside -- parseMapEntry reads its key's group again after the value under it
// has been parsed -- and those sit behind where the innermost run stands. The
// outermost run moves only when a whole entry of the document is done, which is
// the last moment any of them is read.
func (p *Parser) readTo(ctx context) {
	if p.tokens == nil || p.body == nil {
		return
	}
	if tk := p.body.at(p.body.idx); tk != nil {
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

// holdRun keeps the chunk holding seq while a construct that began there is
// read, and returns what gives it back.
//
// The descent reads a construct's own tokens again after everything under it:
// parseMapEntry reads its key's group once the value below it is parsed, and a
// sequence reads the '-' its entries are lined up against. The tail follows the
// outermost run and passes those, so the construct says it still wants them.
//
// One chunk per level open at once, so what this holds is the depth of the
// document and not its length.
func (p *Parser) holdRun(seq int32) func() {
	if p.walk == nil || p.tokens == nil {
		return func() {}
	}
	p.tokens.Save(int(seq), int(seq))

	return func() { p.tokens.Release(int(seq), int(seq)) }
}
