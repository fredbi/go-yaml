// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"testing"
	"unsafe"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/parser"
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

	// ParseBytes scans a string built from the slice it is handed, so a scalar
	// the scanner did not rewrite windows into that string and not into the
	// caller's bytes. What is checked here is that it is a window at all: every
	// scalar carried through unchanged aliases one buffer no larger than the
	// document, and a scalar that had to be rewritten sits outside it.
	data := []byte(src)
	addr := func(b []byte) uintptr {
		if len(b) == 0 {
			return 0
		}

		return uintptr(unsafe.Pointer(&b[0]))
	}

	f, err := parser.New().Parse(data)
	require.NoError(t, err)
	require.Len(t, f.Docs, 1)

	body, ok := f.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)

	text := make(map[string]string)
	at := make(map[string]uintptr)
	for _, v := range body.Values {
		scalar, ok := v.Value.(ast.ScalarNode)
		require.Truef(t, ok, "%s is not a scalar", v.Key.String())

		key := v.Key.String()
		text[key] = scalar.Text()
		at[key] = addr(scalar.Bytes())

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
	// rewrite: all of them fall inside one buffer the size of the document.
	carried := []string{"int", "float", "hex", "bool", "inf", "null", "plain"}
	var low, high uintptr
	for _, key := range carried {
		require.NotZerof(t, at[key], "%s: %q has no bytes at all", key, text[key])
		if low == 0 || at[key] < low {
			low = at[key]
		}
		if at[key] > high {
			high = at[key]
		}
	}
	assert.Lessf(t, high-low, uintptr(len(data)),
		"the scalars carried through span %d bytes for a %d-byte document, so they were copied one by one",
		high-low, len(data))

	// A quoted scalar loses its quotes and a block scalar its layout, so both
	// have to be copies, standing outside the run the others share.
	for _, key := range []string{"quoted", "block"} {
		outside := at[key] < low || at[key] > high
		assert.Truef(t, outside, "%s: a rewritten scalar cannot be the document's own bytes", key)
	}
}
