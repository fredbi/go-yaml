package analysis

import (
	"fmt"
	"math"
	"runtime"
	"testing"
	"time"

	v3 "go.yaml.in/yaml/v3"

	"github.com/go-openapi/go-yaml/lexer"
	"github.com/go-openapi/go-yaml/parser"
)

// TestFlatMapScaling is the headline measurement: how parse time grows with the number of
// sibling keys in one mapping.
//
// A growth factor near 2 per doubling is linear (correct). Near 4 means quadratic, which is
// what parser.parseMap does today by recursing once per sibling entry and discarding a whole
// MappingNode each time (parser/parser.go:490). yaml.v3 is included only as a scale
// reference -- it is linear, so it shows the difference is not inherent to YAML.
//
// Reports rather than asserts, so it stays informative on any machine.
func TestFlatMapScaling(t *testing.T) {
	t.Logf("%-8s %-9s %-14s %-11s %-14s %-11s %s",
		"keys", "size", "go-yaml", "per key", "yaml.v3", "per key", "ratio")

	for _, n := range []int{1000, 2000, 4000, 8000, 16000} {
		src := flatMap(n)

		g := timeIt(t, func() { mustParse(t, src) })
		y := timeIt(t, func() { mustUnmarshalNode(t, src) })

		t.Logf("%-8d %-9s %-14v %-11s %-14v %-11s %.0fx",
			n, size(len(src)),
			g.Round(time.Microsecond), perKey(g, n),
			y.Round(time.Microsecond), perKey(y, n),
			float64(g)/float64(y))
	}

	t.Log("")
	t.Log("Cost PER KEY is the robust signal: flat = linear, growing = super-linear.")
	t.Log("go-yaml's grows and the ratio widens monotonically; yaml.v3's stays flat.")
	t.Log("See ANALYSIS-go-openapi.md §3.")
}

// TestStageAttribution locates the quadratic term. Tokenizing and grouping are ~linear; the
// parse step is not, which is what points at parseMap rather than at the scanner.
func TestStageAttribution(t *testing.T) {
	t.Logf("%-8s %-24s %-24s %s  (time, and per key)",
		"keys", "Tokenize", "CreateGroupedTokens", "Parse(tokens)")

	for _, n := range []int{1000, 2000, 4000, 8000, 16000} {
		src := flatMap(n)
		toks := lexer.Tokenize(src)

		tt := timeIt(t, func() { lexer.Tokenize(src) })
		tg := timeIt(t, func() {
			if _, err := parser.CreateGroupedTokens(toks); err != nil {
				t.Fatal(err)
			}
		})
		tp := timeIt(t, func() {
			if _, err := parser.Parse(toks, 0); err != nil {
				t.Fatal(err)
			}
		})

		t.Logf("%-8d %-13v %-10s %-13v %-10s %-13v %s", n,
			tt.Round(time.Microsecond), perKey(tt, n),
			tg.Round(time.Microsecond), perKey(tg, n),
			tp.Round(time.Microsecond), perKey(tp, n))
	}

	t.Log("")
	t.Log("Tokenize and grouping hold their per-key cost; Parse does not.")
}

// TestWideDocumentCost states the practical consequence: how long a realistically-sized wide
// document takes. Relevant to anything parsing untrusted YAML, since the cost grows with the
// square of the key count while a depth guard bounds only nesting.
func TestWideDocumentCost(t *testing.T) {
	if testing.Short() {
		t.Skip("slow by construction: that is the finding")
	}

	for _, n := range []int{32000, 64000} {
		src := flatMap(n)
		g := timeIt(t, func() { mustParse(t, src) })
		y := timeIt(t, func() { mustUnmarshalNode(t, src) })
		t.Logf("%6d keys (%7s):  go-yaml %-10v  yaml.v3 %-9v  => %.0fx",
			n, size(len(src)), g.Round(time.Millisecond), y.Round(time.Millisecond),
			float64(g)/float64(y))
	}
}

// TestParseScalesLinearly is the timing counterpart of the guard in parser/scaling_test.go,
// which measures allocation instead and so is the one CI relies on. This one catches a
// slowdown that does not show up as allocation, at the cost of being a wall-clock threshold.
//
// Before parseMap parsed sibling entries in a loop, a 4x larger document took ~10x longer and
// this failed outright.
//
// It guards the fix in this branch: parseMap parses sibling entries in
// a loop rather than recursing once per entry. Before that change this test failed outright
// (a 4x larger document took ~10x longer); it now passes with room to spare.
func TestParseScalesLinearly(t *testing.T) {
	const small, large = 4000, 16000 // a 4x increase in size

	s := timeIt(t, func() { mustParse(t, flatMap(small)) })
	l := timeIt(t, func() { mustParse(t, flatMap(large)) })

	// linear would be ~4x; allow generous headroom for constant factors and noise.
	if ratio := float64(l) / float64(s); ratio > 8 {
		t.Errorf("parse time grew %.1fx for a 4x larger document: still super-linear", ratio)
	}
}

func mustParse(t *testing.T, src string) {
	t.Helper()
	if _, err := parser.ParseBytes([]byte(src), 0); err != nil {
		t.Fatalf("parse: %v", err)
	}
}

func mustUnmarshalNode(t *testing.T, src string) {
	t.Helper()
	var n v3.Node
	if err := v3.Unmarshal([]byte(src), &n); err != nil {
		t.Fatalf("yaml.v3 unmarshal: %v", err)
	}
}

// timeIt reports the BEST of several runs. A single run is dominated by GC noise at these
// sizes -- enough to make a quadratic curve look linear -- and the minimum is the standard
// estimator for "how long does this actually take".
func timeIt(t *testing.T, f func()) time.Duration {
	t.Helper()

	const reps = 5

	best := time.Duration(math.MaxInt64)
	for range reps {
		runtime.GC()
		start := time.Now()
		f()
		if d := time.Since(start); d < best {
			best = d
		}
	}

	return best
}

// perKey is the diagnostic: an O(n) algorithm holds it constant as n grows, an O(n^2) one
// grows it in proportion to n.
func perKey(d time.Duration, n int) string {
	return fmt.Sprintf("%.1f us/key", float64(d.Nanoseconds())/float64(n)/1000)
}

func size(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%d KB", n/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
