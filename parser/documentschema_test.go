// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/parser"
	"github.com/go-openapi/go-yaml/token"
)

// TestEachDocumentRecordsTheSchemaItWasReadUnder checks ast.DocumentNode.Schema:
// the version a document's "%YAML" line declares, for that document alone, and
// the WithYAMLVersion option where it declares none.
func TestEachDocumentRecordsTheSchemaItWasReadUnder(t *testing.T) {
	bodies := func(f *ast.File) []token.Schema {
		var out []token.Schema
		for _, doc := range f.Docs {
			if _, directive := doc.Body.(*ast.DirectiveNode); directive {
				continue
			}
			out = append(out, doc.Schema)
		}

		return out
	}

	f, err := parser.ParseBytes([]byte("%YAML 1.1\n---\na: yes\n---\nb: yes\n"))
	require.NoError(t, err)
	assert.Equal(t, []token.Schema{token.Schema11, token.Schema12}, bodies(f),
		"the directive scopes the one document after it")

	f, err = parser.ParseBytes([]byte("a: 1\n---\nb: 2\n"), parser.WithYAMLVersion(parser.YAML11))
	require.NoError(t, err)
	assert.Equal(t, []token.Schema{token.Schema11, token.Schema11}, bodies(f), "the option covers every document")

	f, err = parser.ParseBytes([]byte("a: 1\n"))
	require.NoError(t, err)
	assert.Equal(t, []token.Schema{token.Schema12}, bodies(f), "1.2 by default")
}
