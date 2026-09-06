// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package libfyaml_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/testintegration/libfyaml"
)

func needs(t *testing.T) {
	t.Helper()

	if !libfyaml.Available() {
		t.Skipf("libfyaml is not installed at %s; run hack/conformance/install-libfyaml.sh",
			libfyaml.Home())
	}
}

// TestLibfyamlReadsADocument is the smoke check, and it is also what says the
// binding still works the way the harness assumes.
func TestLibfyamlReadsADocument(t *testing.T) {
	needs(t)

	t.Logf("libfyaml %s at %s", libfyaml.Version(), libfyaml.Home())

	docs, err := libfyaml.Load([]byte("a: 1\n"))
	require.NoError(t, err)
	assert.Equal(t, []string{`{"a": 1}`}, docs)
}

// TestLibfyamlReadsAWholeStream holds the trap the plan records: loads reads
// one document and load_all takes a filename, so loads_all is the one that
// takes a string.
func TestLibfyamlReadsAWholeStream(t *testing.T) {
	needs(t)

	docs, err := libfyaml.Load([]byte("a: 1\n---\nb: 2\n"))
	require.NoError(t, err)
	assert.Len(t, docs, 2, "a two-document stream should come back as two answers")
}

// TestLibfyamlDisagreesAboutAWholeValuedFloatKey pins the disagreement the
// yardstick was reinstated for.
//
// It is the corroboration under yamlcorpus.Departures, "a key that is a float
// with a whole value", and pinning it here means a libfyaml upgrade that
// changed its mind would be visible rather than silently re-scoring the corpus.
func TestLibfyamlDisagreesAboutAWholeValuedFloatKey(t *testing.T) {
	needs(t)

	for _, tc := range []struct{ src, theirs string }{
		{src: "1.0: a\n", theirs: `{"1.0": "a"}`},
		{src: "1e3: a\n", theirs: `{"1000.0": "a"}`},
		{src: "-0.0: a\n", theirs: `{"-0.0": "a"}`},
	} {
		docs, err := libfyaml.Load([]byte(tc.src))
		require.NoError(t, err, "%q", tc.src)
		assert.Equal(t, []string{tc.theirs}, docs,
			"%q: libfyaml keeps the float where this library names it as an integer", tc.src)
	}

	t.Run("and agrees on a fractional one", func(t *testing.T) {
		docs, err := libfyaml.Load([]byte("0.5: a\n"))
		require.NoError(t, err)
		assert.Equal(t, []string{`{"0.5": "a"}`}, docs)
	})
}

// TestLibfyamlRefusalIsReported checks the error path, since a refusal is half
// of what a yardstick is for.
func TestLibfyamlRefusalIsReported(t *testing.T) {
	needs(t)

	_, err := libfyaml.Load([]byte("a: 1\n b: 2\n"))
	assert.Error(t, err, "a mis-indented mapping should be refused")
}
