// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package yamlgen generates YAML documents whose meaning is known in advance.
//
// The YAML Test Suite is a sample rather than a measurement: some four hundred
// documents someone thought to write down, over a grammar with four propagated
// parameters, six contexts, three chomping modes and free indentation. What it
// cannot say is that a whole region of that space was tried and nothing broke.
//
// # Meaning, then presentation
//
// A [Value] is what a document means. A [Style] is one way of writing it down.
// Emitting a Value in several Styles gives several documents that look nothing
// alike and must all read back the same, which is a property that needs no
// grammar, no oracle and no reference implementation -- the invariant is
// internal, so every failure names itself.
//
// Generating text first cannot do this, because there is no expected value to
// compare against. Generating the value first is what makes the expectation
// free.
//
// # Why not the library's own renderer
//
// The renderer emits one style. One style cannot demonstrate invariance across
// styles, so [Emit] is a second, deliberately independent writer. It is
// conservative wherever YAML permits something subtle: being narrow costs
// coverage, while being wrong would mean generating documents whose expected
// value we got wrong and then blaming the library for the difference.
// [TestEmitterAgreesOnKnownDocuments] is what holds it to that.
//
// # Divergences
//
// Where the library disagrees with YAML 1.2, the shape of the disagreement goes
// in [Ledger] and the generator keeps producing it. Steering around a defect
// makes the harness quieter and blinder. Each entry is counted as it is drawn
// and as it diverges, so an entry that stops diverging fails the run and gets
// deleted, rather than sitting there forever describing a world that has moved
// on.
package yamlgen
