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
		Name: "a tag before an anchor on a flow collection used as a key",
		Src:  "!!str &a [1]: v\n",
		Rule: "6.9.2 and 7.4: a node's properties may be written in either order, and a flow " +
			"collection may stand as a mapping key. The reference parser reads it and emits " +
			"+MAP +SEQ &a <tag:yaml.org,2002:str> =VAL :1 -SEQ =VAL :v -MAP.\n\n" +
			"Four things have to be true at once, re-measured on 2026-09-13 against " +
			"conformance-fixes at 0b321d7:\n" +
			"  - the tag comes first: `&a !!str [1]: v` reads;\n" +
			"  - the tag is a `!!` shorthand: `!foo &a [1]: v` reads, and `!!seq &a [1]: v` is " +
			"refused, so it is not about the tag naming the wrong type;\n" +
			"  - both properties are there: `!!str [1]: v` and `&a [1]: v` each read;\n" +
			"  - the node stands as a *key*: `!!str &a [1]` on its own reads, and " +
			"`!!str &a \"x\": v` reads, so it takes a flow collection in the key position. " +
			"`!!str &a {a: 1}: v` is refused too, so a flow mapping does it as well as a " +
			"sequence.\n\n" +
			"The tag-before-anchor entries this used to sit with -- yamlgen.Ledger's " +
			"parse/a-local-tag-before-an-anchor-does-not-type-its-scalar and the flow-value entry " +
			"below -- were both closed on 2026-09-13 and this one was not, so the shared cause is " +
			"the property order and not the fix.",
		Error: "[1:6] value is not allowed in this context",
	},
	{
		Name: "a comment between a tag on its own line and a plain scalar",
		Src:  "a:\n !\n # c\n 1\n",
		Rule: "6.9.1 and 8.2.1: a node's properties may be written on a line of their own, and " +
			"s-l-comments after them may hold comment lines. Without the comment, `a:` over ` !` " +
			"over ` 1` reads here; with it the parse stops. `!!str` in the same place reads with " +
			"the comment, and so does a flow collection under it -- so it is a local or " +
			"non-specific tag over a plain scalar and nothing else.\n\n" +
			"The same shape refuses at a sequence entry and at the document root. libfyaml 1.0.0b1 " +
			"reads {\"a\": \"1\"} and the reference parser emits =VAL <!> :1.\n\n" +
			"Likely the same assumption as the departure \"a local tag on an empty value, with the " +
			"mapping carrying on\": a tag that names no known type makes the parser expect a block " +
			"collection under it.",
		Error: "[4:2] value is not allowed in this context",
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
			"The reference parser reads both. libfyaml 1.0.0b1 refuses all three, `{[a, b]: 1}` " +
			"included, but that is its loader declining a sequence as a mapping key rather than a " +
			"statement about the syntax.",
		Error: "[2:3] map key definition includes an implicit line break",
	},
	{
		Name: "a version directive over a root scalar under an unknown secondary tag",
		Src:  "%YAML 1.1\n---\n!!nulll Null\n",
		Rule: "6.8.1 and 6.9.1: a \"%YAML\" directive states a version, and a tag names a type. " +
			"Whether tag:yaml.org,2002:nulll names anything is a question for resolution, which is " +
			"the application's, and the parse has no business refusing it -- least of all only under " +
			"a directive.\n\n" +
			"The same document without the directive parses. So do `!!str Null`, `!foo Null`, " +
			"`&a Null` and a bare `Null` under the directive, and so does `!!nulll x`. It takes all " +
			"three: the directive, an unknown secondary tag, and content that resolves.\n\n" +
			"The version does not matter -- `%YAML 1.2` refuses it too. Sits beside " +
			"yamlgen.Ledger's parse/a-version-directive-resolves-the-root-block-scalar-it-opens, " +
			"which is the same directive over a root block scalar. Found on 2026-09-07 by a mutation " +
			"of a `!!null` tag.\n\n" +
			"libfyaml 1.0.0b1 reads it as null and the reference parser passes it.",
		Error: "[3:8] value is not allowed in this context",
	},
	{
		Name: "a secondary tag on its own line over a block scalar",
		Src:  "!!null\n>\n",
		Rule: "6.9.1 and 8.1: a node's properties may be written on a line of their own, and the node " +
			"under them may be a block scalar. `!!null` over `>` is a folded scalar carrying the null " +
			"tag, and the parse stops with `value is not allowed in this context`.\n\n" +
			"The tag decides, and the other way round from the comment entry above: `!foo` over `>-` " +
			"over ` x` **reads**, where the `!!` shorthand does not. So is the line break: " +
			"`!!null >` on one line reads.\n\n" +
			"libfyaml 1.0.0b1 reads it as null and the reference parser passes it. Found on " +
			"2026-09-07 by a mutation that put a %TAG handle over the same shape.",
		Error: "[2:1] value is not allowed in this context",
	},
	{
		Name: "a secondary tag before an anchor on an empty flow value",
		Src:  "{a: !!str &x}\n",
		Rule: "6.9.2 and 7.4: a node may carry a tag and an anchor in either order, and a flow " +
			"mapping entry may have an empty value. The parse loses the collection's end: " +
			"`{a: !!str &x}` is refused with `could not find flow mapping end token '}'` and " +
			"`{a: !!str &x, b: 1}` with `',' or '}' must be specified` at the comma. " +
			"`[!!str &x]` goes the same way.\n\n" +
			"Three things narrow it. The order: `{a: &x !!str}` reads, and gives \"\". The tag: " +
			"`{a: !foo &x}` reads, so it is a `!!` shorthand or a verbatim secondary tag and not a " +
			"local one. The context: `a: !!str &x` in block reads. Sibling of the first entry in " +
			"this list, which is the same two properties in the same order on a flow sequence used " +
			"as a key.\n\n" +
			"The reference parser emits +MAP {} =VAL :a =VAL &x <tag:yaml.org,2002:str> : -MAP, " +
			"grammar.NewRecognizer accepts it, and internal/refparser accepts it -- which is how it " +
			"was found, by TestLabParserMatchesProduction on 2026-09-13 over a mutant that put an " +
			"anchor with no node after a verbatim `!!bool`. libfyaml 1.0.0b1 and " +
			"go.yaml.in/yaml/v3 v3.0.5 refuse the tagged empty node at construction, which is a " +
			"question about resolution rather than about the syntax.",
		Error: "[1:1] could not find flow mapping end token '}'",
	},
}
