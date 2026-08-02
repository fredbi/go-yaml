// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package grammar is a spike: a YAML 1.2 recognizer compiled from the spec grammar.
//
// It answers one question -- is this byte sequence a valid YAML 1.2 document --
// and nothing else. No AST, no events, no error messages. That is the whole
// point: an oracle only has to be right, so it can skip everything a parser
// spends its time on.
//
// # Why a recognizer
//
// Generative conformance testing needs a label for every generated document,
// and the generator cannot be the one to supply it: YAML is nearly total, so a
// mutation of a valid document is usually another valid document. An oracle
// built from the grammar supplies the label instead, and the generator is then
// free to be as dumb as we like.
//
// It also localizes faults. When parse(render(ast)) fails we cannot tell
// whether the renderer emitted invalid text or the parser refused valid text.
// Asking the oracle about render(ast) answers that in one call.
//
// # How
//
// github.com/yaml/yaml-grammar publishes the 211 productions of the YAML 1.2
// spec as a data structure. The spec's grammar is a parameterized PEG: rules
// take up to four arguments (n indentation, m extra indentation, c context,
// t chomping), and the DSL adds ordered choice, both lookaheads, lookbehind and
// character-set subtraction.
//
// This package walks that data structure once at init and compiles it into a
// tree of Go closures, then runs the closures. Compiling to closures rather
// than interpreting the data structure directly is the difference between an
// oracle we can run a hundred thousand times and one we cannot: chasing map
// lookups through the decoded JSON on every step costs two orders of magnitude
// more than a closure call.
//
// # Scope of the spike
//
// Implemented: everything reachable from ns-flow-node, which is 131 of the 192
// reachable productions and 24 of the 26 DSL forms -- flow collections, all
// four scalar styles, plain-scalar context sensitivity, comments, separation
// and indentation.
//
// Not implemented: the forms that need mutable parse state, which is to say
// block context and document structure -- (set), (if), (ord), (exclude) and
// <auto-detect-indent>. Every production still compiles, so the whole grammar
// is known to translate; reaching one of those forms panics with its name,
// which keeps the boundary explicit rather than silently failing to match.
//
// # Memoization
//
// Memoization is not an optimization here, it is what makes the recognizer
// total. Nested flow collections are exponential without it -- twenty nested
// sequences take 115 million steps and five seconds, ten take a hundred
// thousand -- and linear with it. It costs roughly two-fold on documents that
// do not backtrack, which is the price of the ones that do.
package grammar
