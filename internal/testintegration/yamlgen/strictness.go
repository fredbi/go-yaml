// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

// Strictness is a document that is YAML 1.2 and that this library refuses.
//
// It is the mirror of [Laxity], and the third of the three lists here because
// it is found a third way. A [Divergence] names a shape, since the generator
// draws a different document every run. A Laxity names a document that survived
// the mutation hunt. A Strictness names a document nothing in this package
// produces at all -- somebody met it while fixing something else, and without
// this list the harness would go on not watching it.
//
// That is what every entry below has in common, and it is the reason to keep
// them: four of the six are an empty node standing where the generator only
// ever puts a full one. Widening the generator to reach them wants Pair.Key to
// become a Value, which is a larger change than pinning them here.
type Strictness struct {
	// Name is short and stable, so a count can be reported against it.
	Name string
	// Src is the document.
	Src string
	// Rule is the production that makes it valid.
	//
	// The claim here is the grammar's acceptance, which is the weaker of the
	// two directions -- a Laxity needs its refusal to be right. Even so the
	// production is named, because the recognizer is not evidence for its own
	// verdict.
	Rule string
	// Error is the first line of what the library says today, so that a changed
	// message is visible as a change rather than passing for a fix.
	//
	// The first line only: the rest is the offending source with a caret under
	// it, which pins the document again to no purpose and would make every
	// entry brittle against a change in how errors are drawn.
	Error string
}

// Strict records valid documents the library refuses.
//
// Deliberately no expected value. Nobody has ruled on what four of these should
// decode to -- an empty key in a Go map is a question about this library's
// mapping model, not about YAML -- and a guessed expectation pinned here would
// be believed. Whoever fixes one should settle the value against another
// parser and record it then.
var Strict = []Strictness{
	{
		Name: "an-anchor-alone-in-a-flow-mapping",
		Src:  "{&a}\n",
		Rule: "ns-flow-map-implicit-entry via c-ns-flow-map-empty-key-entry: an " +
			"entry may be a key with no value, and ns-flow-node admits " +
			"c-ns-properties followed by e-scalar, so &a names the empty node",
		Error: "[1:1] could not find flow mapping end token '}'",
	},
	{
		Name: "an-explicit-key-with-nothing-in-it",
		Src:  "? \n",
		Rule: "c-l-block-map-explicit-entry: the key is c-l-block-map-explicit-key " +
			"followed by l-block-map-explicit-value or e-node, and the key itself " +
			"is s-l+block-indented which admits e-node",
		Error: "[1:1] undefined map key",
	},
	{
		Name: "an-explicit-key-with-nothing-in-it-and-a-value",
		Src:  "?\n: v\n",
		Rule: "c-l-block-map-explicit-entry: the empty key is e-node and the value " +
			"follows on its own line under l-block-map-explicit-value",
		Error: "[2:1] value is not allowed in this context",
	},
	{
		Name: "a-pair-with-neither-side-in-a-flow-sequence",
		Src:  "[:]\n",
		Rule: "ns-flow-seq-entries admits ns-flow-pair, whose ns-flow-pair-entry " +
			"reaches c-ns-flow-map-empty-key-entry: a single pair with an empty " +
			"key and an empty value",
		Error: "[1:3] could not find '[' character corresponding to ']'",
	},
	{
		Name: "a-non-specific-tag-on-nothing",
		Src:  "[!]\n",
		Rule: "ns-flow-node admits c-ns-properties followed by e-scalar, and " +
			"c-ns-tag-property admits the bare \"!\" as the non-specific tag",
		Error: "[1:1] sequence end token ']' not found",
	},
	{
		Name: "a-byte-order-mark-before-a-later-document",
		Src:  "a: 1\n...\n\ufeff---\nb: 2\n",
		Rule: "l-document-prefix ::= c-byte-order-mark? l-comment*, and " +
			"l-yaml-stream puts a run of them after every l-document-suffix. A " +
			"mark opening the stream is dropped; one opening a later document " +
			"still reaches the parser as content",
		Error: "[4:1] value is not allowed in this context",
	},
}
