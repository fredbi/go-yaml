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
// That is what every entry here had in common: an empty node standing where the
// generator only ever puts a full one.
//
// ✅ Pair.Key became a Value on 2026-09-08 and the generator reaches that class
// now, so a document of this shape is a [Divergence] rather than a Strictness --
// see parse/a-mapping-key-written-empty-is-refused, which the change opened on
// its first deep run. What is left here is the narrower case: a valid document
// this package cannot produce at all.
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
// The six entries it held before 2026-09-11 were all read by the time the empty
// node was carried through the scanner, the token grouping and the flow
// parsers, and each left a test in parser/ and scanner/ behind it.
//
// Deliberately no expected value on an entry. An empty key in a Go map is a
// question about this library's mapping model rather than about YAML, and a
// guessed expectation pinned here would be believed. Whoever adds an entry
// should settle the value against another parser and record it then.
var Strict = []Strictness{
	{
		Name: "a collection written as a flow entry's key alone",
		Src:  "{{\"\": 0}}\n",
		Rule: "7.4.2: a flow mapping entry may be a key with no value, and its key may be any flow " +
			"node -- a flow mapping or sequence included. `{{\"\": 0}}` is one entry whose key is " +
			"the mapping {\"\": 0} and whose value is empty.\n\n" +
			"`{{a: 0}: v}` -- the same key with a value -- parses here and reads " +
			"{map[a:0]: v}, so it is the missing value and not the collection key.\n\n" +
			"⚠️ **libfyaml cannot answer this one**, which is worth stating rather than counting it " +
			"as corroboration. It refuses all three of `{{\"\": 0}}`, `{[a]}` and `{{a: 0}: v}` with " +
			"a Python traceback -- the binding cannot hash a collection as a dict key -- and the " +
			"last of those is a document this library reads correctly. A traceback after a parse is " +
			"the construction refusal this register warns about, not a verdict on the syntax.\n\n" +
			"So two sources answer: grammar.NewRecognizer accepts it and the reference parser passes " +
			"it, both generated from the specification's grammar. This library is the only one " +
			"refusing it at a position.\n\n" +
			"Found on 2026-09-07, when Keys began drawing a collection: Style.FlowEmpty writes an " +
			"entry with no value, and a collection key under it is this document.",
		Error: "[1:2] could not find flow map content",
	},
	{
		Name: "a flow mapping key spanning two lines",
		Src:  "{[a\nb]: 1}\n",
		Rule: "7.4.2: a flow mapping's key is under neither of the implicit-key restrictions -- it " +
			"may span lines, and a break before the ':' is ordinary separation. Only a flow " +
			"*sequence* entry has to keep its key and its ':' on one line, which is 7.4.1 and which " +
			"parser/token.go enforces for both.\n\n" +
			"`{[a, b]: 1}` parses here, so it is the break and nothing else. Written the long way, " +
			"`{? [a\nb]\n: 1}`, it is refused too and with a different message -- " +
			"`',' or ']' must be specified` -- so there are two paths into it.\n\n" +
			"⚠️ **This entry is a ruling and not an ordinary refusal.** The two grammar-derived " +
			"oracles accept the document and every hand-written implementation refuses it, this " +
			"library included. Re-measured on 2026-09-13 against all four sources:\n" +
			"  - the reference parser passes it and grammar.NewRecognizer accepts it -- two " +
			"generations of the specification's own grammar, by different toolchains;\n" +
			"  - libfyaml 1.0.0b1 refuses it in its C parser: `missing comma in flow mapping` at " +
			"2:3;\n" +
			"  - go.yaml.in/yaml/v3 v3.0.5 refuses it too: `did not find expected ',' or '}'`.\n\n" +
			"Those last two are parse errors and not the binding declining to hold a collection as " +
			"a key, which is the confound this register warns about elsewhere. The tell is the " +
			"error itself: `{[a, b]: 1}` gives libfyaml a Python `TypeError: unhashable type` and " +
			"gives yaml/v3 `invalid map key`, both after a successful parse, while the document " +
			"here stops in the parser at a position. And `[[a\nb]]` -- the same break inside a " +
			"flow sequence with no key in sight -- is read by both.\n\n" +
			"So nothing corroborates the claim except the grammar. Three implementations reading " +
			"7.4.2 the other way is evidence about 7.4.2, not about this library, and the entry is " +
			"kept for the measurement rather than as an accusation. Whoever settles it should " +
			"decide whether the published grammar is lax here before anyone changes the parser.",
		Error: "[2:3] map key definition includes an implicit line break",
	},
}
