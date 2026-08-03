// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import "math"

// nNull is the n the grammar passes as `null`, in the rules that match an
// implicit key: there is no indentation level to compare against there.
const nNull = math.MinInt32

// mAuto is the m a block scalar's header leaves behind when it states no
// indentation indicator: "detect it from the content".
//
// It cannot be detected where it is set, which is why it is a sentinel rather
// than a number. c-indentation-indicator runs before s-b-comment, so at that
// point the content has not been reached and there is nothing to measure. It is
// resolved one step later, where the grammar computes n+m to enter
// l-literal-content, and there the position is the first content line.
const mAuto = math.MinInt32 + 1

// indentImpossible is an indentation no document can satisfy.
//
// It is how auto-detection reports the one case the spec calls an error rather
// than a width: a leading empty line indented further than the first non-empty
// line. Returning a width there would pick some reading of a document that has
// none, and the readings differ -- taking the first non-empty line's width
// accepts it, which is exactly wrong.
const indentImpossible = math.MaxInt32 / 2

// env holds the four variables the grammar's rules are parameterized by.
//
// It is passed by value and handed back by every matcher. Passing it down is
// what a rule call needs: bind the parameters the callee declares, inherit the
// rest, which is what copying a four-word struct and overwriting some fields
// does. Handing it back is what (set) needs: c-b-block-header does not consume
// the header so much as report it, and the m and t it reports are read by the
// step after it rather than by anything inside it.
//
// Returning the env rather than pointing at one is what makes backtracking free
// and correct at the same time. A failed alternative's env is discarded by the
// combinator that tried it, because the combinator still holds the one it
// started with; there is nothing to undo and so nothing to forget to undo.
type env struct {
	n int // indentation level, or nNull
	m int // extra indentation, detected rather than passed
	c ctxCode
	t chompCodeT
}

// The context and chomping names are interned at compile time, not at
// comparison time. Holding them as strings and looking them up on each step
// costs more than everything else the engine does put together: they are
// compared on every rule invocation and again on every memo probe.
type (
	ctxCode    uint8
	chompCodeT uint8
)

// state is the part of a recognition that is not the env: the input, where we
// are in it, and the memo table.
type state struct {
	src []byte
	pos int

	// limit is the position the enclosing sequence must complete before, set by
	// (max) and by (exclude). Negative when no limit is in force.
	//
	// The two forms want the same thing said differently -- an implicit key
	// runs out after 1024 characters, a bare document runs out where the next
	// c-forbidden marker begins -- and both are naturally expressed as a byte
	// nothing may be read at or past.
	limit int

	// mark is where the innermost (<<<) form started, so that (match) can
	// report what it has consumed. The two indentation comparison rules are
	// its only users.
	mark int

	// memo is indexed by position, each bucket holding the invocations
	// recorded at it. A hash map keyed on the whole invocation is the obvious
	// structure and the wrong one: position is already a dense small integer,
	// so indexing on it directly turns a hash of a twenty-byte key into an
	// array index and a short scan. Nil disables memoization.
	memo [][]memoEntry

	// Instrumentation. Free when nothing reads them, and the whole point of
	// the throughput test is to read them.
	steps int64
	hits  int64
}

// memoEntry records what one rule decided at one position.
//
// It carries the env because the grammar is a parameterized PEG: the same rule
// at the same position genuinely decides differently under a different
// indentation or context. That is also why the hit rate is what it is.
//
// It carries the limit for the same reason. A rule asked inside an implicit key
// or inside a bare document is asked with less input available than the same
// rule outside one, and a table that could not tell those apart would answer
// the second question with the first one's answer.
type memoEntry struct {
	rule  int32
	n     int32
	m     int32
	limit int32
	end   int32
	c     ctxCode
	t     chompCodeT
	ok    bool

	// outM and outT are what the rule left m and t at, for the rules that
	// report rather than only consume. Only m and t are ever (set), which
	// newCompiler checks rather than assumes.
	outM int32
	outT chompCodeT
}

func (e memoEntry) matches(rule int32, v env, limit int) bool {
	return e.rule == rule && e.n == int32(v.n) && e.m == int32(v.m) &&
		e.c == v.c && e.t == v.t && e.limit == int32(limit)
}

// contexts and chomping modes are interned to fit the memo key, and so that
// comparing them is an integer compare rather than a string compare.
var contextCode = map[string]ctxCode{
	"":          0,
	"block-in":  1,
	"block-out": 2,
	"block-key": 3,
	"flow-in":   4,
	"flow-out":  5,
	"flow-key":  6,
}

var chompCode = map[string]chompCodeT{
	"":      0,
	"strip": 1,
	"clip":  2,
	"keep":  3,
}

// numArms bounds the (case) and (flip) arm tables, which are indexed by code
// rather than looked up by name. Contexts and chomping modes share one index
// space, so one table size serves both.
const (
	numContexts = 7
	numChomps   = 4
	numArms     = max(numContexts, numChomps)
)

// reset prepares the state for another document, keeping the memo buckets'
// capacity so that a reused recognizer stops allocating after the first few
// documents.
func (s *state) reset(src []byte, memoize bool) {
	s.src = src
	s.pos = 0
	s.limit = -1
	s.mark = 0
	s.steps = 0
	s.hits = 0

	if !memoize {
		s.memo = nil

		return
	}

	if cap(s.memo) < len(src)+1 {
		s.memo = make([][]memoEntry, len(src)+1)

		return
	}

	s.memo = s.memo[:len(src)+1]
	for i := range s.memo {
		s.memo[i] = s.memo[i][:0]
	}
}

// expr is a compiled matcher: it either consumes input and reports true, or
// leaves the position where it found it and reports false.
//
// Every combinator restores s.pos itself on failure, so a caller may rely on a
// false result meaning nothing moved. The env it returns is meaningful only
// when it reports true; on failure it hands back the one it was given, so that
// a caller which ignores the distinction cannot go wrong by accident.
type expr func(s *state, e env) (env, bool)

// value is a compiled argument expression. The grammar's arguments are either
// integers (n, m, and arithmetic on them) or strings (c, t, and the (flip)
// rules that map one context to another), so one type covers both.
type value func(s *state, e env) any
