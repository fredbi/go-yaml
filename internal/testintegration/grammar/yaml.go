// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import (
	_ "embed"
)

//go:embed testdata/yaml-spec-1.2.json
var yamlSpec []byte

// YAML is the YAML 1.2 grammar, compiled once from the productions the spec
// publishes as a data structure, with the prose-only rules patched in.
var YAML = MustCompile("YAML 1.2", yamlSpec, yamlPatches()...)

// yamlPatches are the places where compiling the published grammar faithfully
// would not compile YAML faithfully.
//
// Each one is a rule the spec states in prose and never puts in the grammar
// file. They are asserted rather than attempted: see [Patch].
func yamlPatches() []Patch {
	return []Patch{
		{
			Rule:    "s-l+block-indented",
			Because: "the extra indentation a compact collection detects is not written down",
			Apply:   patchBlockIndented,
		},
		{
			Rule:    "c-b-block-header",
			Because: "the two block header indicators may be written in either order",
			Apply:   patchBlockHeader,
		},
		{
			Rule:    "c-indentation-indicator",
			Because: "zero is not a legal indentation indicator",
			Apply:   patchIndentationIndicator,
		},
	}
}

// FlowNode reports whether src is exactly one YAML 1.2 flow node.
//
// Narrower than Stream, and worth keeping separate: a failure against one node
// says the fault is in that node, where the same failure against a whole stream
// could be anywhere above it.
func FlowNode(src []byte) Result {
	return YAML.Match("ns-flow-node", src, 0, "flow-out")
}

// Stream reports whether src is a valid YAML 1.2 stream.
//
// This is the question the conformance harness asks: not whether some fragment
// is well formed, but whether a document the generator wrote is one the library
// is obliged to read.
func Stream(src []byte) Result {
	return YAML.Match("l-yaml-stream", src, -1, "block-in")
}

// Match reports whether src is exactly one instance of the named YAML
// production, entered at indentation n in context c.
func Match(rule string, src []byte, n int, c string) Result {
	return YAML.Match(rule, src, n, c)
}

// MatchNoMemo is Match with the memo table disabled, so that a test can check
// memoization changed nothing, and report what it is worth rather than assume
// it.
func MatchNoMemo(rule string, src []byte, n int, c string) Result {
	return YAML.MatchNoMemo(rule, src, n, c)
}

// Rules reports how many productions the YAML grammar defines.
func Rules() int { return YAML.Rules() }

// NewRecognizer returns a reusable YAML oracle whose memo table is sized for
// documents of about hint bytes.
func NewRecognizer(hint int) *Recognizer { return YAML.Recognizer(hint) }

// FlowNode reports whether src is exactly one YAML 1.2 flow node.
func (r *Recognizer) FlowNode(src []byte) Result {
	return r.Match("ns-flow-node", src, 0, "flow-out")
}

// Stream reports whether src is a valid YAML 1.2 stream.
func (r *Recognizer) Stream(src []byte) Result {
	return r.Match("l-yaml-stream", src, -1, "block-in")
}
