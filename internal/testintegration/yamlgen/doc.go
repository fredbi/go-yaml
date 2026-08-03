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
//
// # The direction this package cannot see
//
// [Ledger] records one kind of defect: a valid document the library refuses, or
// reads as the wrong value. It cannot record the opposite -- an invalid
// document the library reads anyway -- and no amount of running it deeper will
// change that.
//
// The reason is structural, not accidental. TestEveryEmittedDocumentIsValidYAML
// holds every generated document to the YAML 1.2 grammar, so being valid is a
// property of the corpus rather than a question asked of it. The library
// accepting one is never news.
//
// Finding the other direction needs documents that are not YAML, and the
// recognizer is what makes generating them practical. YAML is nearly total, so
// a mutation of a valid document is usually another valid document; without an
// oracle there is no way to tell which mutants are worth trying, and a mutator
// spends almost its whole budget re-asking the question already answered above.
// With one, the mutator can be as crude as you like:
//
//	oracle := grammar.NewRecognizer(1024)
//	broken := mutate(rt, yamlgen.Emit(value, style))
//
//	if oracle.Stream([]byte(broken)).OK {
//		return // still YAML: nothing here the existing tests do not cover
//	}
//
//	var got any
//	if err := yaml.Unmarshal([]byte(broken), &got); err == nil {
//		// The library read a document the grammar refuses. Candidate.
//	}
//
// Mutations worth making are the ones that break a rule rather than a byte:
// a tab in the indentation, an indent that does not line up, an unterminated
// quote, an unknown escape, an alias with no anchor, a document marker inside a
// scalar, a second colon in an implicit key, a block scalar indicator wider
// than its content.
//
// # Reading a wrongly-accepted finding
//
// Two things about that ledger differ from this one, and both matter.
//
// It is keyed on documents rather than on shapes. [Divergence.Match] describes
// a shape because every run draws different documents and there is nothing to
// name; a mutation corpus is the opposite -- the interesting mutants are few,
// and each one can be checked in, named and shrunk to the smallest document the
// oracle still refuses.
//
// And it rests on the oracle being right in the harder direction. A finding
// here is entirely a claim that the grammar's rejection is correct, where a
// finding in [Ledger] only needs its acceptance to be. The recognizer has
// already been wrong in exactly that way once, refusing a compact collection
// written under a wider parent -- which the renderer emits, and which every
// other parser reads. Had that surfaced while hunting over-permissiveness it
// would have read as a library defect. So before an entry goes in: shrink it,
// name the production that refuses it, and check that production against the
// spec text rather than against the recognizer.
//
// Severity does not follow the direction. Accepting an invalid document is
// harmless when the value is the obvious one and worse than a rejection when it
// is not, because nothing downstream has any way to notice.
package yamlgen
