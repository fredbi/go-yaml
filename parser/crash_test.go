package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser"
)

// TestParseEmptySourceDocumentToken pins the shape of the AST for an empty
// source, which is the smallest input that reaches a document with no body.
func TestParseEmptySourceDocumentToken(t *testing.T) {
	file, err := parser.ParseBytes(nil, 0)
	require.NoError(t, err)
	require.Len(t, file.Docs, 1)

	doc := file.Docs[0]
	assert.Nil(t, doc.Body, "an empty source still yields a document with no body")

	require.NotPanics(t, func() {
		assert.Nil(t, doc.GetToken(), "a document with no body has no token")
	})
}
