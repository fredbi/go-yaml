// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package suite is the on-disk form of a conformance corpus.
//
// # One file, two ways of consuming it
//
// An artifact is gzipped JSONL: a header object on the first line, then one
// case per line. That shape serves both audiences without forking. Released as
// a build artifact it is readable by anyone with a JSON reader and base64 --
// no Go, no toolkit, no oracle. Checked in and embedded, it is a corpus an
// ordinary Go test reads with one call.
//
// # What is in it, and what is deliberately not
//
// A case carries the bytes, the grammar's verdict, and the
// implementation-defined properties the document exhibits. It does **not**
// carry what any parser should do with it. That is a [stance.Table]'s job at
// replay time, so one artifact scores our parser, our parser under other
// options, and someone else's -- none of them privileged, and none of their
// opinions frozen into the file.
//
// What the header does carry is the contract those opinions are formed
// against: every tag, the stage its question arises at, and whether the
// specification settled it. A consumer can therefore derive an expectation from
// the file alone, with no Go and no import of ours -- which was the claim made
// for the released artifact from the start and was not true of format 1, where
// the stages lived only in our source.
//
// # Verdicts are not the only thing worth storing
//
// Some documents are valid under every reading and mean different things. A
// cycle is accepted by any conforming parser and is unrepresentable in a tree;
// "0777" is a valid document that denotes 511 under one schema and 777 under
// another. A corpus of accept-or-refuse scores both as passes and says nothing.
//
// So a case may also carry a [Meaning]: what the document denotes under the
// specification's own default reading, rendered as JSON. It is optional
// because most documents raise no such question, and it is one reading rather
// than all of them because the disagreements are tagged -- a consumer whose
// reading differs knows exactly which cases to skip.
//
// # Reproducibility is a property of the bytes
//
// The same seed and the same grammar must produce a byte-identical artifact, or
// regenerate-and-diff cannot tell a corpus that drifted from one that was
// edited. So nothing here reads a clock, no field is written from a map, and
// the gzip header carries no modification time. A corpus is a cache of an
// oracle's verdicts, and this is what makes it possible to prove the cache is
// still what the oracle would say.
package suite

// Format is the artifact layout version.
//
// It is bumped when a reader written against an older version would
// misunderstand a newer file -- not when a field is added, which readers are
// expected to ignore.
const Format = 3

// Header describes an artifact and is the first line of one.
type Header struct {
	// Format is the layout version, [Format] at the time of writing.
	Format int `json:"format"`
	// Grammar is the language, as the recognizer names it: "RFC 8259".
	Grammar string `json:"grammar"`
	// Digest identifies the grammar file the verdicts came from. A corpus
	// whose digest no longer matches the grammar is stale by definition.
	Digest string `json:"digest"`
	// Generator names what produced the documents, so that a change in the
	// generator is as visible as a change in the grammar.
	Generator string `json:"generator"`
	// Seed reproduces the documents exactly.
	Seed uint64 `json:"seed"`
	// Tier says how much was kept: "smoke" for the small embedded corpus,
	// "full" for the one that ships as a release artifact.
	Tier string `json:"tier"`
	// Cases is how many follow, so a truncated file is detectable.
	Cases int `json:"cases"`
	// Vocabulary is every tag this artifact uses, with the stage its question
	// arises at and whether the specification settles it.
	//
	// It is here so a consumer can fail loudly rather than quietly, and so it
	// can score at all. A stance that has never heard of a tag would treat it
	// as absent and score the document as though a question it does not
	// understand were settled; a stance that does not know a tag's stage cannot
	// tell a question it never reaches from one it is ducking.
	//
	// Sorted by tag, because the same corpus has to come out as the same bytes.
	Vocabulary []TagSpec `json:"vocabulary"`
}

// TagSpec is one tag and everything a consumer needs to reason about it.
//
// This is the part of the contract that used to live only in our source. A
// released artifact whose consumer has to guess which stage a tag belongs to is
// an artifact only we can score against, which defeats the purpose of releasing
// it.
type TagSpec struct {
	// Tag is the name, as it appears in a case.
	Tag string `json:"tag"`
	// Stage is where the question arises: "parse", "compose" or "construct".
	//
	// A consumer that stops earlier than this is not being asked. A lexer is
	// not wrong about numbers it never converts, and without this field it had
	// no way to say so except by declaring a position it does not hold.
	Stage string `json:"stage"`
	// Settled is what a conforming consumer must do, for the questions the
	// specification answered rather than left open: "accept", "reject", or
	// empty where the consumer chooses.
	//
	// A settled tag is not a matter of opinion and a consumer's stance does not
	// override it. It may be declined -- a parser that never resolves aliases
	// cannot check one -- which leaves the case unscored rather than passed.
	Settled string `json:"settled,omitempty"`
	// Because is what the specification says, and where, so that a consumer
	// disagreeing with a settled rule has something to argue with.
	Because string `json:"because,omitempty"`
}

// Case is one document and everything known about it that is not an opinion.
type Case struct {
	// Name identifies the case, and is stable across regenerations.
	Name string `json:"name"`
	// Src is the document. It is base64 in the file, because a conformance
	// corpus is full of bytes that are not text.
	Src []byte `json:"src"`
	// WellFormed is the grammar's verdict, after the losslessly normalizable
	// encoding properties have been normalized away.
	WellFormed bool `json:"wellFormed"`
	// Opaque says the oracle could not read the bytes under any policy it
	// knows, so its verdict is not evidence for a parser that would decode
	// them differently.
	Opaque bool `json:"opaque,omitempty"`
	// Tags are the implementation-defined properties the document exhibits.
	Tags []string `json:"tags,omitempty"`
	// VerdictAt is the furthest stage at which WellFormed is evidence, as a
	// stage name. Empty means parse, which is the least this can claim.
	//
	// # Why acceptance needs a bound and refusal does not
	//
	// Refusal propagates upward: a document that does not parse does not
	// compose or construct either, so a recorded refusal is evidence for every
	// consumer. Acceptance does not. A document can parse perfectly and fail to
	// compose, and the corpus is only entitled to say a consumer should read it
	// as far as it actually knows.
	//
	// It is not knowing that makes this necessary. A mutation that breaks an
	// anchor leaves a document the grammar accepts and a conforming parser must
	// refuse, and the corpus cannot tell which mutations did that -- guessing
	// would apply a settled rejection to documents that do not deserve one and
	// accuse in the other direction. So a mutant says "parse", and a consumer
	// that composes scores it on its refusals and leaves its acceptances alone.
	//
	// Empty defaulting to parse is deliberate. The permissive default is what
	// produced the defect this field exists for: an absent claim read as the
	// strongest one.
	VerdictAt string `json:"verdictAt,omitempty"`
	// Meaning is what the document denotes, where the corpus can say.
	//
	// Absent for most cases: a document that is refused denotes nothing, and a
	// document every reading agrees about raises no question worth storing an
	// answer to. Present for the families where the verdict is not the
	// interesting part.
	Meaning *Meaning `json:"meaning,omitempty"`
	// Origin traces the case back to what produced it.
	Origin Origin `json:"origin"`
}

// Meaning is what a document denotes, under one stated reading.
//
// One reading and not all of them, because the readings that differ are exactly
// what the tags name: a consumer implementing a different one looks at the tags
// and skips the cases it disagrees about, rather than needing the corpus to
// enumerate every implementation's answer.
type Meaning struct {
	// Under names the reading: "yaml-1.2-core", "rfc8259". It is the
	// specification's own default, not ours.
	Under string `json:"under"`
	// JSON is the value, rendered as JSON. Absent when Cyclic is set, because
	// then there is no rendering.
	JSON []byte `json:"json,omitempty"`
	// Cyclic says the meaning is a graph with a cycle and therefore has no JSON
	// form at all.
	//
	// It is worth a field rather than an omission, because it is checkable and
	// the check is the only one that finds the interesting failure. A consumer
	// that produced a value it can serialize to JSON from a document marked
	// cyclic has not represented the cycle -- it has quietly put something else
	// there, which is what this library does today.
	Cyclic bool `json:"cyclic,omitempty"`
}

// Origin is where a case came from, in enough detail to make it again.
type Origin struct {
	// Document is the index of the generated document this came from.
	Document int `json:"document"`
	// Mutation names how it was broken, empty for a document generated whole.
	Mutation string `json:"mutation,omitempty"`
	// Signature is the failure fingerprint that put this case in the corpus,
	// for a document the grammar refuses. It is what a minimizer grouped on,
	// recorded so that a later run can tell whether the grouping still holds.
	Signature string `json:"signature,omitempty"`
}
