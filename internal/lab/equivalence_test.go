// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package lab_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/internal/corpus"
	"github.com/go-openapi/go-yaml/internal/fuzzseeds"
	"github.com/go-openapi/go-yaml/internal/refparser"
	"github.com/go-openapi/go-yaml/internal/yamltestsuite"
	"github.com/go-openapi/go-yaml/parser"
)

// TestLabParserMatchesProduction is the gate every lab candidate clears before
// it is measured, and the alarm that fires when the copy has drifted.
//
// A candidate that refuses a document production accepts, accepts one it
// refuses, or builds a tree that differs anywhere -- node type, value, or the
// position of the token a node was built from -- is not a faster parser. It is
// a different one, and the difference is the finding.
//
// A difference that is a fix is named in intendedDivergence and skipped there.
// internal/refparser is frozen, so a defect corrected in the shipped parser
// shows up here as a disagreement and stays one: the entry is what says which
// of the two is right.
func TestLabParserMatchesProduction(t *testing.T) {
	t.Parallel()

	for _, mode := range []struct {
		name string
		mode refparser.Mode
	}{
		{"without comments", 0},
		{"with comments", refparser.ParseComments},
	} {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()

			for _, src := range sources(t) {
				t.Run(src.name, func(t *testing.T) {
					assertSameParse(t, src.text, mode.mode)
				})
			}
		})
	}
}

// divergesOnPurpose reports whether a refusal the frozen parser did not make is
// one the shipped parser makes on purpose.
//
// Two rules so far.
//
// A flow entry written as a key alone is an entry like any other, so its key
// counts when the mapping is checked for duplicates: "{a, a: 1}" repeats a key
// as much as "{a: 1, a: 2}" does. refparser recorded only the keys that came
// with a ':', so it read the first and refused the second. 3.2.1.1 says both
// are errors.
//
// A flow mapping entry that already holds a value cannot take a second ':':
// after the "[]" in "{a: []:}" the mapping must continue with ',' or close.
// refparser built an empty entry from the trailing ':';
// validateMapKeyValueNextToken refuses it, and grammar.NewRecognizer reads the
// document as not YAML 1.2 (7.4.2).
//
// Matching the reason rather than the document, because the fuzz seeds hold
// many shapes of each and they are one finding apiece. A duplicate the shipped
// parser reports wrongly would still be caught: yamlcorpus holds the key rules,
// and the conformance suite the documents.
func divergesOnPurpose(err error) (string, bool) {
	if err == nil {
		return "", false
	}

	switch msg := err.Error(); {
	case strings.Contains(msg, "already defined at"):
		return "a flow entry written as a key alone repeats its key (3.2.1.1)", true
	case strings.Contains(msg, "map key-value is pre-defined"):
		return "a flow mapping entry takes a single ':' (7.4.2)", true
	default:
		return "", false
	}
}

func assertSameParse(t *testing.T, text string, mode refparser.Mode) {
	t.Helper()

	want, wantErr := refparser.ParseBytes([]byte(text), mode)

	var opts []parser.Option
	if mode&refparser.ParseComments != 0 {
		opts = append(opts, parser.Comments())
	}
	got, gotErr := parser.ParseBytes([]byte(text), opts...)

	switch {
	case wantErr != nil && gotErr != nil:
		// Both refuse it. The messages may be worded differently while the
		// document is refused for the same reason, so the refusal is what is
		// compared, not the text of it.
		return
	case wantErr != nil:
		t.Fatalf("production refuses the document and the lab accepts it\nproduction: %v\nsource:\n%s", wantErr, text)
	case gotErr != nil:
		if why, ok := divergesOnPurpose(gotErr); ok {
			t.Skipf("refused on purpose: %s\nlab: %v", why, gotErr)
		}

		t.Fatalf("the lab refuses a document production accepts\nlab: %v\nsource:\n%s", gotErr, text)
	}

	require.Equal(t, dump(want), dump(got), "the two parsers build different trees")
}

// dump writes every node of f as one line: its depth, its type, the position of
// the token it was built from, and its text.
//
// Rendering the file back would compare a smaller thing -- two trees can render
// alike and carry different positions, and positions are what a tooling
// consumer reads. Walking is what catches that.
func dump(f *ast.File) string {
	var out strings.Builder

	for i, doc := range f.Docs {
		fmt.Fprintf(&out, "--- document %d\n", i)
		ast.Walk(&dumper{out: &out}, doc)
	}

	return out.String()
}

type dumper struct {
	out   *strings.Builder
	depth int
}

func (d *dumper) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		return nil
	}

	pos := "<no token>"
	if tk := n.GetToken(); tk != nil {
		pos = fmt.Sprintf("%d:%d+%d %q", tk.Position.Line, tk.Position.Column, tk.Position.Offset(), tk.Value)
	}
	fmt.Fprintf(d.out, "%*s%s %s\n", d.depth*2, "", n.Type(), pos)

	return &dumper{out: d.out, depth: d.depth + 1}
}

type source struct {
	name string
	text string
}

// sources is every document both parsers are held to: the YAML Test Suite,
// the shapes the benchmarks run on, and the fuzz seeds, which are the documents
// that have already broken something once.
func sources(t *testing.T) []source {
	t.Helper()

	var srcs []source

	suites, err := yamltestsuite.TestSuites()
	require.NoError(t, err)
	for _, s := range suites {
		srcs = append(srcs, source{name: "suite/" + s.Name, text: string(s.InYAML)})
		if len(s.OutYAML) > 0 {
			srcs = append(srcs, source{name: "suite/" + s.Name + "/out", text: string(s.OutYAML)})
		}
	}

	for _, c := range []struct {
		name string
		gen  func(int) string
	}{
		{"flat-map", corpus.FlatMap},
		{"flat-sequence", corpus.FlatSequence},
		{"nested-doc", corpus.NestedDoc},
		{"anchored", corpus.Anchored},
		{"block-scalars", corpus.BlockScalars},
	} {
		for _, n := range []int{1, 10, 200} {
			srcs = append(srcs, source{
				name: fmt.Sprintf("corpus/%s-%d", c.name, n),
				text: c.gen(n),
			})
		}
	}

	seeds, err := fuzzseeds.All()
	require.NoError(t, err)
	for i, seed := range seeds {
		srcs = append(srcs, source{name: fmt.Sprintf("fuzzseed/%04d", i), text: seed})
	}

	return srcs
}
