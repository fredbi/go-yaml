// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"fmt"
	"sort"
	"testing"

	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
	"github.com/go-openapi/go-yaml/internal/yamltestsuite"
	"github.com/go-openapi/go-yaml/lexer"
	"github.com/go-openapi/go-yaml/parser"
)

// span returns the number of raw tokens a grouped token covers.
func span(tk *parser.Token) int {
	if tk.Group == nil {
		return 1
	}
	n := 0
	for _, inner := range tk.Group.Tokens {
		n += span(inner)
	}
	return n
}

// TestTokenDensity reports what one token costs.
//
// The counts are what turn the allocation profile into a per-token figure:
// azure_swagger is 35,472 tokens, and lexer.Tokenize allocates 164,505 times
// for it -- 4.6 allocations and 358 bytes per token, where a token.Token is 96
// bytes and its token.Position another 40.
func TestTokenDensity(t *testing.T) {
	all, err := workloads.All()
	if err != nil {
		t.Fatal(err)
	}

	for _, w := range all {
		tokens := lexer.Tokenize(string(w.Data))
		t.Logf("%-18s %8d bytes %8d tokens %5.1f bytes/token",
			w.Name, len(w.Data), len(tokens), float64(len(w.Data))/float64(len(tokens)))
	}
}

// TestGroupSpans measures how many raw tokens one grouped token covers, which
// is the window a tokenizer that streams would have to hold.
//
// CreateGroupedTokens runs nine passes over the whole token slice, so today the
// question does not arise. It arises the moment the parser pulls tokens one at
// a time: a pass that groups "&a !!str |" into one node needs the tokens after
// the anchor before it can decide anything.
//
// The answer, over the five workloads and the 402 documents of the YAML Test
// Suite: every property group -- anchor, alias, tag, literal, folded, directive
// -- fits in 4 raw tokens. map_key and map_key_value reach 18 and 19, and those
// are the explicit keys, which are unbounded by construction: "? " may be
// followed by a whole block. document covers the entire stream, which is a
// wrapper rather than lookahead.
func TestGroupSpans(t *testing.T) {
	all, err := workloads.All()
	if err != nil {
		t.Fatal(err)
	}

	maxByType := map[string]int{}
	countByType := map[string]int{}
	histByType := map[string]map[int]int{}

	var walk func(tk *parser.Token)
	walk = func(tk *parser.Token) {
		if tk.Group == nil {
			return
		}
		name := fmt.Sprint(tk.Group.Type)
		n := span(tk)
		countByType[name]++
		if n > maxByType[name] {
			maxByType[name] = n
		}
		if histByType[name] == nil {
			histByType[name] = map[int]int{}
		}
		bucket := n
		if bucket > 8 {
			bucket = 9 // "9+"
		}
		histByType[name][bucket]++
		for _, inner := range tk.Group.Tokens {
			walk(inner)
		}
	}

	sources := make([]string, 0, len(all))
	for _, w := range all {
		sources = append(sources, string(w.Data))
	}

	// The workloads are ordinary documents. The YAML Test Suite is where the
	// anchors, tags and explicit keys live, so it is what bounds the window.
	suite, err := yamltestsuite.TestSuites()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range suite {
		sources = append(sources, string(test.InYAML))
	}

	for _, src := range sources {
		grouped, err := parser.CreateGroupedTokens(lexer.Tokenize(src))
		if err != nil {
			continue // the suite carries documents the parser refuses on purpose
		}
		for _, tk := range grouped {
			walk(tk)
		}
	}

	names := make([]string, 0, len(maxByType))
	for name := range maxByType {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return maxByType[names[i]] > maxByType[names[j]] })

	t.Log("raw tokens covered by one group, over the five workloads and the YAML Test Suite")
	for _, name := range names {
		h := histByType[name]
		t.Logf("  %-22s count %7d  max %6d   sizes 1:%d 2:%d 3:%d 4:%d 5-8:%d 9+:%d",
			name, countByType[name], maxByType[name],
			h[1], h[2], h[3], h[4], h[5]+h[6]+h[7]+h[8], h[9])
	}
}
