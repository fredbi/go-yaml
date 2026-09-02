// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package lab_test

import (
	"strconv"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
	"github.com/go-openapi/go-yaml/internal/refparser"
	"github.com/go-openapi/go-yaml/parser"
)

// TestGroupingSurvivesEveryJoin reads each document in runs of a few tokens and
// checks the tree is the one a single run gives.
//
// The grouping reads a run at a time and each pass keeps what it cannot settle
// yet on the grouper, so a group straddling the join between two runs is
// grouped as one. A run of two tokens puts a join between almost every pair,
// which is the hardest a chunk boundary can be.
//
// Without the state carried, a "|" at the end of one run would lose its content
// at the start of the next, an anchor would lose its name, and a key would lose
// the ':' that names it.
func TestGroupingSurvivesEveryJoin(t *testing.T) {
	all, err := workloads.All()
	require.NoError(t, err)

	stress, err := workloads.Stress()
	require.NoError(t, err)

	for _, runs := range []int{1, 2, 3, 7, 64} {
		t.Run("runs of "+strconv.Itoa(runs), func(t *testing.T) {
			for _, set := range [][]workloads.Workload{all, stress} {
				for _, w := range set {
					want, err := refparser.ParseBytes(w.Data, 0)
					require.NoError(t, err, w.Name)

					got, err := parser.ParseBytes(w.Data, parser.ChunkSize(runs))
					require.NoError(t, err, "%s in runs of %d", w.Name, runs)

					assert.Equal(t, want.String(), got.String(),
						"%s read in runs of %d is not the document read in one", w.Name, runs)
				}
			}
		})
	}
}

// TestGroupingSurvivesEveryJoinOnSmallDocuments covers the shapes the workloads
// do not, each read one token at a time.
func TestGroupingSurvivesEveryJoinOnSmallDocuments(t *testing.T) {
	for _, src := range []string{
		"a: |\n  text\n", "a: >\n  folded\n", "&a x\n", "&a\n", "*a\n",
		"? foo\n: bar\n", "%YAML 1.2\n---\na: 1\n", "!!str x\n", "&a !!str x\n",
		"a: [1, 2, {b: c}]\n", "- &x 1\n- *x\n", "a: # comment\n  b: 1\n",
		"? [a, b]\n: c\n", "--- a\n--- b\n", "a: 1\n...\nb: 2\n",
	} {
		t.Run(src, func(t *testing.T) {
			want, err := refparser.ParseBytes([]byte(src), refparser.ParseComments)
			require.NoError(t, err)

			got, err := parser.ParseBytes([]byte(src), parser.ChunkSize(1), parser.Comments())
			require.NoError(t, err)

			assert.Equal(t, want.String(), got.String(),
				"read one token at a time it is not the document read in one")
		})
	}
}
