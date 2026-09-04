// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner_test

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/scanner"
)

// TestOriginsTileTheSource checks that the tokens' extents follow one another
// with nothing between, so that src[previous end:this end] is the text the
// document wrote each token as, indentation and all.
//
// That span is the verbatim image of the document: it is what writes a file
// back with its comments, its blank lines and the spelling the author chose.
// Only a scanner that records everything it consumes can offer it, and one did
// not -- the escapes naming a code point, \xXX, \uXXXX and \UXXXXXXXX, set how
// far to skip and appended the decoded rune to the value without counting the
// marker or its digits, so the extents fell behind by the length of every
// escape in the document.
//
// The ends have to reach the end of the source, less the final line break,
// which closes the stream rather than opening a token.
func TestOriginsTileTheSource(t *testing.T) {
	for _, src := range []string{
		"a: \"\\u0041\"\n", "a: \"\\x41\"\n", "a: \"\\U0001F600\"\n",
		"a: \"x\\u3000y\"\n", "a: \"\\uD83D\\uDE00\"\n",
		"a: \"tab\\there\"\n", "a: 'it''s'\n", "a: plain\n",
		"# lead\na: 1 # trail\n\nb: 2\n",
	} {
		assertOriginsTile(t, src)
	}

	for _, w := range workloadDocs(t) {
		assertOriginsTile(t, w)
	}
}

func assertOriginsTile(t *testing.T, src string) {
	t.Helper()

	var s scanner.Scanner
	s.Init([]byte(src))

	prev := 0
	for tk := range s.All() {
		end := int(tk.EndOffset())
		require.GreaterOrEqualf(t, end, prev,
			"a %s token ends before the one before it, at %d after %d", tk.Type, end, prev)
		require.LessOrEqualf(t, end, len(src),
			"a %s token ends past the document, at %d of %d", tk.Type, end, len(src))
		prev = end
	}
	require.NoError(t, s.Err())

	require.Equal(t, len(strings.TrimSuffix(src, "\n")), len(strings.TrimSuffix(src[:prev], "\n")),
		"the extents do not reach the end of the document")
}

// workloadDocs reads the workloads, which are large enough to hold the shapes a
// handwritten case does not think of.
func workloadDocs(t *testing.T) []string {
	t.Helper()

	const dir = "../analysis/workloads/testdata"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("the workloads are not readable from here: %v", err)
	}

	var out []string
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".yaml.gz") {
			continue
		}
		f, err := os.Open(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		z, err := gzip.NewReader(f)
		require.NoError(t, err)
		b, err := io.ReadAll(z)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		out = append(out, string(b))
	}

	return out
}
