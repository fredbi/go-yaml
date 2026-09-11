// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package ast_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
	"github.com/go-openapi/go-yaml/parser"
)

// TestAPlainScalarReadingAsAMarkerIsQuotedInColumnZero checks how File.String
// writes a plain scalar that opens with "---" or "...".
//
// Section 9.1.1 puts a document marker at the start of a line, so an indented
// "    ---" over "false" is the plain scalar "--- false". File.String lays the
// document's own node and the keys of a block mapping at the root in column 0,
// where the same text opens a document: "--- false" read back as the boolean
// false, "..." as an empty stream and a key "--- x" did not parse. Written there,
// the scalar is single-quoted. It resolves to a string either way, so the value
// and its type are unchanged.
func TestAPlainScalarReadingAsAMarkerIsQuotedInColumnZero(t *testing.T) {
	for name, tc := range map[string]struct{ src, want string }{
		"the document's own scalar":    {src: "    ---\nfalse\n", want: "'--- false'\n"},
		"a closing marker":             {src: "    ...\nfalse\n", want: "'... false'\n"},
		"three dashes alone":           {src: "  ---\n", want: "'---'\n"},
		"three dots alone":             {src: "  ...\n", want: "'...'\n"},
		"a key at the root":            {src: "  --- x: 1\n  y: 2\n", want: "'--- x': 1\ny: 2\n"},
		"a quote inside":               {src: "  --- it's\n", want: "'--- it''s'\n"},
		"a comment after":              {src: "  --- x # c\n", want: "'--- x' # c\n"},
		"a later document":             {src: "a\n---\n    ---\nfalse\n", want: "a\n---\n'--- false'\n"},
		"no blank after the dashes":    {src: " ---x\n", want: "---x\n"},
		"a nested value":               {src: "k: a\n  --- b\n", want: "k: a --- b\n"},
		"a value beside a root key":    {src: "k:\n  --- b\n", want: "k: --- b\n"},
		"a sequence entry":             {src: "- a\n  ---\n", want: "- a ---\n"},
		"a nested key":                 {src: "k:\n  --- x: 1\n", want: "k:\n  --- x: 1\n"},
		"an anchor in front":           {src: "&a --- x\n", want: "&a --- x\n"},
		"written quoted by the author": {src: "\"--- x\"\n", want: "\"--- x\"\n"},
	} {
		t.Run(name, func(t *testing.T) {
			f, err := parser.ParseBytes([]byte(tc.src), parser.WithComments())
			require.NoError(t, err)

			rendered := f.String()
			assert.Equal(t, tc.want, rendered)

			want, got := allDocuments(t, tc.src), allDocuments(t, rendered)
			assert.Equalf(t, want, got, "%q renders to %q", tc.src, rendered)
		})
	}
}

// allDocuments decodes every document of src.
func allDocuments(t *testing.T, src string) []any {
	t.Helper()

	f, err := parser.ParseBytes([]byte(src))
	require.NoErrorf(t, err, "%q", src)

	docs := make([]any, 0, len(f.Docs))
	for _, doc := range f.Docs {
		var v any
		require.NoErrorf(t, codec.Unmarshal([]byte(doc.String()), &v), "%q", src)
		docs = append(docs, v)
	}

	return docs
}
