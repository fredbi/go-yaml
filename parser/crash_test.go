// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser"
)

// TestParseDoesNotCrash collects inputs that used to bring the parser down.
//
// Each entry is a reduced input, and each is also a seed in
// testdata/fuzzseeds so that fuzzing keeps the surrounding shape covered. What
// the parser answers for these is secondary -- an error is a perfectly good
// answer -- as long as it answers.
func TestParseDoesNotCrash(t *testing.T) {
	tests := map[string]struct {
		source string
		reason string
	}{
		"secondary tag directive with no value": {
			source: "%TAG !! 0\n--- ! ",
			reason: "the value token is absent, and a node was built from it regardless",
		},
		"secondary tag directive at end of input": {
			source: "%TAG !! 0\n--- !",
			reason: "the same, with nothing at all after the tag",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.NotPanicsf(t, func() {
				_, _ = parser.ParseBytes([]byte(test.source))
				_, _ = parser.ParseBytes([]byte(test.source), parser.WithComments())
			}, "%s: %s", name, test.reason)
		})
	}
}

// TestParseEmptySourceDocumentToken pins the shape of the AST for an empty
// source, which is the smallest input that reaches a document with no body.
func TestParseEmptySourceDocumentToken(t *testing.T) {
	file, err := parser.ParseBytes(nil)
	require.NoError(t, err)
	require.Len(t, file.Docs, 1)

	doc := file.Docs[0]
	assert.Nil(t, doc.Body, "an empty source still yields a document with no body")

	require.NotPanics(t, func() {
		assert.Nil(t, doc.GetToken(), "a document with no body has no token")
	})
}
