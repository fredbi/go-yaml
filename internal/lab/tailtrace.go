// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package lab

import (
	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/internal/tokenarena"
	"github.com/go-openapi/go-yaml/parser"
	"github.com/go-openapi/go-yaml/token"
)

// TailTrace is where the tail would stand as a conversion walks a document.
//
// EXPERIMENT (2026-08-30). Converting to JSON keeps no parent node: a container
// is announced, its children are written as they come, and the closing brace
// goes after the last of them. So every token below the node just finished is
// finished with, and the tail can follow the parse.
//
// The parse still holds everything -- it takes a pin and keeps it -- so this
// records where the tail could have gone rather than moving it. The arena is
// fed the same tokens in the same order with the same tail, which gives the
// figure a released pin would give without risking the parse that produced it.
type TailTrace struct {
	// Tokens is how many the document held, and Steps how many times a node
	// completed, so how many times the tail could have moved.
	Tokens, Steps int
	// Waiting is the most nodes that stood completed and not yet taken by the
	// node above them at once. It is what a level gathers, and it is memory.
	Waiting int
	// Reach is how far back the oldest of those sat, counted in completed
	// nodes. It is not memory: it is what stops the tail, since no token below
	// the oldest node still waiting may be given up.
	Reach int

	// Handed is what the tape holds where the parse gives each node to the
	// caller as it finishes it and gathers nothing. The tail follows the last
	// token of the node just handed over.
	Handed tokenarena.Stats
	// Gathered is what it holds where the parse gathers a level's entries
	// before building the node above them, which is what it does today. The
	// tail may not pass the oldest entry still waiting for its parent.
	Gathered tokenarena.Stats
}

// TraceToJSONTail converts src to JSON and reports where the tail would stand,
// both ways.
//
// chunk is the chunk size of the arenas the trace is replayed through.
func TraceToJSONTail(src []byte, chunk int) (TailTrace, error) {
	w := &tailFolder{reach: map[ast.Node]int{}, start: map[ast.Node]int{}}

	p := parser.New(parser.ChunkSize(chunk), parser.OnComplete(w.complete))
	w.tokens = p.Tokens

	if _, err := p.Parse(src); err != nil {
		return TailTrace{}, err
	}

	held := make([]*parser.Token, 0, p.Tokens().Len())
	for tk := range p.Tokens().All() {
		held = append(held, tk)
	}

	return TailTrace{
		Tokens:   len(held),
		Steps:    len(w.handed),
		Waiting:  w.mostWaiting,
		Reach:    w.longestReach,
		Handed:   replay(held, w.handed, chunk),
		Gathered: replay(held, w.gathered, chunk),
	}, nil
}

// replay fills an arena with the tokens and moves the tail to each figure in
// turn, which is what the parse would have done.
//
// The parse reads forward, so by the time a node completes the scanner has
// passed its last token: fill to there, then move the tail.
func replay(held []*parser.Token, tail []int, chunk int) tokenarena.Stats {
	arena := tokenarena.New[parser.Token](chunk)

	var at int
	for _, reach := range tail {
		for at <= reach && at < len(held) {
			arena.Add(*held[at])
			at++
		}
		arena.SetTail(reach)
	}
	for at < len(held) {
		arena.Add(*held[at])
		at++
	}

	return arena.Stats()
}

// tailFolder records where the tail could stand after each node completes,
// under both rules.
type tailFolder struct {
	tokens func() *tokenarena.TokenArena[parser.Token]
	seq    map[*token.Token]int
	reach  map[ast.Node]int
	start  map[ast.Node]int

	// waiting holds the nodes completed and not yet claimed, oldest first, and
	// oldest is how far into it the claimed ones reach. at gives a node's place
	// in it, so claiming one costs no search.
	waiting []ast.Node
	at      map[ast.Node]int
	oldest  int

	pending      int
	mostWaiting  int
	longestReach int

	handed, gathered []int
}

func (w *tailFolder) complete(n ast.Node) {
	reach := w.seqOf(n.GetToken())

	switch t := n.(type) {
	case *ast.MappingValueNode:
		reach = max(reach, w.claim(t.Key), w.claim(t.Value))
	case *ast.MappingNode:
		for _, entry := range t.Values {
			reach = max(reach, w.claim(entry))
		}
	case *ast.SequenceNode:
		for _, value := range t.Values {
			reach = max(reach, w.claim(value))
		}
	}

	w.reach[n] = reach
	w.start[n] = w.seqOf(n.GetToken())
	if w.at == nil {
		w.at = make(map[ast.Node]int)
	}
	w.at[n] = len(w.waiting)
	w.waiting = append(w.waiting, n)
	w.pending++
	w.mostWaiting = max(w.mostWaiting, w.pending)
	w.longestReach = max(w.longestReach, len(w.waiting)-w.oldest)

	// Handing each node over as it is finished, nothing below it is wanted
	// again.
	w.handed = append(w.handed, reach)

	// Gathering a level instead, the tail may not pass the oldest entry still
	// waiting for the node above it.
	for w.oldest < len(w.waiting) && w.waiting[w.oldest] == nil {
		w.oldest++
	}
	switch {
	case w.oldest < len(w.waiting):
		w.gathered = append(w.gathered, w.start[w.waiting[w.oldest]])
	default:
		w.gathered = append(w.gathered, reach)
	}
}

// claim takes what a child reached and forgets it, the way a converter forgets
// a child once it is written.
func (w *tailFolder) claim(n ast.Node) int {
	if n == nil {
		return 0
	}
	reach, ok := w.reach[n]
	if !ok {
		return w.seqOf(n.GetToken())
	}
	delete(w.reach, n)
	delete(w.start, n)

	if i, ok := w.at[n]; ok {
		w.waiting[i] = nil
		delete(w.at, n)
		w.pending--
	}

	return reach
}

func (w *tailFolder) seqOf(tk *token.Token) int {
	if tk == nil {
		return 0
	}
	if w.seq == nil {
		// The first node has finished, so the read is over and the tape holds
		// every token. Numbering them before that would find an empty arena.
		w.seq = sequenceOf(w.tokens())
	}

	return w.seq[tk]
}

// sequenceOf numbers the tokens an arena holds, so a token the tree points at
// can be found on the tape.
func sequenceOf(a *tokenarena.TokenArena[parser.Token]) map[*token.Token]int {
	out := make(map[*token.Token]int, a.Len())

	var i int
	for tk := range a.All() {
		out[tk.RawToken()] = i
		i++
	}

	return out
}
