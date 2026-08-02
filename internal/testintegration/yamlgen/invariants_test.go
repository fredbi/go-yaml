// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"flag"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"pgregory.net/rapid"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"
	"github.com/go-openapi/go-yaml/parser"
)

// The three invariants a YAML library should hold, stated without excuses.
//
// The properties next door tolerate the ledger, because a suite that is red for
// known reasons stops being read. These do not tolerate anything: they say what
// the parser and renderer ought to do, and they fail today. That is what makes
// them useful to somebody fixing a defect rather than to somebody guarding
// against a regression.
//
// Run them with:
//
//	go test -run TestInvariant ./internal/testintegration/yamlgen/ -args -yamlgen.invariants -rapid.checks=10000
//
// The flags go after -args because go test validates the ones it does not
// recognize against the package in the current directory, which is not this
// one. From this directory `go test -yamlgen.invariants .` works as written.
//
// Ten thousand checks rather than the default hundred: the rarer shapes are
// drawn a few times in a hundred thousand, so a short run reports success it
// has not earned.
//
// Every failure arrives reduced to the smallest document that still shows it,
// with a test case to paste. When the last one passes, the ledger next door is
// empty and these can simply replace the tolerant versions.
var runInvariants = flag.Bool("yamlgen.invariants", false,
	"assert the invariants the parser should hold, which currently fail")

func requireInvariantMode(t *testing.T) {
	t.Helper()

	if !*runInvariants {
		t.Skip("known to fail: pass -yamlgen.invariants to assert the invariants the parser should hold")
	}
}

// TestInvariantEveryPresentationReadsAsTheValue: writing one value down in any
// style and reading it back gives that value.
//
// This is the invariant that makes YAML's presentation choices a matter of
// taste rather than of meaning. It has no reduction pass: the expected value
// comes from the generator, so shrinking the document would change what it is
// supposed to say. rapid shrinks the value and the style instead.
func TestInvariantEveryPresentationReadsAsTheValue(t *testing.T) {
	requireInvariantMode(t)

	rapid.Check(t, func(rt *rapid.T) {
		value := yamlgen.Values().Draw(rt, "value")
		style := yamlgen.Styles().Draw(rt, "style")

		src := yamlgen.Emit(value, style)
		expected := value.Decoded()

		var got any
		if err := yaml.Unmarshal([]byte(src), &got); err != nil {
			rt.Fatalf("style %s: the document does not read back at all:\n%s\nerror: %v",
				style, indent(src), err)
		}

		if !assert.ObjectsAreEqual(expected, got) {
			rt.Fatalf("style %s: read back as a different value:\n%s\nexpected: %#v\ngot:      %#v\n\nreproducer:\n%s",
				style, indent(src), expected, got,
				indent(yamlgen.Reproducer("PresentationInvariance", src, expected, got)))
		}
	})
}

// TestInvariantRenderingPreservesTheValue: reading a document and writing it
// back does not change what it means.
//
// This is the one the library's reason for existing rests on. A tool that
// rewrites a file to change one field must not quietly change another.
func TestInvariantRenderingPreservesTheValue(t *testing.T) {
	requireInvariantMode(t)

	rapid.Check(t, func(rt *rapid.T) {
		value := yamlgen.Values().Draw(rt, "value")
		style := yamlgen.Styles().Draw(rt, "style")
		src := yamlgen.Emit(value, style)

		if _, err := parser.ParseBytes([]byte(src), parser.ParseComments); err != nil {
			return
		}

		if renderChangesValue([]byte(src)) {
			rt.Fatalf("style %s: rendering changed the value.\n%s",
				style, reduced("RenderChangedTheValue", src, renderChangesValue))
		}
	})
}

// TestInvariantRenderingSettles: rendering twice gives the same text twice.
//
// A renderer that does not settle rewrites the file a little differently every
// time it is used, so every save produces a diff whether or not anything
// changed.
func TestInvariantRenderingSettles(t *testing.T) {
	requireInvariantMode(t)

	rapid.Check(t, func(rt *rapid.T) {
		value := yamlgen.Values().Draw(rt, "value")
		style := yamlgen.Styles().Draw(rt, "style")
		src := yamlgen.Emit(value, style)

		if _, err := parser.ParseBytes([]byte(src), parser.ParseComments); err != nil {
			return
		}

		if renderDoesNotSettle([]byte(src)) {
			rt.Fatalf("style %s: rendering does not settle.\n%s",
				style, reduced("RenderDoesNotSettle", src, renderDoesNotSettle))
		}
	})
}

// TestInvariantsAreStillOutstanding reports which invariants fail today,
// without failing itself.
//
// It runs in an ordinary test run, so the state of the work is visible without
// anyone having to remember the flag. Each named document is one the generator
// found and reduced; they are the same defects the ledger records, in the form
// a fixer can act on.
func TestInvariantsAreStillOutstanding(t *testing.T) {
	outstanding := []struct {
		invariant string
		src       string
		fails     func([]byte) bool
	}{
		{
			invariant: "reading any presentation gives the value",
			src:       "|-\n  trailing \n",
			fails: func(b []byte) bool {
				var got any
				return yaml.Unmarshal(b, &got) != nil || got != "trailing "
			},
		},
		{
			invariant: "rendering preserves the value",
			src:       "k: |+\n  trail\n\n",
			fails:     renderChangesValue,
		},
		{
			invariant: "rendering settles after one cycle",
			src:       "-\n# c\n - x\n",
			fails:     renderDoesNotSettle,
		},
		{
			invariant: "rendering keeps every comment",
			src:       "# c1\n-  # c2\n",
			fails:     commentsAreLost,
		},
	}

	var open int
	for _, o := range outstanding {
		if o.fails([]byte(o.src)) {
			open++
			t.Logf("still failing -- %s: %q", o.invariant, o.src)

			continue
		}
		t.Logf("NOW HOLDS  -- %s: %q -- drop it from this list and from the ledger",
			o.invariant, o.src)
	}

	t.Logf("%d of %d known documents still violate an invariant; run with -yamlgen.invariants to search for more",
		open, len(outstanding))
}
