// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar_test

import (
	"testing"
	"time"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/testintegration/grammar"
)

// TestCompiles asserts the whole grammar file turned into closures, rather than
// some prefix of it that happened not to hit an unimplemented form.
func TestCompiles(t *testing.T) {
	assert.Equal(t, 211, grammar.Rules())
}

// flowCases are documents that are exactly one flow node, and documents that
// are not. They are the spike's correctness gate: not a conformance claim, just
// enough to show the engine agrees with the spec where we can check it by hand.
var flowCases = []struct {
	name  string
	src   string
	valid bool
}{
	// Flow collections.
	{"empty sequence", `[]`, true},
	{"empty mapping", `{}`, true},
	{"plain entries", `[a, b, c]`, true},
	{"trailing comma", `[a, b, c, ]`, true},
	{"nested", `[a, [b, {c: d}], e]`, true},
	{"mapping pair", `{a: 1, b: 2}`, true},
	{"spaces around", `[ a , b ]`, true},
	{"multi line", "[\n  a,\n  b\n ]", true},
	{"unclosed sequence", `[a, b`, false},
	{"mismatched close", `[a, b}`, false},
	{"stray close", `]`, false},
	{"double comma", `[a,, b]`, false},

	// Scalar styles.
	{"plain scalar", `hello world`, true},
	{"single quoted", `'it''s here'`, true},
	{"double quoted", `"a \n b"`, true},
	{"double quoted escape", `"☺"`, true},
	{"bad escape", `"\q"`, false},
	{"unterminated single", `'abc`, false},
	{"unterminated double", `"abc`, false},

	// Plain scalars are where the context sensitivity lives.
	{"colon in plain out", `a:b`, true},
	{"hash after text", `a#b`, true},
	{"comment starts", `a #b`, false},
	{"plain in flow with colon", `[a:b]`, true},
	{"plain cannot start with comma in flow", `[,a]`, false},

	// Node properties.
	{"anchor", `&a [1]`, true},
	{"tag", `!!str x`, true},
	{"tag and anchor", `!!str &a x`, true},
	{"alias", `*a`, true},
	{"alias cannot take content", `*a b`, false},
	{"empty node with tag", `!!str`, true},
}

func TestFlowNode(t *testing.T) {
	for _, tc := range flowCases {
		t.Run(tc.name, func(t *testing.T) {
			got := grammar.FlowNode([]byte(tc.src))
			assert.Equalf(t, tc.valid, got.OK, "%q: %s", tc.src, got)
		})
	}
}

// TestMemoAgrees checks that memoization is an optimization and not a change of
// behavior. A parameterized PEG makes this worth asserting: if the memo key
// missed one of the four variables, this is where it would show.
func TestMemoAgrees(t *testing.T) {
	for _, tc := range flowCases {
		t.Run(tc.name, func(t *testing.T) {
			memo := grammar.Match("ns-flow-node", []byte(tc.src), 0, "flow-out")
			plain := grammar.MatchNoMemo("ns-flow-node", []byte(tc.src), 0, "flow-out")
			require.Equalf(t, memo.OK, plain.OK, "%q: memo=%s plain=%s", tc.src, memo, plain)
		})
	}
}

// throughputCases are the documents the spike measures against: small, but
// shaped like the ones a generator would produce.
var throughputCases = []string{
	`[a, b, c]`,
	`{name: value, other: thing}`,
	`[a, [b, {c: d}], e, [f, [g, [h]]]]`,
	`"a double quoted scalar with \t escapes and spaces"`,
	`[&anchor !!str value, *anchor, {key: [1, 2, 3]}]`,
	`a fairly long plain scalar that keeps going for a while yet`,
}

// TestThroughput is the measurement the spike was built for: how long a
// hundred thousand oracle calls would take, and what memoization is worth.
//
// It reports rather than asserts. The number is the input to a decision, not a
// property to defend.
func TestThroughput(t *testing.T) {
	const runs = 2000

	measure := func(match func(string) grammar.Result) (time.Duration, int64, int64) {
		start := time.Now()
		var steps, hits int64
		for range runs {
			for _, src := range throughputCases {
				r := match(src)
				steps += r.Steps
				hits += r.Hits
			}
		}

		return time.Since(start), steps, hits
	}

	fresh, steps, hits := measure(func(s string) grammar.Result {
		return grammar.Match("ns-flow-node", []byte(s), 0, "flow-out")
	})

	rec := grammar.NewRecognizer(64)
	reused, _, _ := measure(func(s string) grammar.Result {
		return rec.FlowNode([]byte(s))
	})

	docs := int64(runs * len(throughputCases))

	report := func(name string, total time.Duration) {
		per := total / time.Duration(docs)
		t.Logf("%-18s %8s/doc   100k documents in %s", name, per, (per * 100_000).Truncate(time.Millisecond))
	}

	t.Logf("%d steps/doc, %.0f%% of them answered by the memo table", steps/docs, 100*float64(hits)/float64(steps))
	report("table per call", fresh)
	report("table reused", reused)
	t.Logf("reusing the table is worth %.1fx", float64(fresh)/float64(reused))
}

func BenchmarkFlowNode(b *testing.B) {
	src := []byte(`[&anchor !!str value, *anchor, {key: [1, 2, 3]}]`)

	b.ReportAllocs()
	for b.Loop() {
		grammar.FlowNode(src)
	}
}
