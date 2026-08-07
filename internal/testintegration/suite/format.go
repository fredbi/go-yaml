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
const Format = 1

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
	// Vocabulary is every tag name this artifact uses.
	//
	// It is here so a consumer can fail loudly rather than quietly. A stance
	// that has never heard of a tag would otherwise treat it as absent and
	// score the document as though a question it does not understand were
	// settled.
	Vocabulary []string `json:"vocabulary"`
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
	// Origin traces the case back to what produced it.
	Origin Origin `json:"origin"`
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
