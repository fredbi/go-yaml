// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser2

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/go-openapi/go-yaml/scanner"
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
		f, err := os.Open("../internal/analysis/workloads/testdata/" + name + ".yaml.gz")
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
