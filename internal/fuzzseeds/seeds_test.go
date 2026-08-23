// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package fuzzseeds

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

// TestTheCorpusReachesTheSeeds guards the one part of All that can break
// silently.
//
// corpusSeeds reads a file across a module boundary and returns nothing when it
// is not there, which is right for a published copy of this module and wrong
// here: in this repository the artifact is present, and a path that stopped
// resolving would drop 10,405 documents from every fuzz target without a word.
func TestTheCorpusReachesTheSeeds(t *testing.T) {
	corpus, err := corpusSeeds()
	require.NoError(t, err)
	require.NotEmpty(t, corpus, "the stored YAML corpus is not being read; check corpusPath")

	all, err := All()
	require.NoError(t, err)
	assert.Greater(t, len(all), len(corpus)/2,
		"the corpus contributes almost nothing to the seeds")
}

// TestSeedsAreUniqueAndOrdered pins what All promises its callers: the same
// list every run, with no document seeded twice.
func TestSeedsAreUniqueAndOrdered(t *testing.T) {
	first, err := All()
	require.NoError(t, err)
	require.NotEmpty(t, first)

	seen := make(map[string]struct{}, len(first))
	for _, seed := range first {
		_, dup := seen[seed]
		require.Falsef(t, dup, "seeded twice: %q", seed)
		seen[seed] = struct{}{}
	}

	second, err := All()
	require.NoError(t, err)
	assert.Equal(t, first, second)
}
