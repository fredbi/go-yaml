// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	yamlerrors "github.com/go-openapi/go-yaml/errors"
	"github.com/go-openapi/go-yaml/token"
)

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
		stageAnchors,
	}
	flushers = []flusher{
		nil,
		flushBlockScalars,
		flushAnchors,
	}
}

// feed walks tk through the chain and appends what comes out the far end.
//
// A token no stage is waiting for and no stage reads goes straight out. That is
// the common case by a distance: a String, an Integer, a Float or a Bool means
// nothing to any stage, and those are 53.8% of the tokens in the workloads.
// Walking the chain to learn as much costs a call and a type test at every
// stage; the table answers it once.
func (g *grouper) feed(tk *tapeToken, out []*tapeToken) []*tapeToken {
	if g.settled() && !readByAStage[tk.Type()] {
		// stageLineComments notes every token it hands on, a comment closing a
		// line attaching to whatever stood before it. Taking the short way
		// still owes it that note.
		g.lineComment = tk

		return append(out, tk)
	}

	return g.pass(-1, tk, out)
}

// settled reports whether every stage has handed on what it was holding. Where
// one is still waiting the token has to walk the chain, whatever its type: it
// may be what the waiting stage was waiting for.
//
// g.lineComment is not among them. It is not a token held back but a note of
// the one last handed on, so that a comment closing a line finds what it
// closes; feed keeps it up to date on the short way.
func (g *grouper) settled() bool {
	return g.blockHeader == nil &&
		g.anchor == nil && g.name == nil && g.alias == nil
}

// readByAStage says which token types a stage reads. Every other type walks the
// chain only to be handed from one stage to the next.
var readByAStage = map[token.Type]bool{
	token.CommentType:       true, // stageLineComments
	token.LiteralType:       true, // stageBlockScalars
	token.FoldedType:        true,
	token.AnchorType:        true, // stageAnchors
	token.AliasType:         true,
	token.SequenceEntryType: true,
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

// stageAnchors joins "&" with the name after it, that name with what it names,
// and "*" with the name it stands for.
func stageAnchors(g *grouper, at int, tk *tapeToken, out []*tapeToken) []*tapeToken {
	switch {
	case g.alias != nil:
		grouped := g.group2(TokenGroupAlias, g.alias, tk)
		g.alias = nil

		return g.pass(at, grouped, out)
	case g.anchor != nil:
		g.name, g.anchor = g.group2(TokenGroupAnchorName, g.anchor, tk), nil

		return out
	case g.name != nil:
		sameLine := g.name.Line() == tk.Line()
		if sameLine && tk.Type() == token.SequenceEntryType {
			g.fail(yamlerrors.NewSyntax("sequence entries are not allowed after anchor on the same line", tk.RawToken()))

			return out
		}
		if sameLine && isScalarType(tk) {
			grouped := g.group2(TokenGroupAnchor, g.name, tk)
			g.name = nil

			return g.pass(at, grouped, out)
		}

		// The anchor names the empty node, and tk is read as any other token
		// would be: two tokens leave the stage for the one that arrived.
		out = g.pass(at, g.name, out)
		g.name = nil
	}

	switch tk.Type() {
	case token.AnchorType:
		g.anchor = tk

		return out
	case token.AliasType:
		g.alias = tk

		return out
	default:
		return g.pass(at, tk, out)
	}
}

// flushAnchors settles what the stage holds when the stream ends: a "&" or "*"
// with no name is an error, and a name with nothing after it names the empty
// node the parser supplies.
func flushAnchors(g *grouper, at int, out []*tapeToken) []*tapeToken {
	switch {
	case g.anchor != nil:
		g.fail(yamlerrors.NewSyntax("undefined anchor name", g.anchor.RawToken()))
	case g.alias != nil:
		g.fail(yamlerrors.NewSyntax("undefined alias name", g.alias.RawToken()))
	case g.name != nil:
		out = g.pass(at, g.name, out)
		g.name = nil
	}

	return out
}
