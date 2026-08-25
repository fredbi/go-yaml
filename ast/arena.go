// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"strconv"

	"github.com/go-openapi/go-yaml/token"
)

// nodeBlockSize bounds how many nodes of one type an allocation covers.
const (
	minNodeBlock = 16
	maxNodeBlock = 512
)

// block hands out values of one type from an allocation at a time.
type block[T any] struct {
	free []T
}

// next returns the next unused value, taking a new allocation of size when the
// one in hand runs out. Blocks are never reused, so a value handed out stays
// valid for as long as anything points at it.
func (b *block[T]) next(size int) *T {
	if len(b.free) == 0 {
		b.free = make([]T, size)
	}
	v := &b.free[0]
	b.free = b.free[1:]

	return v
}

// slab hands out runs of T from blocks, for a run whose length is already
// known.
type slab[T any] struct {
	free []T
}

// take returns a copy of src that no later take will write over, and nil for an
// empty run.
//
// The capacity is held to the length, so appending to what it returns takes a
// new allocation rather than writing over the run handed out next.
func (s *slab[T]) take(src []T, size int) []T {
	if len(src) == 0 {
		return nil
	}
	if len(s.free) < len(src) {
		s.free = make([]T, max(size, len(src)))
	}

	out := s.free[:len(src):len(src)]
	copy(out, src)
	s.free = s.free[len(src):]

	return out
}

// Arena hands out nodes from blocks rather than one allocation each.
//
// A parser building a tree of N nodes costs N/size allocations rather than N.
// The nodes are addressed by pointer as they are anywhere else, so a node from
// an arena is an [Node] like any other and nothing downstream can tell.
//
// Two things follow from a block being one heap object. A node stays valid for
// as long as anything points at it, whatever happens to the arena -- the arena
// is a source of nodes, not their owner. And a block is freed only when nothing
// points into it, so holding one node of a document holds the block it came
// from, up to 512 nodes. That is the trade: a tree lives and dies together, and
// a caller keeping one node out of a large tree keeps more than it asked for.
//
// The zero Arena is ready to use and allocates blocks of minNodeBlock. Use
// [NewArena] to size them for a document.
type Arena struct {
	strings       block[StringNode]
	integers      block[IntegerNode]
	floats        block[FloatNode]
	bools         block[BoolNode]
	nulls         block[NullNode]
	mappingValues block[MappingValueNode]
	mappings      block[MappingNode]
	sequences     block[SequenceNode]
	sequenceEntry block[SequenceEntryNode]
	// mappingRuns holds the entry lists of the mappings, so that a mapping's
	// Values costs no allocation of its own.
	mappingRuns slab[*MappingValueNode]

	size int
}

// NewArena returns an arena whose blocks are sized for a document of n tokens.
//
// Roughly one token in four becomes a node of any one type, which is where the
// size comes from. It is capped both ways: a long document takes more blocks
// rather than one huge one, and a short document does not pay for a block it
// will use a tenth of.
func NewArena(n int) *Arena {
	return &Arena{size: min(max(n/4, minNodeBlock), maxNodeBlock)}
}

func (a *Arena) blockSize() int {
	if a.size == 0 {
		return minNodeBlock
	}

	return a.size
}

// String returns a [StringNode] for tk, as [String] does.
func (a *Arena) String(tk *token.Token) *StringNode {
	n := a.strings.next(a.blockSize())
	n.Token, n.Value = tk, tk.Value

	return n
}

// Integer returns an [IntegerNode] for tk, as [Integer] does.
func (a *Arena) Integer(tk *token.Token) *IntegerNode {
	n := a.integers.next(a.blockSize())
	n.Token = tk

	return n
}

// Float returns a [FloatNode] for tk, as [Float] does.
func (a *Arena) Float(tk *token.Token) *FloatNode {
	n := a.floats.next(a.blockSize())
	n.Token = tk

	return n
}

// SequenceEntry returns a [SequenceEntryNode] for start, as [SequenceEntry]
// does.
func (a *Arena) SequenceEntry(start *token.Token, value Node, headComment *CommentGroupNode) *SequenceEntryNode {
	n := a.sequenceEntry.next(a.blockSize())
	n.Start, n.Value, n.HeadComment = start, value, headComment

	return n
}

// Bool returns a [BoolNode] for tk, as [Bool] does.
func (a *Arena) Bool(tk *token.Token) *BoolNode {
	b, _ := strconv.ParseBool(tk.Value)
	n := a.bools.next(a.blockSize())
	n.Token, n.Value = tk, b

	return n
}

// Null returns a [NullNode] for tk, as [Null] does.
func (a *Arena) Null(tk *token.Token) *NullNode {
	n := a.nulls.next(a.blockSize())
	n.Token = tk

	return n
}

// MappingValue returns a [MappingValueNode] for tk, as [MappingValue] does.
func (a *Arena) MappingValue(tk *token.Token, key MapKeyNode, value Node) *MappingValueNode {
	n := a.mappingValues.next(a.blockSize())
	n.Start, n.Key, n.Value = tk, key, value

	return n
}

// Mapping returns a [MappingNode] for tk holding values, as [Mapping] does.
//
// values is copied, and the copy is held to its own length: a caller appending
// to the mapping's Values takes an allocation of its own rather than writing
// over another mapping's entries.
func (a *Arena) Mapping(tk *token.Token, isFlowStyle bool, values []*MappingValueNode) *MappingNode {
	n := a.mappings.next(a.blockSize())
	n.Start, n.IsFlowStyle = tk, isFlowStyle
	n.Values = a.mappingRuns.take(values, a.blockSize())

	return n
}

// Sequence returns a [SequenceNode] for tk, as [Sequence] does.
func (a *Arena) Sequence(tk *token.Token, isFlowStyle bool) *SequenceNode {
	n := a.sequences.next(a.blockSize())
	n.Start, n.IsFlowStyle, n.Values = tk, isFlowStyle, []Node{}

	return n
}
