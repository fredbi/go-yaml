package analysis

import (
	"runtime"
	"testing"

	v3 "go.yaml.in/yaml/v3"

	"github.com/go-openapi/go-yaml/parser"
)

// TestMemoryFootprint reports LIVE heap retained by each representation, which is what
// decides the largest document that can be handled -- not the cumulative allocation a
// benchmark reports.
//
// The AST figure is the important one: it is what a consumer must hold before it can look at
// anything, since nothing is emitted until the whole document is parsed.
func TestMemoryFootprint(t *testing.T) {
	src := nestedDoc(2000)
	mb := float64(len(src)) / (1 << 20)
	t.Logf("source %.2f MB (OpenAPI-shaped, 2000 paths)", mb)

	for _, c := range []struct {
		name string
		f    func() any
	}{
		{"[]rune(src)", func() any { return []rune(src) }},
		{"scanner.Scanner -> token.Tokens", func() any { return tokenize(t, src) }},
		{"parser.ParseBytes -> *ast.File", func() any {
			f, err := parser.ParseBytes([]byte(src))
			if err != nil {
				t.Fatal(err)
			}

			return f
		}},
		{"yaml.v3 -> yaml.Node", func() any {
			var n v3.Node
			if err := v3.Unmarshal([]byte(src), &n); err != nil {
				t.Fatal(err)
			}

			return &n
		}},
		{"yaml.v3 -> map[string]any", func() any {
			var m map[string]any
			if err := v3.Unmarshal([]byte(src), &m); err != nil {
				t.Fatal(err)
			}

			return m
		}},
	} {
		r := retained(c.f)
		t.Logf("  %-32s %7.1f MB   %4.0fx source", c.name, r, r/mb)
	}

	t.Log("")
	t.Log("[]rune is 4x because the scanner indexes runes, not bytes -- which is also why")
	t.Log("token.Position.Offset() is a rune index. See ANALYSIS-go-openapi.md §6.")
}

// retained measures live heap with the value still reachable.
func retained(f func() any) float64 {
	runtime.GC()

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	v := f()

	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(v)

	return float64(after.HeapAlloc-before.HeapAlloc) / (1 << 20)
}

var benchSrc = nestedDoc(2000)

// BenchmarkParse is the profiling entry point:
//
//	go test -run XXX -bench Parse -cpuprofile cpu.out -memprofile mem.out ./...
func BenchmarkParse(b *testing.B) {
	b.SetBytes(int64(len(benchSrc)))
	b.ReportAllocs()

	for b.Loop() {
		if _, err := parser.ParseBytes([]byte(benchSrc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTokenizeOnly(b *testing.B) {
	b.SetBytes(int64(len(benchSrc)))
	b.ReportAllocs()

	for b.Loop() {
		_ = tokenize(b, benchSrc)
	}
}

// BenchmarkYAMLv3Node is the comparison baseline: same input, to a node tree.
func BenchmarkYAMLv3Node(b *testing.B) {
	b.SetBytes(int64(len(benchSrc)))
	b.ReportAllocs()

	for b.Loop() {
		var n v3.Node
		if err := v3.Unmarshal([]byte(benchSrc), &n); err != nil {
			b.Fatal(err)
		}
	}
}
