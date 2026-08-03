// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import "fmt"

// Result is the oracle's verdict, with the instrumentation that says what it
// cost to reach.
type Result struct {
	// OK reports whether the whole input matched the production.
	OK bool
	// Steps counts rule invocations, memo hits included. It is the honest
	// measure of how much work a PEG over YAML actually is.
	Steps int64
	// Hits counts the invocations the memo table answered.
	Hits int64
}

func (r Result) String() string {
	return fmt.Sprintf("ok=%t steps=%d hits=%d (%.0f%%)", r.OK, r.Steps, r.Hits,
		100*float64(r.Hits)/float64(max(r.Steps, 1)))
}

// FlowNode reports whether src is exactly one YAML 1.2 flow node.
//
// Narrower than Stream, and worth keeping separate: a failure against one node
// says the fault is in that node, where the same failure against a whole stream
// could be anywhere above it.
func FlowNode(src []byte) Result {
	return Match("ns-flow-node", src, 0, "flow-out")
}

// Stream reports whether src is a valid YAML 1.2 stream.
//
// This is the question the conformance harness asks: not whether some fragment
// is well formed, but whether a document the generator wrote is one the library
// is obliged to read.
func Stream(src []byte) Result {
	return Match("l-yaml-stream", src, -1, "block-in")
}

// Match reports whether src is exactly one instance of the named production,
// entered at indentation n in context c.
func Match(rule string, src []byte, n int, c string) Result {
	s, ok := grammarCompiler.slots[rule]
	if !ok {
		panic(fmt.Sprintf("grammar: no production named %q", rule))
	}

	var st state
	st.reset(src, true)

	_, matched := invoke(s, &st, env{n: n, m: 0, c: contextCode[c], t: chompCode["clip"]})

	return Result{
		OK:    matched && st.pos == len(src),
		Steps: st.steps,
		Hits:  st.hits,
	}
}

// MatchNoMemo is Match with the memo table disabled, so that a test can check
// memoization changed nothing, and report what it is worth rather than assume
// it.
func MatchNoMemo(rule string, src []byte, n int, c string) Result {
	s, ok := grammarCompiler.slots[rule]
	if !ok {
		panic(fmt.Sprintf("grammar: no production named %q", rule))
	}

	var st state
	st.reset(src, false)

	_, matched := invoke(s, &st, env{n: n, m: 0, c: contextCode[c], t: chompCode["clip"]})

	return Result{
		OK:    matched && st.pos == len(src),
		Steps: st.steps,
		Hits:  st.hits,
	}
}

// Rules reports how many productions the grammar defines, so a test can assert
// the whole file compiled rather than some prefix of it.
func Rules() int { return len(grammarCompiler.slots) }

// Recognizer is a reusable oracle.
//
// Match allocates a memo table per call, and the table is where nearly all of
// the cost is. A generative harness calls the oracle in a loop, so it can hold
// one of these and keep the table across calls instead.
//
// A Recognizer is not safe for concurrent use. Give each goroutine its own.
type Recognizer struct {
	st state
}

// NewRecognizer returns a Recognizer whose memo table is sized for documents of
// about hint bytes.
func NewRecognizer(hint int) *Recognizer {
	return &Recognizer{st: state{memo: make([][]memoEntry, 0, hint+1)}}
}

// Match reports whether src is exactly one instance of the named production.
func (r *Recognizer) Match(rule string, src []byte, n int, c string) Result {
	s, ok := grammarCompiler.slots[rule]
	if !ok {
		panic(fmt.Sprintf("grammar: no production named %q", rule))
	}

	r.st.reset(src, true)

	_, matched := invoke(s, &r.st, env{n: n, m: 0, c: contextCode[c], t: chompCode["clip"]})

	return Result{
		OK:    matched && r.st.pos == len(src),
		Steps: r.st.steps,
		Hits:  r.st.hits,
	}
}

// FlowNode reports whether src is exactly one YAML 1.2 flow node.
func (r *Recognizer) FlowNode(src []byte) Result {
	return r.Match("ns-flow-node", src, 0, "flow-out")
}

// Stream reports whether src is a valid YAML 1.2 stream.
func (r *Recognizer) Stream(src []byte) Result {
	return r.Match("l-yaml-stream", src, -1, "block-in")
}
