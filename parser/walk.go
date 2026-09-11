// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/token"
)

// Kind names the collection around a node handed to a [Visitor].
type Kind uint8

const (
	// KindNone marks a document's body, which is in no collection.
	KindNone Kind = iota
	// KindMapping is a mapping. Its entries are keys, each followed by its value.
	KindMapping
	// KindSequence is a sequence. Its entries are values.
	KindSequence
	// KindAnchor is an anchor, which encloses the node it names.
	KindAnchor
	// KindTag is a tag, which encloses the node it types.
	KindTag
	// KindKey is a mapping key written with "?", which encloses the key node.
	KindKey
)

// String returns the kind's name in lower case: "none" for [KindNone] and for any undefined value.
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
	case KindKey:
		return "key"
	default:
		return "none"
	}
}

// Step locates a node handed to a [Visitor].
//
// It carries what a writer needs to emit the document again:
// the enclosing collection, the depth, the entry index and the source position.
type Step struct {
	// In is the collection around the node.
	In Kind
	// Depth counts the collections enclosing the node. A document's body is at depth 0.
	Depth int
	// Index is the node's entry number in In, counted from 0.
	// A writer reads it to decide whether a separator goes before the node.
	Index int
	// Key reports that the node is a mapping key and that its value comes next.
	// A key and its value are two handovers of one entry, and nothing on the node tells them apart.
	Key bool
	// At is the position of the node's first token.
	At token.Position
	// Document indexes the node's document in the Docs of the [ast.File] that [Parser.Walk] returns.
	// An anchor and a "%YAML" directive each apply to one document, so use it to tell documents apart.
	//
	// An empty document hands over no node, and the count still moves past it:
	// "---" over "---" over "b: 2" hands over one mapping, at Document 1.
	//
	// Each "%YAML" or "%TAG" line counts as a document of its own, ahead of the one it applies to.
	// "%YAML 1.2" over "---" over "a: 1" puts the mapping at Document 1, and two directive lines put it at 2.
	// To find the nth document a reader sees, skip the documents whose node is an [ast.DirectiveNode],
	// as [github.com/go-openapi/go-yaml/codec.ToJSON] does.
	Document int
}

// Visitor receives each node of a document as [Parser.Walk] reaches it.
//
// Enter comes before the node's content and Leave after it,
// so a writer opens a collection on Enter and closes it on Leave.
// A scalar gets Enter, then Leave at once.
//
// A node is valid until its Leave returns.
// The parse reuses the tokens and the node cells behind the walk, so copy what is needed before then.
//
// Do not key a map on the node pointer.
// Because the parse reuses node cells, two different nodes of one document often share a pointer.
// Key on the token's offset, or on the node's value.
type Visitor interface {
	// Enter is called before the node's content.
	// Returning false skips the content, and Leave is not called for the node.
	Enter(node ast.Node, at Step) bool
	// Leave is called after the node's content has been visited.
	Leave(node ast.Node, at Step)
}

// walkState is where a walk stands. It is nil where the parse was not asked to
// walk, and every hook below returns at once on that.
type walkState struct {
	visitor Visitor
	// in and index hold the collection at each depth, so a node knows what it
	// stands in and which entry of it it is. key says whether the node opened
	// at that depth was a mapping key, so Leave says what Enter did.
	in    []Kind
	index []int
	key   []bool
	// keyNext says the next node handed over is a mapping's key. A key may turn
	// out to be a scalar, an anchor, a tag or a "?" standing around one of
	// those, and each of them goes over through a different path; the flag
	// reaches whichever it is, so the key is announced once and as a key.
	keyNext bool
	// skip counts the depths below a node Enter refused, which are walked
	// without being handed over.
	skip int
	// quiet counts the parses standing inside a node already handed over. A
	// block scalar reads its content through parseToken, and the content is
	// part of the literal rather than a value of its own.
	quiet int
	// document is which document of the stream is being walked, counted from 0.
	document int
	// err is the first thing the walk refused.
	err error
}

// Walk reads the YAML stream src and hands each node to v as the parse reaches it.
//
// Walk is experimental: its contract may change without a deprecation.
// Use [Parser.Parse] for a stable API.
// The last entry of a block reaches v only once the parse reads the next token that is not a comment.
//
// Walk does not build the tree. A collection keeps none of its entries,
// so the memory a walk holds grows with the depth of the document and not with its length.
// The returned [ast.File] holds the documents without their bodies, which went to v.
// On error the file is nil.
//
// A node is valid until its Leave returns, as [Visitor] describes.
//
// An anchor is handed over before the node it names and left after it, with [KindAnchor] as the step's In,
// so a writer has the anchor open while it writes the node.
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

// openWalkDocument records which document of the stream the walk is about to
// read, so [Step.Document] tells the nodes of one from the nodes of the next.
//
// It is counted here rather than at the first node of a document, because an
// empty document has no node to count: "---" over "---" over "b: 2" hands over
// only the mapping, and it belongs to document 1.
func (p *Parser) openWalkDocument(n int) {
	if p.walk == nil {
		return
	}
	p.walk.document = n
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
	return p.enterAs(ctx, node, in, false)
}

// enterKey hands a mapping key over before what it holds, for the "?" that
// stands around a key rather than being one.
func (p *Parser) enterKey(ctx context, node ast.Node, in Kind) bool {
	return p.enterAs(ctx, node, in, true)
}

func (p *Parser) enterAs(ctx context, node ast.Node, in Kind, key bool) bool {
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

	// takeKey comes after the guards above: a node that is not handed over has
	// not announced the key, and handKey still has it to hand. It is called
	// whatever key says, so the flag is cleared either way -- left standing it
	// would mark the entry's value as a key too.
	if p.takeKey() {
		key = true
	}

	at := p.step(node)
	at.Key = key
	if !p.walk.visitor.Enter(node, at) {
		p.walk.skip = 1

		return false
	}

	p.walk.key = append(p.walk.key, key)
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
	at := p.step(node)
	at.Key = p.walk.key[len(p.walk.key)-1]
	p.walk.key = p.walk.key[:len(p.walk.key)-1]
	p.walk.visitor.Leave(node, at)
	p.count()
	p.readTo(ctx)
}

// markKey says the next node handed over is a mapping's key.
//
// It is set before the key is parsed rather than after, because a key that is
// an anchor, a tag or a "?" stands around what it holds and goes over as it
// opens -- before parseMapKey has returned anything to hand over.
func (p *Parser) markKey() {
	if p.walk != nil {
		p.walk.keyNext = true
	}
}

// takeKey reports whether the node about to go over is the key that markKey
// announced, and forgets it.
func (p *Parser) takeKey() bool {
	if p.walk == nil || !p.walk.keyNext {
		return false
	}
	p.walk.keyNext = false

	return true
}

// handKey gives a mapping's key over, before its value is parsed.
//
// A key that opened on its way here -- a "?", an anchor, a tag, a collection --
// took markKey's flag as it went over, and handing it again would put it in the
// mapping twice. The flag still standing is what says nothing has gone over
// yet: parseScalarValue builds a scalar key without handing it anywhere.
func (p *Parser) handKey(ctx context, node ast.Node) {
	if p.walk != nil && !p.walk.keyNext {
		return
	}
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
	// takeKey is called whatever key says, so the flag is cleared either way:
	// left standing it would mark the entry's value as a key too.
	if p.takeKey() {
		key = true
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
	at := Step{Depth: len(p.walk.in), Document: p.walk.document}
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
	tk := p.body.at(p.body.idx)
	if tk == nil {
		return
	}
	p.tokens.SetTail(int(tk.Seq()))

	if p.reader != nil {
		p.reader.g.Release(p.tokens.Released(), p.releasedByTape)
	}
}

// releasedByTape answers whether the tape has given up the chunk holding seq.
//
// It stands in for the group.Grouper's own liveness: a cell of its own belongs to a
// run of the stream, and the tape knows what it still holds -- the tail it has
// been given, what holdRun saved, and what an anchor pinned.
func (p *Parser) releasedByTape(seq int32) bool {
	_, held := p.tokens.Generation(int(seq))

	return !held
}

/*
// saveHere keeps the chunks holding [from, to] without holding the tape first,
// for a run whose extent is already known.
func (p *Parser) saveHere(from, to int32) {
	if p.walk == nil || p.tokens == nil {
		return
	}
	p.tokens.Save(int(from), int(to))
}
*/

// holdRun keeps the chunk holding seq while a construct that began there is
// read. releaseRun gives it back, and every caller defers one against the other.
//
// The descent reads a construct's own tokens again after everything under it:
// parseMapEntry reads its key's group once the value below it is parsed, and a
// sequence reads the '-' its entries are lined up against. The tail follows the
// outermost run and passes those, so the construct says it still wants them.
//
// One chunk per level open at once, so what this holds is the depth of the
// document and not its length.
//
// The pair takes the sequence rather than holdRun returning what undoes it: a
// closure escapes, and this runs once per mapping and once per sequence -- a
// megabyte of them on citm_catalog, 14% of what the conversion allocated.
func (p *Parser) holdRun(seq int32) {
	if p.walk == nil || p.tokens == nil {
		return
	}
	p.tokens.Save(int(seq), int(seq))
}

// releaseRun gives back the chunk holdRun kept.
func (p *Parser) releaseRun(seq int32) {
	if p.walk == nil || p.tokens == nil {
		return
	}
	p.tokens.Release(int(seq), int(seq))
}
