// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import "math"

// nNull is the n the grammar passes as `null`, in the rules that match an
// implicit key: there is no indentation level to compare against there.
const nNull = math.MinInt32

// env holds the four variables the grammar's rules are parameterized by.
//
// It is passed by value. That is deliberate: a rule call binds only the
// parameters it declares and inherits the rest, which is exactly what copying a
// four-word struct and overwriting some fields does. It also means backtracking
// costs nothing to undo -- the caller still holds its own copy.
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

	// limit is the position an implicit key must complete before, set by the
	// (max) form. Negative when no limit is in force.
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
	// the spike is to read them.
	steps int64
	hits  int64
}

// memoEntry records what one rule decided at one position.
//
// It carries the env because the grammar is a parameterized PEG: the same rule
// at the same position genuinely decides differently under a different
// indentation or context. That is also why the hit rate is what it is.
type memoEntry struct {
	rule int32
	n    int32
	m    int32
	end  int32
	c    ctxCode
	t    chompCodeT
	ok   bool
}

func (e memoEntry) matches(rule int32, v env) bool {
	return e.rule == rule && e.n == int32(v.n) && e.m == int32(v.m) &&
		e.c == v.c && e.t == v.t
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

// numContexts and numChomps bound the (case) and (flip) arm tables, which are
// indexed by code rather than looked up by name.
const (
	numContexts = 7
	numChomps   = 4
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
// false result meaning nothing moved.
type expr func(s *state, e env) bool

// value is a compiled argument expression. The grammar's arguments are either
// integers (n, m, and arithmetic on them) or strings (c, t, and the (flip)
// rules that map one context to another), so one type covers both.
type value func(s *state, e env) any
