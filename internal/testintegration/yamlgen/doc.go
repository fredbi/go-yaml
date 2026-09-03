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
// # The axes that cost nothing and found the most
//
// [Style.Break] writes the same document with LF, with CRLF and with a lone
// carriage return. It is one [strings.ReplaceAll] at the end of [Emit], it
// crosses every other axis for free, and it opened two [Ledger] entries on the
// run it was added. [Style.PropertyOrder] is the same shape of thing -- YAML
// lets a node's anchor and tag come in either order, so writing the tag first
// costs one swap -- and it opened three more. Reach for that shape of axis
// first.
//
// [Depth] and [DeepDocument] are the other direction: documents with no [Value]
// behind them at all, nested past anything a person would write. Their point is
// the cost of reading them. A parser that is quadratic in nesting depth reads
// every document this package generates correctly, so nothing here that
// compares an outcome can see it -- see TestNestingCostStaysLinear, which
// compares the curve instead.
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
// # The direction Emit cannot see
//
// [Ledger] records one kind of defect: a valid document the library refuses, or
// reads as the wrong value. It cannot record the opposite -- an invalid
// document the library reads anyway -- and no amount of running it deeper will
// change that.
//
// The reason is structural. TestEveryEmittedDocumentIsValidYAML holds every
// generated document to the YAML 1.2 grammar, so being valid is a property of
// the corpus rather than a question asked of it, and the library accepting one
// is never news.
//
// [Mutate] is the generator for the other direction: it breaks a document, and
// makes no claim that the result is invalid. YAML is nearly total, so most ways
// of disturbing a document leave another good one -- about nine in ten. The
// recognizer is what makes that affordable, since one call sorts the mutants
// worth asking about from the ones already covered. What survives goes in
// [Lax], one entry per document rather than per shape, because the mutants that
// get that far are few enough to reduce and read.
//
// # Reading a wrongly-accepted finding
//
// Severity does not follow the direction. Accepting an invalid document is
// mostly harmless when the value is the obvious one, and worse than a rejection
// when it is not, because nothing downstream is in a position to notice. Both
// are in [Lax]: an anchor written against a flow collection loses the
// collection, and a numeric escape with non-hex digits produces some other
// character entirely.
//
// A finding here rests on the grammar's refusal being right, where a finding in
// [Ledger] only needs its acceptance to be. The recognizer has already been
// wrong in exactly that way once, refusing a compact collection written under a
// wider parent -- which this library's renderer emits and every other parser
// reads. So each entry names the production it breaks, and that production is
// what to check against the spec. The recognizer is not evidence for its own
// verdict.
//
// # What no grammar can see
//
// A grammar says what a document looks like and nothing more. An alias
// resolving to an anchor, the keys of a mapping being distinct, a tag having a
// meaning: none of these are syntax, so the recognizer admits documents that
// break all three. A mutation aimed at one of them is filtered out as valid --
// which is how the alias mutation that used to be in [Mutate] was found to be
// dead weight. Those classes need an oracle this package does not have.
package yamlgen
