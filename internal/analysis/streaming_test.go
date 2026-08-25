// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"sort"
	"testing"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
	"github.com/go-openapi/go-yaml/parser2"
	"github.com/go-openapi/go-yaml/scanner"
	"github.com/go-openapi/go-yaml/token"
)

// What a parser building the tree as it reads would have to hold.
//
// The parser keeps the whole token stream and the whole tree today. These
// measure what it would keep instead if a consumer took each node as it was
// finished and forgot it, which is what decides whether that is worth building.

// TestTopLevelEntriesAreNotAStopGap: splitting the stream at the top level does
// not bound anything. Two of the five workloads are one root collection, and
// the largest top-level entry is then the whole document.
func TestTopLevelEntriesAreNotAStopGap(t *testing.T) {
	all, err := workloads.All()
	if err != nil {
		t.Fatal(err)
	}

	for _, w := range all {
		var s scanner.Scanner
		s.Init(string(w.Data))

		var toks []token.Token
		for tk := range s.Tokens() {
			toks = append(toks, tk)
		}

		base := int32(1 << 30)
		for _, tk := range toks {
			base = min(base, tk.Position.Column)
		}

		var maxEntry, run, entries int
		for _, tk := range toks {
			if tk.Position.Column == base {
				maxEntry = max(maxEntry, run)
				entries++
				run = 0
			}
			run++
		}
		maxEntry = max(maxEntry, run)

		t.Logf("%-16s %7d tokens   %6d top-level entries   largest %7d (%.3f%% of the stream)",
			w.Name, len(toks), entries, maxEntry, 100*float64(maxEntry)/float64(len(toks)))
	}
}

// TestCollectionBreadth reports how wide the widest collection is, which is what
// an open container accumulates before it can be handed over.
func TestCollectionBreadth(t *testing.T) {
	all, err := workloads.All()
	if err != nil {
		t.Fatal(err)
	}

	for _, w := range all {
		f, err := parser2.ParseBytes(w.Data, 0)
		if err != nil {
			t.Fatal(err)
		}

		var seq, mapp []int
		for _, d := range f.Docs {
			ast.Walk(breadthVisitor{seq: &seq, mapp: &mapp}, d)
		}
		sort.Sort(sort.Reverse(sort.IntSlice(seq)))
		sort.Sort(sort.Reverse(sort.IntSlice(mapp)))

		t.Logf("%-16s %6d sequences, widest %v   %6d mappings, widest %v",
			w.Name, len(seq), head(seq, 3), len(mapp), head(mapp, 3))
	}
}

// TestSlidingPeak is the number that matters: for every ancestor of the node
// being built, how many of that ancestor's children are already finished. A
// consumer taking each node as it completes holds that many, and no more.
func TestSlidingPeak(t *testing.T) {
	all, err := workloads.All()
	if err != nil {
		t.Fatal(err)
	}

	for _, w := range all {
		f, err := parser2.ParseBytes(w.Data, 0)
		if err != nil {
			t.Fatal(err)
		}

		var total, peak int
		var done []int // done[d] counts the finished children of the ancestor at depth d
		var walk func(n ast.Node, d int)
		walk = func(n ast.Node, d int) {
			for len(done) <= d {
				done = append(done, 0)
			}
			done[d] = 0
			for _, c := range childrenOf(n) {
				walk(c, d+1)
				done[d]++
				live := 0
				for i := 0; i <= d; i++ {
					live += done[i]
				}
				peak = max(peak, live)
			}
			total++
		}
		for _, d := range f.Docs {
			walk(d, 0)
		}

		t.Logf("%-16s %7d nodes   peak live under a sliding build %5d (%.2f%%)",
			w.Name, total, peak, 100*float64(peak)/float64(total))
	}
}

type breadthVisitor struct{ seq, mapp *[]int }

func (b breadthVisitor) Visit(n ast.Node) ast.Visitor {
	switch v := n.(type) {
	case *ast.SequenceNode:
		*b.seq = append(*b.seq, len(v.Values))
	case *ast.MappingNode:
		*b.mapp = append(*b.mapp, len(v.Values))
	}

	return b
}

func head(v []int, n int) []int {
	if len(v) < n {
		return v
	}

	return v[:n]
}

// childrenOf returns the nodes n holds, in the order they are read.
func childrenOf(n ast.Node) []ast.Node {
	switch v := n.(type) {
	case *ast.DocumentNode:
		if v.Body == nil {
			return nil
		}

		return []ast.Node{v.Body}
	case *ast.MappingNode:
		out := make([]ast.Node, 0, len(v.Values))
		for _, e := range v.Values {
			out = append(out, e)
		}

		return out
	case *ast.MappingValueNode:
		var out []ast.Node
		if v.Key != nil {
			out = append(out, v.Key)
		}
		if v.Value != nil {
			out = append(out, v.Value)
		}

		return out
	case *ast.SequenceNode:
		return append([]ast.Node{}, v.Values...)
	case *ast.SequenceEntryNode:
		if v.Value == nil {
			return nil
		}

		return []ast.Node{v.Value}
	case *ast.AnchorNode:
		return []ast.Node{v.Value}
	case *ast.TagNode:
		return []ast.Node{v.Value}
	}

	return nil
}
