// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package refparser

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/go-openapi/go-yaml/internal/scanner"
	"github.com/go-openapi/go-yaml/token"
)

type stat struct{ vals []int }

func (s *stat) add(v int) { s.vals = append(s.vals, v) }
func (s *stat) report(name string) {
	if len(s.vals) == 0 {
		fmt.Printf("  %-34s (none)\n", name)
		return
	}
	sort.Ints(s.vals)
	p := func(q float64) int { return s.vals[min(int(float64(len(s.vals))*q), len(s.vals)-1)] }
	fmt.Printf("  %-34s n=%-7d p50=%-4d p99=%-5d max=%-6d\n", name, len(s.vals), p(.50), p(.99), s.vals[len(s.vals)-1])
}

func measure(src string, colonBack, colonLines, flowBack, keyBody *stat) {
	var s scanner.Scanner
	s.Init(src)
	var raw rawTokens
	for tk := range s.Tokens() {
		raw.add(tk)
	}
	if s.Err() != nil {
		return
	}
	g := newGrouper(raw.n)
	tks := g.collect(raw.n, g.groupAnchorsWithScalarTags(g.groupScalarTags(g.groupAnchors(g.groupBlockScalars(g.attachLineComments(g.stream(&raw)))))))

	for i, tk := range tks {
		switch tk.Type() {
		case token.MappingValueType:
			j := keyCandidateIndex(tks, i)
			if j < 0 {
				continue
			}
			colonBack.add(i - j)
			colonLines.add(tks[i].Line() - tks[j].Line())
			if closesFlowCollection(tks[j]) {
				if start := flowCollectionStart(tks[:j+1]); start >= 0 {
					flowBack.add(i - start)
				}
			}
		case token.MappingKeyType:
			end := explicitKeyEnd(tks, i, false)
			keyBody.add(end - i)
		}
	}
}

// TestPassWindows measures how far the grouping passes actually reach, which is
// what a grouper working on a window rather than the whole stream would have to
// hold.
//
// Eight of the ten passes read one or two tokens ahead and no further. The other
// two are data-dependent: the ':' has to find its key, and the '?' has to find
// the end of its body. Neither reaches far -- an implicit key is one line by the
// grammar, and the body of an explicit key is what the parser is about to read
// anyway.
func TestPassWindows(t *testing.T) {
	var colonBack, colonLines, flowBack, keyBody stat

	_ = filepath.WalkDir("../", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if ext := filepath.Ext(path); ext != ".yaml" && ext != ".yml" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		measure(string(b), &colonBack, &colonLines, &flowBack, &keyBody)
		return nil
	})
	for _, name := range []string{"azure_swagger", "citm_catalog", "golang_source", "twitter_status", "canada_geometry"} {
		f, err := os.Open("../../internal/analysis/workloads/testdata/" + name + ".yaml.gz")
		if err != nil {
			t.Skipf("the workloads are not readable from here: %v", err)
		}
		zr, _ := gzip.NewReader(f)
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, zr)
		f.Close()
		measure(buf.String(), &colonBack, &colonLines, &flowBack, &keyBody)
	}

	fmt.Println("\nhow far a pass reaches, over the corpus and the workloads:")
	colonBack.report("':' back to its key (tokens)")
	colonLines.report("':' back to its key (lines)")
	flowBack.report("'[a,b]:' back to the '[' (tokens)")
	keyBody.report("'?' forward over its body (tokens)")
}

// The three helpers below exist for TestPassWindows and for nothing else. They
// answer "how far back does this pass have to look" over a slice holding the
// whole stream, which is the measurement a windowed grouper would be sized
// from. The grouping passes themselves read from an iterator and never index
// like this.

// explicitKeyEnd returns the index just past the body of the explicit key
// introduced by the '?' at tokens[i].
//
// In block context the body is everything indented deeper than the '?' itself,
// and nothing else bounds it -- in particular a ':' on the same line does not.
// "? []: x" has the mapping {[]: x} for its key and no value at all, which is
// what the test suite records for it, so taking the whole indented run is both
// simpler and right.
//
// A flow collection is not indentation-sensitive, so there the body runs to the
// punctuation that ends it: its ':', a ',', or the bracket closing the
// collection it sits in.
func explicitKeyEnd(tokens []*Token, i int, inFlow bool) int {
	if inFlow {
		return explicitFlowKeyEnd(tokens, i)
	}

	col := tokens[i].Column()

	j := i + 1
	for ; j < len(tokens); j++ {
		if tokens[j].Column() <= col {
			break
		}
	}

	return j
}

func explicitFlowKeyEnd(tokens []*Token, i int) int {
	var depth int

	j := i + 1
	for ; j < len(tokens); j++ {
		switch tokens[j].Type() {
		case token.MappingStartType, token.SequenceStartType:
			depth++
		case token.MappingEndType, token.SequenceEndType:
			if depth == 0 {
				return j
			}
			depth--
		case token.MappingValueType, token.CollectEntryType:
			if depth == 0 {
				return j
			}
		}
	}

	return j
}

// keyCandidateIndex finds what would be the key of the ':' at tokens[i],
// skipping comments -- a comment is never a key, and one may sit between an
// explicit "?" key and its ':'. It reports -1 when there is nothing before it.
func keyCandidateIndex(tokens []*Token, i int) int {
	for j := i - 1; j >= 0; j-- {
		if tokens[j].Type() != token.CommentType {
			return j
		}
	}

	return -1
}
