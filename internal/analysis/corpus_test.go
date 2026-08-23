package analysis

import (
	"fmt"
	"strings"
)

// flatMap is a mapping of n sibling keys at ONE level: the shape that exposes the quadratic
// behavior in parser.parseMap, and the shape of a large OpenAPI "paths:" mapping.
func flatMap(n int) string {
	var b strings.Builder
	b.Grow(n * 24)
	for i := range n {
		fmt.Fprintf(&b, "key%06d: value%06d\n", i, i)
	}

	return b.String()
}

// nestedDoc is an OpenAPI-shaped document: n sibling paths, each a small nested mapping.
// Deliberately different from flatMap -- it understates the quadratic effect, because the
// sibling count at any one level stays low. Both shapes are needed to see the real picture.
func nestedDoc(n int) string {
	var b strings.Builder
	b.WriteString("openapi: 3.0.0\ninfo:\n  title: Analysis\n  version: 1.0.0\npaths:\n")
	for i := range n {
		fmt.Fprintf(&b,
			"  /res%d:\n    get:\n      operationId: getRes%d\n"+
				"      summary: a reasonably long summary line for realism\n"+
				"      responses:\n        '200':\n          description: ok\n", i, i)
	}

	return b.String()
}
