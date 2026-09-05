// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/internal/corpus"
	"github.com/go-openapi/go-yaml/internal/fuzzseeds"
	"github.com/go-openapi/go-yaml/internal/yamltestsuite"
	"github.com/go-openapi/go-yaml/parser"
)

// TestWalkDigest prints one digest over everything a walk hands to a visitor,
// for every document of the corpus.
//
// It is the A/B for the arena rewind: run it with and without -tags yamlprobe
// and compare the two digests. The tag makes a rewind clear the cells it hands
// back, so a walk that reads a node it has already handed over reads an empty
// one and the digest moves. Same digest both ways means nothing read past its
// own handover.
//
//	go test -run TestWalkDigest -v ./parser/ | grep digest
//	go test -tags yamlprobe -run TestWalkDigest -v ./parser/ | grep digest
func TestWalkDigest(t *testing.T) {
	sum := sha256.New()

	var documents, refused int
	for _, src := range walkSources(t) {
		d := &digestVisitor{out: sum}
		fmt.Fprintf(sum, "\n=== %s\n", src.name)
		if _, err := parser.New(parser.WithComments()).Walk([]byte(src.text), d); err != nil {
			fmt.Fprintf(sum, "refused: %v\n", err)
			refused++

			continue
		}
		documents++
	}

	t.Logf("digest %s over %d documents walked and %d refused",
		hex.EncodeToString(sum.Sum(nil)), documents, refused)
}

// digestVisitor writes what it is handed, so that anything the walk reads out
// of a reclaimed cell shows up as a different document.
type digestVisitor struct {
	out interface{ Write([]byte) (int, error) }
}

func (d *digestVisitor) Enter(node ast.Node, at parser.Step) bool {
	d.write("enter", node, at)

	return true
}

func (d *digestVisitor) Leave(node ast.Node, at parser.Step) { d.write("leave", node, at) }

func (d *digestVisitor) write(what string, node ast.Node, at parser.Step) {
	tk := node.GetToken()
	value := ""
	var offset, end int32
	if tk != nil {
		value, offset, end = tk.Value, tk.Position.Offset(), tk.EndOffset()
	}
	fmt.Fprintf(d.out, "%s %v in=%v depth=%d idx=%d key=%v at=%d:%d span=%d:%d value=%q\n",
		what, node.Type(), at.In, at.Depth, at.Index, at.Key,
		at.At.Line, at.At.Column, offset, end, value)
}

type walkSource struct{ name, text string }

func walkSources(t *testing.T) []walkSource {
	t.Helper()

	var srcs []walkSource

	suites, err := yamltestsuite.TestSuites()
	require.NoError(t, err)
	for _, s := range suites {
		srcs = append(srcs, walkSource{name: "suite/" + s.Name, text: string(s.InYAML)})
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
			srcs = append(srcs, walkSource{name: fmt.Sprintf("corpus/%s-%d", c.name, n), text: c.gen(n)})
		}
	}

	seeds, err := fuzzseeds.All()
	require.NoError(t, err)
	for i, seed := range seeds {
		srcs = append(srcs, walkSource{name: fmt.Sprintf("fuzzseed/%04d", i), text: seed})
	}

	return srcs
}
