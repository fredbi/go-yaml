// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package goyaml_test

import (
	"bytes"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
	"github.com/go-openapi/go-yaml/internal/testintegration/goyaml"
	"github.com/go-openapi/go-yaml/internal/testintegration/libfyaml"
)

// TestGoYAMLReadsADocument is the smoke check.
func TestGoYAMLReadsADocument(t *testing.T) {
	docs, err := goyaml.LoadJSON([]byte("a: 1\n"))
	require.NoError(t, err)
	assert.Equal(t, []string{`{"a":1}`}, docs)
}

// TestGoYAMLReadsAWholeStream keeps it comparable with libfyaml.Load, which
// answers per document.
func TestGoYAMLReadsAWholeStream(t *testing.T) {
	docs, err := goyaml.LoadJSON([]byte("a: 1\n---\nb: 2\n"))
	require.NoError(t, err)
	assert.Len(t, docs, 2)
}

// TestTheThreeReadingsOnAWholeValuedFloatKey is what the third source was added
// for, kept as a regression test now the departure is closed.
//
//	this library  map[string]any{"1.0": "a"}      named by type since 2026-09-10
//	libfyaml      {"1.0": "a"}                    stringified, the float kept
//	yaml.v3       map[any]any{float64(1): "a"}    not stringified at all
//
// yaml.v3 still declines the question rather than answering it, and keeps every
// key's type -- float64(1) for "1.0", int(1) for "1". That is why Load hands
// back Go values: asking it for JSON reports ErrNotJSON and hides the answer.
func TestTheThreeReadingsOnAWholeValuedFloatKey(t *testing.T) {
	const src = "1.0: a\n"

	docs, err := goyaml.Load([]byte(src))
	require.NoError(t, err)
	require.Len(t, docs, 1)

	assert.Equal(t, map[string]any{"1.0": "a"}, decodedByUs(t, src),
		"this library names the key by the canonical spelling of its type")

	keyed, ok := docs[0].(map[string]any)
	assert.False(t, ok, "yaml.v3 does not key it by a string, got %#v", keyed)
	t.Logf("yaml.v3 reads %#v", docs[0])

	_, err = goyaml.LoadJSON([]byte(src))
	assert.ErrorIs(t, err, goyaml.ErrNotJSON, "and JSON cannot name that key")

	if !libfyaml.Available() {
		t.Skipf("libfyaml is not installed at %s; run hack/conformance/install-libfyaml.sh",
			libfyaml.Home())
	}

	theirs, err := libfyaml.Load([]byte(src))
	require.NoError(t, err)
	assert.Equal(t, []string{`{"1.0": "a"}`}, theirs, "libfyaml agrees, now")
}

// decodedByUs is what this library makes of a document, for the comparisons
// above.
func decodedByUs(t *testing.T, src string) any {
	t.Helper()

	var v any
	require.NoError(t, codec.NewDecoder(bytes.NewReader([]byte(src))).Decode(&v))

	return v
}

// TestGoYAMLRefusalIsReported checks the error path, since a refusal is half of
// what a second opinion is for.
func TestGoYAMLRefusalIsReported(t *testing.T) {
	_, err := goyaml.LoadJSON([]byte("a: 1\n b: 2\n"))
	assert.Error(t, err, "a mis-indented mapping should be refused")
}
