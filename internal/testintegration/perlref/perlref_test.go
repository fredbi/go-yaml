// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package perlref_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/testintegration/perlref"
)

func needs(t *testing.T) {
	t.Helper()

	if !perlref.Available() {
		t.Skipf("the reference parser is not installed at %s; run "+
			"hack/conformance/install-reference-parser.sh", perlref.Home())
	}
}

// TestItEmitsTheTestSuiteEventStream is the smoke check, and it also pins the
// format the corpus reads.
func TestItEmitsTheTestSuiteEventStream(t *testing.T) {
	needs(t)

	t.Logf("reference parser %s", perlref.Commit())

	events, ok, err := perlref.Events([]byte("a: 1\n"))
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []string{
		"+STR", "+DOC", "+MAP", "=VAL :a", "=VAL :1", "-MAP", "-DOC", "-STR",
	}, events)
}

// TestItResolvesNothing is the property that makes it the third source rather
// than a third opinion.
//
// A plain scalar comes back as the text that was written. No schema is applied
// and no type is decided, so asking it whether "1.0" is a float is asking the
// wrong source -- and that is exactly why it can settle what the node *is*
// while libfyaml and yaml.v3 disagree about what it means.
func TestItResolvesNothing(t *testing.T) {
	needs(t)

	for _, tc := range []struct{ src, scalar string }{
		{src: "1.0: a\n", scalar: "=VAL :1.0"},
		{src: "~: a\n", scalar: "=VAL :~"},
		{src: "0x1: a\n", scalar: "=VAL :0x1"},
		{src: "True: a\n", scalar: "=VAL :True"},
	} {
		events, ok, err := perlref.Events([]byte(tc.src))
		require.NoError(t, err, "%q", tc.src)
		require.True(t, ok, "%q", tc.src)
		assert.Contains(t, events, tc.scalar, "%q keeps the text it was written with", tc.src)
	}
}

// TestAFloatAndAnIntegerAreTwoEntries is the structural answer to the departure
// the three sources were assembled for.
//
// Four scalar events, so two entries. This library reads the same document as
// one entry with the first value gone.
func TestAFloatAndAnIntegerAreTwoEntries(t *testing.T) {
	needs(t)

	events, ok, err := perlref.Events([]byte("1.0: a\n1: b\n"))
	require.NoError(t, err)
	require.True(t, ok, "the document is well formed")

	var scalars int

	for _, e := range events {
		if len(e) > 0 && e[0] == '=' {
			scalars++
		}
	}

	assert.Equal(t, 4, scalars,
		"two keys and two values, where this library keeps one entry: %v", events)
}

// TestARefusalKeepsWhatItManaged holds the other half of what a parser is for.
//
// Where a document stops being one is more informative than the fact that it
// did, so a FAIL still hands back the events it got through.
func TestARefusalKeepsWhatItManaged(t *testing.T) {
	needs(t)

	events, ok, err := perlref.Events([]byte("a: 1\n b: 2\n"))
	require.NoError(t, err)
	assert.False(t, ok, "a mis-indented mapping is not a document")
	assert.NotEmpty(t, events, "and the events up to the fault are the useful part")
}
