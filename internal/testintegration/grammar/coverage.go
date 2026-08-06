// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import (
	"fmt"
	"slices"
	"strings"
)

// Coverage records which productions a recognition entered, and in which
// context.
//
// # Why this is not the compiler's coverage
//
// The recognizer is a closure interpreter: every production is a slot holding
// an expr built from shared combinators, so one copy of choice's machine code
// serves around a hundred distinct choices. Go's edge instrumentation saturates
// on it almost at once and can offer no gradient at all toward a production
// nothing has reached. Exploring the recognizer's *code* is not exploring the
// *grammar*; the code has stopped telling them apart.
//
// So production coverage is measured semantically, at [invoke], which is the
// single funnel every production entry passes through.
//
// # Production by context
//
// The bucket is a production paired with the context it was entered in, not the
// production alone. The same rule genuinely decides differently in block-in and
// in flow-key, and a vector that could not tell those apart would call a
// grammar covered while a whole half of it had never run.
//
// The indentation parameters n and m are deliberately left out. They are
// unbounded, so they need a binning nobody has designed, and the omission is a
// known gap rather than a judgement that indentation does not matter -- it is
// exactly where both this library and this recognizer have had their bugs.
//
// # Attempts and successes are different questions
//
// invoke fires on backtracked attempts too. "The grammar considered this rule
// here" and "this rule matched here" are both worth knowing and are not the
// same thing, so they get separate counters rather than a shared one.
//
// # Indices are not portable
//
// A bucket index is a production id, and ids are assigned per compiled
// [Grammar]. They are stable across processes -- productions are numbered in
// name order for exactly this reason -- but they are not stable across grammar
// *revisions*. Persist documents, never vectors.
type Coverage struct {
	g         *Grammar
	attempts  []uint32
	successes []uint32
}

// NewCoverage returns a vector sized for this grammar, with every bucket empty.
func (g *Grammar) NewCoverage() *Coverage {
	n := len(g.slots) * numContexts

	return &Coverage{
		g:         g,
		attempts:  make([]uint32, n),
		successes: make([]uint32, n),
	}
}

func (c *Coverage) attempt(id int32, ctx ctxCode) {
	c.attempts[int(id)*numContexts+int(ctx)]++
}

func (c *Coverage) succeed(id int32, ctx ctxCode) {
	c.successes[int(id)*numContexts+int(ctx)]++
}

// Reset empties every bucket, keeping the allocation, so that a builder asking
// about one document after another does not allocate per document.
func (c *Coverage) Reset() {
	clear(c.attempts)
	clear(c.successes)
}

// Buckets is how many production-by-context pairs the vector holds.
//
// Most of them are unreachable: the great majority of productions are only ever
// entered in one or two contexts, so a full vector is not the target and a
// percentage against this number means very little.
func (c *Coverage) Buckets() int { return len(c.attempts) }

// Reached is how many buckets have been entered at least once.
func (c *Coverage) Reached() int { return countNonZero(c.attempts) }

// Matched is how many buckets have been entered and matched at least once.
//
// The gap between this and [Coverage.Reached] is the part of the grammar that
// has been *considered* but never satisfied, which is a different kind of hole
// and often the more interesting one.
func (c *Coverage) Matched() int { return countNonZero(c.successes) }

func countNonZero(v []uint32) int {
	var n int

	for _, at := range v {
		if at > 0 {
			n++
		}
	}

	return n
}

// Merge folds another vector into this one. Both must come from the same
// grammar.
func (c *Coverage) Merge(o *Coverage) {
	c.mustAgree(o)

	for i, at := range o.attempts {
		c.attempts[i] += at
		c.successes[i] += o.successes[i]
	}
}

// AddsTo reports whether this vector reaches a bucket that seen does not.
//
// This is the greedy minimizer's whole decision: keep a candidate exactly when
// it takes the corpus somewhere the corpus has not been. It deliberately says
// nothing about how *often* a bucket was reached, because a document that
// enters a production a thousand times is not more valuable than one that
// enters it once -- and selecting on counts would quietly prefer large
// documents, which is the opposite of what a corpus wants.
func (c *Coverage) AddsTo(seen *Coverage) bool {
	c.mustAgree(seen)

	for i, at := range c.attempts {
		if at > 0 && seen.attempts[i] == 0 {
			return true
		}

		if c.successes[i] > 0 && seen.successes[i] == 0 {
			return true
		}
	}

	return false
}

func (c *Coverage) mustAgree(o *Coverage) {
	if c.g != o.g {
		panic("grammar: coverage vectors from different grammars")
	}
}

// Unreached names the productions no context has ever entered, in name order.
//
// A production here is one nothing in the corpus exercises at all, which is the
// strongest kind of gap: not a rule reached in too few contexts, but a rule the
// generator has never once produced a document for.
//
// The productions that nothing in the *grammar* refers to are left out. They
// can never be entered from any start symbol, so listing them would mean
// reporting nineteen gaps that no corpus can ever close and burying the real
// ones underneath -- see [Grammar.Unreferenced].
func (c *Coverage) Unreached() []string {
	var out []string

	for name, s := range c.g.slots {
		if !c.enteredAnywhere(s.id) && !slices.Contains(c.g.unreferenced, name) {
			out = append(out, name)
		}
	}

	slices.Sort(out)

	return out
}

// Reachable is how many productions any document could enter.
//
// It is the rule count less the productions that nothing refers to *and*
// nothing entered. That second condition is what tells a start symbol from a
// decorative rule: both are unreferenced, since nothing in a grammar refers to
// its root, and the difference is that a recognition begins at one of them.
//
// This is the denominator worth quoting. Against the full rule count a corpus
// looks permanently short by however many named-but-unused productions the
// grammar happens to define -- nineteen in YAML 1.2, every one of them an
// indicator character the spec names for the prose and then writes literally
// wherever it is actually used.
func (c *Coverage) Reachable() int {
	dead := 0

	for _, name := range c.g.unreferenced {
		if !c.enteredAnywhere(c.g.slots[name].id) {
			dead++
		}
	}

	return len(c.g.slots) - dead
}

// Considered names the productions that were entered somewhere and never
// matched anywhere.
//
// These are the rules the grammar keeps trying and the corpus never satisfies.
// A rule that is always attempted and never matched is either genuinely
// unreachable from the start symbol, or names a shape nothing generates -- and
// the two look identical from here, which is why this reports rather than
// fails.
func (c *Coverage) Considered() []string {
	var out []string

	for name, s := range c.g.slots {
		if c.enteredAnywhere(s.id) && !c.matchedAnywhere(s.id) {
			out = append(out, name)
		}
	}

	slices.Sort(out)

	return out
}

func (c *Coverage) enteredAnywhere(id int32) bool {
	return slices.ContainsFunc(c.attempts[int(id)*numContexts:(int(id)+1)*numContexts],
		func(at uint32) bool { return at > 0 })
}

func (c *Coverage) matchedAnywhere(id int32) bool {
	return slices.ContainsFunc(c.successes[int(id)*numContexts:(int(id)+1)*numContexts],
		func(at uint32) bool { return at > 0 })
}

// Entered reports how often a production was entered and matched in one
// context, naming the context the way the grammar does.
func (c *Coverage) Entered(rule, ctx string) (attempts, matches uint32) {
	s := c.g.slotFor(rule)
	code, ok := contextCode[ctx]

	if !ok {
		panic(fmt.Sprintf("grammar: no context named %q", ctx))
	}

	at := int(s.id)*numContexts + int(code)

	return c.attempts[at], c.successes[at]
}

// String summarizes what has been reached, for a test that wants one line.
//
// Entered and matched are reported apart because they behave differently, and
// the difference is the finding: entry saturates almost at once -- both the
// YAML and the JSON official suites enter every reachable production -- while
// matching does not. A corpus measured on entry alone would report itself
// finished while a dozen rules had never once been satisfied.
func (c *Coverage) String() string {
	var entered, matched int

	for _, s := range c.g.slots {
		if c.enteredAnywhere(s.id) {
			entered++
		}

		if c.matchedAnywhere(s.id) {
			matched++
		}
	}

	return fmt.Sprintf("%d/%d reachable productions entered, %d matched; %d buckets reached",
		entered, c.Reachable(), matched, c.Reached())
}

// Report lists what has not been reached, in the shape a work list wants.
func (c *Coverage) Report() string {
	var b strings.Builder

	b.WriteString(c.String())

	if missing := c.Unreached(); len(missing) > 0 {
		fmt.Fprintf(&b, "\n\nnever entered (%d):\n  %s", len(missing), strings.Join(missing, "\n  "))
	}

	if considered := c.Considered(); len(considered) > 0 {
		fmt.Fprintf(&b, "\n\nentered but never matched (%d):\n  %s",
			len(considered), strings.Join(considered, "\n  "))
	}

	return b.String()
}
