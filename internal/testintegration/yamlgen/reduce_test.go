// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"
)

func TestReduce(t *testing.T) {
	tests := []struct {
		name        string
		src         string
		interesting func([]byte) bool
		want        string
	}{
		{
			name:        "drops the lines that do not matter",
			src:         "a: 1\nb: 2\nBANG: 3\nd: 4\n",
			interesting: func(b []byte) bool { return bytes.Contains(b, []byte("BANG")) },
			// Reduction does not stop at the line: once the other lines are
			// gone the byte pass takes the rest of this one with them.
			want: "BANG",
		},
		{
			name:        "shortens a scalar once the lines are gone",
			src:         "key: aaaaaaaaaa\nother: bbbb\n",
			interesting: func(b []byte) bool { return bytes.Contains(b, []byte("aa")) },
			want:        "aa",
		},
		{
			name:        "leaves an already minimal document alone",
			src:         "x",
			interesting: func(b []byte) bool { return bytes.Contains(b, []byte("x")) },
			want:        "x",
		},
		{
			name:        "returns the input when it is not interesting to begin with",
			src:         "a: 1\n",
			interesting: func(_ []byte) bool { return false },
			want:        "a: 1\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(yamlgen.Reduce([]byte(tc.src), tc.interesting))
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestReduceKeepsThePredicateTrue is the property that matters for a reducer:
// whatever it returns must still be a case of the thing being reduced.
func TestReduceKeepsThePredicateTrue(t *testing.T) {
	src := "a: 1\nb:\n-\n# c\n - x\n"
	require.True(t, renderDoesNotSettle([]byte(src)), "the fixture must show the defect to begin with")

	small := yamlgen.Reduce([]byte(src), renderDoesNotSettle)

	require.True(t, renderDoesNotSettle(small),
		"the reduced document no longer shows the defect: %q", small)
	assert.Less(t, len(small), len(src), "and it should be smaller than what it started from")
	t.Logf("reduced %q to %q", src, string(small))
}

// TestReducedReportIsUseful exercises the whole failure path on a document that
// is known to fail, because the report is only ever produced when something has
// already gone wrong -- which is precisely when nobody wants to discover that
// the reporting itself is broken.
func TestReducedReportIsUseful(t *testing.T) {
	report := reduced("Defect", "a: 1\nb:\n-\n# c\n - x\n", renderDoesNotSettle)

	assert.Contains(t, report, "as generated")
	assert.Contains(t, report, "reduced to")
	assert.Contains(t, report, "renders to:")
	assert.Contains(t, report, "reads as:")
	assert.Contains(t, report, "func TestDefect(t *testing.T) {")
	t.Log(report)
}

func TestReproducerIsValidGo(t *testing.T) {
	got := yamlgen.Reproducer("Defect", "k: |+\n  a\n\n", "x", "y")

	assert.Contains(t, got, "func TestDefect(t *testing.T) {")
	assert.Contains(t, got, "yaml.Unmarshal")
	// A YAML document is far more readable backquoted, and every document the
	// generator emits can be.
	assert.Contains(t, got, "const src = `k: |+")
	assert.True(t, strings.HasSuffix(got, "}\n"))
}
