// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser

import "github.com/go-openapi/go-yaml/token"

// The grouping as a state machine.
//
// The passes it replaces each walked every token of every run and handed the
// whole run to the next: eight walks and eight buffers for a document, and
// 4,318,536 token visits for the 539,817 tokens of the workloads. Only 46.2% of
// tokens mean anything to any pass -- a String, an Integer or a Float reaches
// none of them -- so most of that was carrying a token from one buffer to the
// next.
//
// Here a token walks the stages instead, one at a time. A stage that has no
// interest in its type hands it straight on, which costs a type test rather
// than a copy, and a stage that is holding one keeps it until the token that
// settles it arrives.
//
// The stages are ordered, and the order is what the passes' order was: a later
// stage reads what an earlier one made, so groupAnchorsWithScalarTags sees an
// anchor group rather than the '&' that opened it.
//
// A stage hands a token on by calling [grouper.pass] with its own index, which
// is what keeps this a pipeline without a buffer between every pair of stages.

// stage reads one token and hands on what it has settled, which may be nothing,
// the token itself, or a group built from tokens it was holding.
//
// at is the stage's own place in the chain: to hand a token to the next stage
// it calls g.pass(at, tk, out).
type stage func(g *grouper, at int, tk *tapeToken, out []*tapeToken) []*tapeToken

// flusher hands on whatever a stage still holds when the stream ends.
type flusher func(g *grouper, at int, out []*tapeToken) []*tapeToken

// stages is the chain, in the order the passes ran, and flushers matches it one
// for one with a nil where a stage holds nothing.
//
// Both are filled in init: a stage hands on through grouper.pass, which reads
// stages, so naming them here directly is a cycle the compiler refuses.
var (
	stages   []stage
	flushers []flusher
)

func init() {
	stages = []stage{
		stageLineComments,
		stageBlockScalars,
	}
	flushers = []flusher{
		nil,
		flushBlockScalars,
	}
}

// feed walks tk through the chain and appends what comes out the far end.
func (g *grouper) feed(tk *tapeToken, out []*tapeToken) []*tapeToken {
	return g.pass(-1, tk, out)
}

// pass hands tk to the stage after at, or to the output where at is the last.
func (g *grouper) pass(at int, tk *tapeToken, out []*tapeToken) []*tapeToken {
	next := at + 1
	if next >= len(stages) {
		return append(out, tk)
	}

	return stages[next](g, next, tk, out)
}

// finish empties the chain, each stage's leavings walking the stages after it.
func (g *grouper) finish(out []*tapeToken) []*tapeToken {
	for i, flush := range flushers {
		if flush != nil {
			out = flush(g, i, out)
		}
	}

	return out
}

// stageLineComments gives a comment closing a token's line to that token.
func stageLineComments(g *grouper, at int, tk *tapeToken, out []*tapeToken) []*tapeToken {
	if tk.Type() == token.CommentType && g.lineComment != nil && g.lineComment.Line() == tk.Line() {
		g.setLineComment(g.lineComment, tk.RawToken())

		return out
	}

	g.lineComment = tk

	return g.pass(at, tk, out)
}

// stageBlockScalars joins a "|" or ">" header with the content that follows it.
func stageBlockScalars(g *grouper, at int, tk *tapeToken, out []*tapeToken) []*tapeToken {
	if g.blockHeader != nil {
		// Whatever follows the header is its content, read as it stands: a
		// second "|" is content, not another header.
		grouped := g.group2(g.blockType, g.blockHeader, tk)
		g.blockHeader = nil

		return g.pass(at, grouped, out)
	}

	switch tk.Type() {
	case token.LiteralType:
		g.blockHeader, g.blockType = tk, TokenGroupLiteral

		return out
	case token.FoldedType:
		g.blockHeader, g.blockType = tk, TokenGroupFolded

		return out
	default:
		return g.pass(at, tk, out)
	}
}

// flushBlockScalars hands on a header that ended the stream, which has no
// content and so is a group of one.
func flushBlockScalars(g *grouper, at int, out []*tapeToken) []*tapeToken {
	if g.blockHeader == nil {
		return out
	}

	grouped := g.group1(g.blockType, g.blockHeader)
	g.blockHeader = nil

	return g.pass(at, grouped, out)
}
