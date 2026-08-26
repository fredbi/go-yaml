// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package lab_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/lab"
)

// TestDecodeProgressiveMatchesUnmarshal is the gate for the progressive
// decoder, and it is a different gate from the parser's: a decoder is judged on
// the value it produces, not on the tree it walked.
func TestDecodeProgressiveMatchesUnmarshal(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		src  string
	}{
		{"scalar", "hello\n"},
		{"flat mapping", "a: 1\nb: two\nc: true\nd: 1.5\ne:\n"},
		{"flat sequence", "- 1\n- two\n- false\n"},
		{"nested mapping", "a:\n  b:\n    c: 1\n  d: 2\ne: 3\n"},
		{"sequence of mappings", "- a: 1\n  b: 2\n- a: 3\n  b: 4\n"},
		{"mapping of sequences", "a:\n  - 1\n  - 2\nb:\n  - 3\n"},
		{"flow", "{a: 1, b: [2, 3], c: {d: 4}}\n"},
		{"empty values", "a:\nb: ~\nc: null\n"},
		{"deep", "a:\n b:\n  c:\n   d:\n    e: 1\n"},
		{"quoted and block", "a: \"x y\"\nb: 'z'\nc: |\n  one\n  two\n"},
		{"numbers", "a: 0\nb: -1\nc: 1e3\nd: 0.5\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var want any
			require.NoError(t, yaml.Unmarshal([]byte(tc.src), &want))

			got, err := lab.DecodeProgressive([]byte(tc.src))
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

// TestDecodeProgressiveRefusesWhatItDoesNotRead holds the experiment's edges,
// so a wrong number cannot come out of a document it only half understands.
func TestDecodeProgressiveRefusesWhatItDoesNotRead(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ name, src string }{
		{"anchor", "a: &x 1\nb: *x\n"},
		{"tag", "a: !!str 1\n"},
		{"merge key", "a: &b\n  x: 1\nc:\n  <<: *b\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := lab.DecodeProgressive([]byte(tc.src))
			require.Error(t, err)
		})
	}
}
