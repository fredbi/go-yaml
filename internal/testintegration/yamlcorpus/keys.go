// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus

import "github.com/go-openapi/go-yaml/internal/testintegration/stance"

// Unique keys: the rule that needs resolution rather than spelling.
//
// # Why a grammar cannot state it
//
// "It is an error for two equal keys to appear in the same mapping node"
// (3.2.1.1). A production cannot hold the keys it has already seen, so the
// grammar accepts every document here, the invalid ones included -- the same
// shape as an undefined alias, and it gets rules for the same reason.
//
// # Why *equal* is the hard word
//
// Two keys are equal when they resolve to the same node, not when they are
// written the same way. Three consequences, and every one of them is a document
// a checker comparing spellings gets wrong:
//
//   - "a" and a are written differently and are the same key.
//   - 1 and !!int 1 are written differently and are the same key.
//   - 1 and "1" are written almost identically and are *different* keys, one an
//     integer and one a string. A checker comparing text refuses a valid
//     document here, which is how this family catches a checker rather than
//     merely exercising it.
//   - An alias resolves to whatever it names, so a key can collide with one
//     written out in full somewhere else entirely.
//
// The valid case is carried deliberately. A family of violations alone would be
// passed by a parser that refused every mapping with two similar-looking keys,
// and that parser would be badly wrong.
const (
	// TagDuplicateKey is two keys with the same spelling.
	TagDuplicateKey stance.Tag = "key/duplicate"
	// TagDuplicateAfterResolution is two keys that are equal only once they are
	// resolved: quoting removed, tags applied, aliases followed.
	TagDuplicateAfterResolution stance.Tag = "key/duplicate-after-resolution"
	// TagDistinctAfterResolution is two keys that look nearly alike and are not
	// equal, because resolution gives them different types.
	//
	// The control, and the one that fails a text-comparing checker.
	TagDistinctAfterResolution stance.Tag = "key/distinct-after-resolution"
	// TagKeyIntegralFloat is a mapping key that resolves to a float whose
	// value is a whole number: "1.0", "1e3", "-0.0".
	//
	// Its own tag because a float key is a key like any other everywhere else,
	// and this is the one shape where naming the key loses which type it was.
	TagKeyIntegralFloat stance.Tag = "key/integral-float"
	// TagKeyNotAString is a mapping key that resolves to something other than
	// a string: a number, a boolean, a null.
	//
	// The question is what a decoder hands back for one. Keeping the type is a
	// position -- go.yaml.in/yaml/v3 reads "1.0: a" as map[any]any keyed by
	// float64(1) -- and stringifying it is another, which is what JSON needs
	// and what codec.ToJSON has to do. A library may reasonably offer both;
	// what it cannot do is offer an option for one and give the other either
	// way.
	TagKeyNotAString stance.Tag = "key/not-a-string"
)

// KeyRules is what the specification settles about them.
//
// Enforcement is another matter and deliberately not settled here: libfyaml
// reads every duplicate below and keeps the last, so a parser declining the
// check is in respectable company. Declining leaves a document unscored, which
// is the honest answer -- what it may not do is claim a pass.
func KeyRules() stance.Rules {
	return stance.Rules{
		{
			Tag:     TagDuplicateKey,
			Because: "3.2.1.1: it is an error for two equal keys to appear in the same mapping node",
			Then:    stance.Reject,
		},
		{
			Tag:     TagDuplicateAfterResolution,
			Because: "3.2.1.1: keys are equal when they resolve to the same node, however they are written",
			Then:    stance.Reject,
		},
		{
			Tag:     TagDistinctAfterResolution,
			Because: "3.2.1.1: keys resolving to different nodes are different keys, however alike they look",
			Then:    stance.Accept,
		},
	}
}

// KeyVocabulary places them.
//
// A spelling can be compared while parsing; equality after resolution cannot,
// since resolution is what composing does. That the two sit at different stages
// is not bookkeeping -- it is the whole difference between the check a parser
// can do and the check the specification asks for.
func KeyVocabulary() stance.Vocabulary {
	return stance.Vocabulary{
		TagDuplicateKey:             stance.Parse,
		TagDuplicateAfterResolution: stance.Compose,
		TagDistinctAfterResolution:  stance.Compose,
		// Naming a key happens when the representation becomes a native value,
		// so which text a float ends up under is a construction question.
		TagKeyIntegralFloat: stance.Construct,
		TagKeyNotAString:    stance.Construct,
	}
}

// KeyShapes are the documents.
func KeyShapes() []stance.Shape {
	return []stance.Shape{
		{
			Name:   "the same key twice",
			Src:    []byte("a: 1\na: 2\n"),
			Intent: []stance.Tag{TagDuplicateKey},
		},
		{
			Name:   "a key that is a boolean",
			Src:    []byte("true: a\n"),
			Intent: []stance.Tag{TagKeyNotAString},
		},
		{
			Name:   "a key that is null",
			Src:    []byte("~: a\n"),
			Intent: []stance.Tag{TagKeyNotAString},
		},
		{
			Name:   "a key tagged !!float",
			Src:    []byte("!!float 226.0: x\n"),
			Intent: []stance.Tag{TagKeyIntegralFloat},
		},
		{
			Name:   "a key tagged !!timestamp",
			Src:    []byte("!!timestamp 2001-12-14: x\n"),
			Intent: []stance.Tag{TagKeyNotAString},
		},
		{
			Name:   "a key tagged !!binary",
			Src:    []byte("!!binary aGVsbG8=: x\n"),
			Intent: []stance.Tag{TagKeyNotAString},
		},
		{
			Name:   "a key that is a float with a whole value",
			Src:    []byte("1.0: a\n"),
			Intent: []stance.Tag{TagKeyIntegralFloat},
		},
		{
			Name:   "a key written as a float in exponent form",
			Src:    []byte("1e3: a\n"),
			Intent: []stance.Tag{TagKeyIntegralFloat},
		},
		{
			// The consequence rather than the spelling: two keys of different
			// types, one of which loses its type when it is named.
			Name:   "a whole-valued float key beside the integer of the same value",
			Src:    []byte("1.0: a\n1: b\n"),
			Intent: []stance.Tag{TagKeyIntegralFloat},
		},
		{
			Name:   "the same key twice inside a flow mapping",
			Src:    []byte("{a: 1, a: 2}\n"),
			Intent: []stance.Tag{TagDuplicateKey},
		},
		{
			// Found by the generator on 2026-09-03, once Style.FlowEmpty
			// started writing a flow entry as a key with no colon. It was read
			// while "{a: 1, a: 2}" above was refused: the duplicate-key check
			// saw only the keys that came with a ':'. Both are refused now.
			Name:   "the same key twice, one of them written as a key alone",
			Src:    []byte("{a, a: 1}\n"),
			Intent: []stance.Tag{TagDuplicateKey},
		},
		{
			Name:   "the same key twice in a nested mapping",
			Src:    []byte("outer:\n  a: 1\n  a: 2\n"),
			Intent: []stance.Tag{TagDuplicateKey},
		},
		{
			Name:   "the same key quoted and plain",
			Src:    []byte("\"a\": 1\na: 2\n"),
			Intent: []stance.Tag{TagDuplicateAfterResolution},
		},
		{
			Name:   "the same key tagged and plain",
			Src:    []byte("1: x\n!!int 1: y\n"),
			Intent: []stance.Tag{TagDuplicateAfterResolution},
		},
		{
			Name:   "a key colliding with one an alias resolves to",
			Src:    []byte("k: &a n\n*a : 1\nn: 2\n"),
			Intent: []stance.Tag{TagDuplicateAfterResolution, TagAliasAsKey},
		},
		{
			// Valid, and the shape that catches a checker comparing spellings.
			Name:   "two keys alike in text and different once resolved",
			Src:    []byte("1: x\n\"1\": y\n"),
			Intent: []stance.Tag{TagDistinctAfterResolution},
		},
	}
}
