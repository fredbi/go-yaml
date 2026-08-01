// Package corpus generates the synthetic documents the benchmarks run on.
//
// It lives under testdata so that it is not part of the published module, and
// it is shared so that a number measured in one package means the same thing as
// a number measured in another.
//
// The shapes are chosen for what they expose, not for realism:
//
//   - FlatMap is wide and shallow. Sibling count at one level is the axis that
//     super-linear parsing shows up on, and a depth guard does not bound it.
//   - NestedDoc is the shape of a real OpenAPI specification. It understates
//     the width effect -- the sibling count at any one level stays low -- which
//     is exactly why both shapes are needed.
//   - Anchored and BlockScalars cover the two constructs that constrain
//     streaming: anchors must be retained until the document ends, and block
//     scalars are where the scanner does its least regular work.
package corpus

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// FlatMap builds a mapping of n sibling keys at one level.
func FlatMap(n int) string {
	var b strings.Builder
	b.Grow(n * 24)

	for i := range n {
		fmt.Fprintf(&b, "key%06d: value%06d\n", i, i)
	}

	return b.String()
}

// FlatSequence builds a sequence of n entries at one level, the sequence
// counterpart of FlatMap.
func FlatSequence(n int) string {
	var b strings.Builder
	b.Grow(n * 16)

	for i := range n {
		fmt.Fprintf(&b, "- value%06d\n", i)
	}

	return b.String()
}

// NestedDoc builds an OpenAPI-shaped document with n sibling paths, each a
// small nested mapping.
func NestedDoc(n int) string {
	var b strings.Builder
	b.Grow(n * 160)
	b.WriteString("openapi: 3.0.0\ninfo:\n  title: Corpus\n  version: 1.0.0\npaths:\n")

	for i := range n {
		fmt.Fprintf(&b,
			"  /res%d:\n    get:\n      operationId: getRes%d\n"+
				"      summary: a reasonably long summary line for realism\n"+
				"      responses:\n        '200':\n          description: ok\n", i, i)
	}

	return b.String()
}

// Anchored builds a document defining one anchor and referring to it n times,
// which is the shape that pins content in memory for a streaming parser.
func Anchored(n int) string {
	var b strings.Builder
	b.Grow(n * 32)
	b.WriteString("defaults: &defaults\n  timeout: 30\n  retries: 3\n  verbose: true\nitems:\n")

	for i := range n {
		fmt.Fprintf(&b, "  - name: item%06d\n    <<: *defaults\n", i)
	}

	return b.String()
}

// BlockScalars builds a document of n literal block scalars, each a few lines.
func BlockScalars(n int) string {
	var b strings.Builder
	b.Grow(n * 96)

	for i := range n {
		fmt.Fprintf(&b, "text%06d: |\n  first line of block %d\n  second line, a little longer\n  third\n", i, i)
	}

	return b.String()
}

// Shape names the document shapes worth measuring separately. The order is
// fixed so that benchmark output lines up run to run.
var Shapes = []struct {
	Name     string
	Generate func(int) string
}{
	{"flat", FlatMap},
	{"nested", NestedDoc},
	{"anchored", Anchored},
	{"blockscalars", BlockScalars},
}

// Sizes are held small enough that a full benchmark set runs in seconds. The
// super-linear behavior of wide mappings is charted in the analysis module,
// which is free to be slow; this set exists to notice a change, not to draw a
// curve.
var Sizes = []int{100, 1000}

// ForEachDocument runs fn over every shape and size, naming each sub-benchmark
// and reporting throughput and allocations for it.
func ForEachDocument(b *testing.B, fn func(*testing.B, []byte)) {
	b.Helper()

	for _, shape := range Shapes {
		for _, size := range Sizes {
			src := []byte(shape.Generate(size))

			b.Run(shape.Name+"-"+strconv.Itoa(size), func(b *testing.B) {
				b.SetBytes(int64(len(src)))
				b.ReportAllocs()
				fn(b, src)
			})
		}
	}
}
