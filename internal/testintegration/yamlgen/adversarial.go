// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import "strings"

// Depth is a document shape written to a nesting depth the generator's own
// values never reach.
//
// [Values] draws trees at most maxDepth deep, because a deep tree says nothing
// about meaning that a shallow one does not. Cost is the other question. A
// parser that is quadratic in nesting depth reads every document in this
// repository's corpus without complaint and takes 67 seconds over 400,000
// brackets, so the defect lives entirely in the shape of the curve and not in
// any one document's verdict.
//
// These carry no [Value]: their meaning is uninteresting and their point is
// their size. Use [DeepDocument] to write one and measure what reading it
// costs, then compare the cost at n against the cost at 4n.
type Depth int

const (
	// FlowSeqNesting is [[[[...]]]], the cheapest deep document to write: one
	// byte of opening bracket per level.
	FlowSeqNesting Depth = iota
	// FlowMapNesting is {a: {a: {a: ...}}}.
	FlowMapNesting
	// BlockSeqCompact is `- - - - x`, a block sequence nested on one line. Each
	// level costs two bytes and no line break, so the depth outruns the line
	// count.
	BlockSeqCompact
	// BlockSeqIndented is a block sequence nested one level per line, which
	// costs a line and a growing indent per level -- so the document is
	// quadratic in its own depth before the parser sees it.
	BlockSeqIndented
	// BlockMapIndented is the same for mappings: `a:` then `  a:` and so on.
	BlockMapIndented
	// AliasChain anchors each level and aliases the one before it, so the
	// document is linear and the value it denotes is not.
	AliasChain
	// FlatSeq is the control: n entries at one level, nested nowhere. Reading
	// it is linear in every parser, so a curve that bends here is measuring the
	// harness rather than the parser.
	FlatSeq
)

func (d Depth) String() string {
	switch d {
	case FlowMapNesting:
		return "flow-map-nesting"
	case BlockSeqCompact:
		return "block-seq-compact"
	case BlockSeqIndented:
		return "block-seq-indented"
	case BlockMapIndented:
		return "block-map-indented"
	case AliasChain:
		return "alias-chain"
	case FlatSeq:
		return "flat-seq"
	default:
		return "flow-seq-nesting"
	}
}

// Quadratic reports whether writing this shape to depth n takes bytes
// proportional to n squared rather than to n.
//
// Two shapes indent one space further per level, so a curve measured against
// their depth is measuring the wrong variable. Measure those against the byte
// count instead, which [DeepDocument] returns anyway.
func (d Depth) Quadratic() bool {
	return d == BlockSeqIndented || d == BlockMapIndented
}

// DeepDocument writes one, nested n levels deep.
//
// Every shape produces a document the YAML 1.2 grammar accepts, which is what
// makes a cost measurement over it a statement about the parser. A document a
// parser refuses tells you how long a refusal takes.
func DeepDocument(d Depth, n int) []byte {
	var b strings.Builder

	switch d {
	case FlowMapNesting:
		b.Grow(5*n + 2)
		for range n {
			b.WriteString("{a: ")
		}
		b.WriteString("x")
		b.WriteString(strings.Repeat("}", n))
		b.WriteString("\n")
	case BlockSeqCompact:
		b.Grow(2*n + 2)
		b.WriteString(strings.Repeat("- ", n))
		b.WriteString("x\n")
	case BlockSeqIndented:
		b.Grow(n*n/2 + 3*n)
		for i := range n {
			b.WriteString(strings.Repeat(" ", i))
			b.WriteString("-\n")
		}
		b.WriteString(strings.Repeat(" ", n))
		b.WriteString("- x\n")
	case BlockMapIndented:
		b.Grow(n*n/2 + 4*n)
		for i := range n {
			b.WriteString(strings.Repeat(" ", i))
			b.WriteString("a:\n")
		}
		b.WriteString(strings.Repeat(" ", n))
		b.WriteString("a: x\n")
	case AliasChain:
		// Each level names the one below it twice, so the document is n lines
		// long and the tree it denotes has 2^n leaves. Nothing here builds that
		// tree; the point is that a parser must not either.
		b.Grow(20 * n)
		b.WriteString("a0: &a0 x\n")
		for i := 1; i <= n; i++ {
			b.WriteString("a")
			writeInt(&b, i)
			b.WriteString(": &a")
			writeInt(&b, i)
			b.WriteString(" [*a")
			writeInt(&b, i-1)
			b.WriteString(", *a")
			writeInt(&b, i-1)
			b.WriteString("]\n")
		}
	case FlatSeq:
		b.Grow(4 * n)
		for range n {
			b.WriteString("- x\n")
		}
	default:
		b.Grow(2*n + 2)
		b.WriteString(strings.Repeat("[", n))
		b.WriteString(strings.Repeat("]", n))
		b.WriteString("\n")
	}

	return []byte(b.String())
}

func writeInt(b *strings.Builder, n int) {
	if n >= 10 {
		writeInt(b, n/10)
	}
	b.WriteByte(byte('0' + n%10))
}
