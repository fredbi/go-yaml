// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package lab_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/lab/labparser"
	"github.com/go-openapi/go-yaml/parser"
)

// TestDocumentBoundaries checks the splitter cuts a stream into the same
// documents the pass it replaced did.
//
// The old one took the whole stream as a slice and recursed over what was left
// after each marker. The new one is given tokens one at a time and keeps only
// the document being read, so every case the recursion settled by looking at
// the slice has to be settled by state instead. These are the ones that say it
// does.
//
// The counts matter as much as the rendering: "..." closing nothing opens no
// document, an empty stream is one empty document, and a directive stands as a
// document of its own.
func TestDocumentBoundaries(t *testing.T) {
	for _, src := range []string{
		"", "\n", "a: 1", "---", "---\n---", "...", "...\n...",
		"---\n...", "a: 1\n---\nb: 2", "a: 1\n...\nb: 2",
		"...\na: 1", "---\na: 1\n...\n---\nb: 2", "# just a comment\n",
		"%YAML 1.2\n---\na: 1", "---\n", "\n---\n\n", "a: 1\n...\n...\n",
		"--- a\n--- b\n", "--- a\n...\n--- b\n...\n",
		"%YAML 1.2\n---\na\n---\nb\n",
	} {
		t.Run(src, func(t *testing.T) {
			want, wantErr := parser.ParseBytes([]byte(src), parser.ParseComments)
			got, gotErr := labparser.ParseBytes([]byte(src), labparser.Comments())

			require.NoError(t, wantErr)
			require.NoError(t, gotErr)

			assert.Equal(t, len(want.Docs), len(got.Docs), "a different number of documents")
			assert.Equal(t, want.String(), got.String())
		})
	}
}

// TestDocumentBoundariesRefused checks the two rules that need to see what
// follows a marker, which the splitter settles on the next token rather than by
// looking ahead.
func TestDocumentBoundariesRefused(t *testing.T) {
	for _, src := range []string{
		"--- a: 1\n", // a value cannot stand after "---" on its line
		"--- - a\n",  // nor a sequence entry
		"... a\n",    // nothing but a comment may follow "..." on its line
		"a: 1\n... b\n",
	} {
		t.Run(src, func(t *testing.T) {
			_, wantErr := parser.ParseBytes([]byte(src), 0)
			_, gotErr := labparser.ParseBytes([]byte(src))

			require.Error(t, wantErr, "the parser that ships takes this")
			require.Error(t, gotErr, "the splitter takes what the parser that ships refuses")
		})
	}
}

// TestDocumentBoundariesInRunsOfOne reads the same documents a token at a time.
//
// The splitter is fed by the grouping, which now hands over a run at a time, so
// the two markers and the token after them may arrive in different runs.
func TestDocumentBoundariesInRunsOfOne(t *testing.T) {
	for _, src := range []string{
		"---\n---", "...", "a: 1\n---\nb: 2", "a: 1\n...\nb: 2",
		"%YAML 1.2\n---\na: 1", "---\na: 1\n...\n---\nb: 2",
	} {
		t.Run(src, func(t *testing.T) {
			want, err := parser.ParseBytes([]byte(src), 0)
			require.NoError(t, err)

			got, err := labparser.ParseBytes([]byte(src), labparser.ChunkSize(1))
			require.NoError(t, err)

			assert.Equal(t, len(want.Docs), len(got.Docs))
			assert.Equal(t, want.String(), got.String())
		})
	}
}
