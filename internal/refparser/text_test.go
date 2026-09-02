// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package refparser_test

import (
	"testing"
	"unsafe"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/internal/refparser"
	"github.com/go-openapi/go-yaml/parser/scanner"
)

// A caller that reads numbers as text -- validating them rather than converting
// them, as a lexer built on this parser would -- reads the document's own bytes
// and copies nothing.
func TestScalarTextReachesTheAST(t *testing.T) {
	const src = "int: 1234567890\n" +
		"float: 3.25\n" +
		"hex: 0xFF\n" +
		"bool: TRUE\n" +
		"inf: -.inf\n" +
		"null: ~\n" +
		"plain: plain text\n" +
		"quoted: \"needs a copy\"\n" +
		"block: |\n  first\n  second\n"

	base := uintptr(unsafe.Pointer(unsafe.StringData(src)))
	indexIn := func(b []byte) int {
		if len(b) == 0 {
			return -1
		}
		p := uintptr(unsafe.Pointer(&b[0]))
		if p < base || p >= base+uintptr(len(src)) {
			return -1
		}

		return int(p - base)
	}

	var s scanner.Scanner
	s.Init(src)
	p, err := refparser.New(s.Tokens(), 0)
	require.NoError(t, err)
	require.NoError(t, s.Err())

	f, err := p.Parse()
	require.NoError(t, err)
	require.Len(t, f.Docs, 1)

	body, ok := f.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)

	text := make(map[string]string)
	inSource := make(map[string]bool)
	for _, v := range body.Values {
		scalar, ok := v.Value.(ast.ScalarNode)
		require.Truef(t, ok, "%s is not a scalar", v.Key.String())

		key := v.Key.String()
		text[key] = scalar.Text()
		inSource[key] = indexIn(scalar.Bytes()) >= 0

		assert.Equalf(t, scalar.Text(), string(scalar.Bytes()),
			"%s: Bytes and Text disagree", key)
	}

	assert.Equal(t, "1234567890", text["int"])
	assert.Equal(t, "3.25", text["float"])
	assert.Equal(t, "0xFF", text["hex"])
	assert.Equal(t, "TRUE", text["bool"])
	assert.Equal(t, "-.inf", text["inf"])
	assert.Equal(t, "~", text["null"])
	assert.Equal(t, "plain text", text["plain"])
	assert.Equal(t, "needs a copy", text["quoted"])
	assert.Equal(t, "first\nsecond\n", text["block"])

	// The document's own bytes, for everything the scanner did not have to
	// rewrite.
	for _, key := range []string{"int", "float", "hex", "bool", "inf", "null", "plain"} {
		assert.Truef(t, inSource[key], "%s: %q should be a window into the source", key, text[key])
	}
	// A quoted scalar loses its quotes and a block scalar its layout, so both
	// have to be copies.
	assert.False(t, inSource["quoted"], "a quoted scalar cannot be the source's own bytes")
	assert.False(t, inSource["block"], "a block scalar cannot be the source's own bytes")
}
